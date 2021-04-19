package pumps

import (
	"context"
	b64 "encoding/base64"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/lonelycode/mgohacks"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

var mongoAggregatePumpPrefix = "PMP_MONGOAGG"
var mongoAggregateDefaultEnv = PUMPS_ENV_PREFIX + "_MONGOAGGREGATE" + PUMPS_ENV_META_PREFIX

var THRESHOLD_LEN_TAG_LIST = 1000
var COMMON_TAGS_COUNT = 5

type MongoAggregatePump struct {
	dbSession *mgo.Session
	dbConf    *MongoAggregateConf
	CommonPumpConfig
}

type MongoAggregateConf struct {
	BaseMongoConf
	UseMixedCollection      bool     `mapstructure:"use_mixed_collection"`
	TrackAllPaths           bool     `mapstructure:"track_all_paths"`
	IgnoreTagPrefixList     []string `mapstructure:"ignore_tag_prefix_list"`
	ThresholdLenTagList     int      `mapstructure:"threshold_len_tag_list"`
	StoreAnalyticsPerMinute bool     `mapstructure:"store_analytics_per_minute"`
	IgnoreAggregationsList  []string `mapstructure:"ignore_aggregations"`
}

func (m *MongoAggregatePump) New() Pump {
	newPump := MongoAggregatePump{}
	return &newPump
}

func getListOfCommonPrefix(list []string) []string {
	count := make(map[string]int)
	result := make([]string, 0)
	length := len(list)

	if length == 0 || length == 1 {
		return list
	}

	for i := 0; i < length-1; i++ {
		for j := i + 1; j < length; j++ {
			var prefLen int
			str1 := list[i]
			str2 := list[j]

			if len(str1) > len(str2) {
				prefLen = len(str2)
			} else {
				prefLen = len(str1)
			}

			k := 0
			for k = 0; k < prefLen; k++ {
				if str1[k] != str2[k] {
					if k != 0 {
						count[str1[:k]]++
					}
					break
				}
			}
			if k == prefLen {
				count[str1[:prefLen]]++
			}
		}
	}

	for k := range count {
		result = append(result, k)
	}

	sort.Slice(result, func(i, j int) bool { return count[result[i]] > count[result[j]] })

	return result
}

func (m *MongoAggregatePump) printAlert(doc analytics.AnalyticsRecordAggregate, thresholdLenTagList int) {
	var listofTags []string

	for k := range doc.Tags {
		listofTags = append(listofTags, k)
	}

	listOfCommonPrefix := getListOfCommonPrefix(listofTags)

	// list 5 common tag prefix
	l := len(listOfCommonPrefix)
	if l > COMMON_TAGS_COUNT {
		l = COMMON_TAGS_COUNT
	}

	m.log.Warnf("WARNING: Found more than %v tag entries per document, which may cause performance issues with aggregate logs. List of most common tag-prefix: [%v]. You can ignore these tags using ignore_tag_prefix_list option", thresholdLenTagList, strings.Join(listOfCommonPrefix[:l], ", "))
}

func (m *MongoAggregatePump) doHash(in string) string {
	sEnc := b64.StdEncoding.EncodeToString([]byte(in))
	search := strings.TrimRight(sEnc, "=")
	return search
}

func (m *MongoAggregatePump) GetName() string {
	return "MongoDB Aggregate Pump"
}

func (m *MongoAggregatePump) GetEnvPrefix() string {
	return m.dbConf.EnvPrefix
}

func (m *MongoAggregatePump) GetCollectionName(orgid string) (string, error) {
	if orgid == "" {
		return "", errors.New("OrgID cannot be empty")
	}

	return "z_tyk_analyticz_aggregate_" + orgid, nil
}

func (m *MongoAggregatePump) Init(config interface{}) error {
	m.dbConf = &MongoAggregateConf{}
	m.log = log.WithField("prefix", analytics.MongoAggregatePrefix)

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
	}

	if err != nil {
		m.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(m, m.log, m.dbConf, mongoAggregateDefaultEnv)

	//we keep this env check for backward compatibility
	overrideErr := envconfig.Process(mongoAggregatePumpPrefix, m.dbConf)
	if overrideErr != nil {
		m.log.Error("Failed to process environment variables for mongo aggregate pump: ", overrideErr)
	}

	if m.dbConf.ThresholdLenTagList == 0 {
		m.dbConf.ThresholdLenTagList = THRESHOLD_LEN_TAG_LIST
	}

	m.connect()

	m.log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.log.Info(m.GetName() + " Initialized")

	return nil
}

func (m *MongoAggregatePump) connect() {
	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongoDialInfo(m.dbConf.BaseMongoConf)
	if err != nil {
		m.log.Panic("Mongo URL is invalid: ", err)
	}

	if m.timeout > 0 {
		dialInfo.Timeout = time.Second * time.Duration(m.timeout)
	}

	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	for err != nil {
		m.log.WithError(err).WithField("dialinfo", dialInfo).Error("Mongo connection failed. Retrying.")
		time.Sleep(5 * time.Second)
		m.dbSession, err = mgo.DialWithInfo(dialInfo)
	}

	if err == nil && m.dbConf.MongoDBType == 0 {
		m.dbConf.MongoDBType = mongoType(m.dbSession)
	}
}

func (m *MongoAggregatePump) ensureIndexes(c *mgo.Collection) error {
	var err error
	ttlIndex := mgo.Index{
		Key:         []string{"expireAt"},
		ExpireAfter: 0,
		Background:  m.dbConf.MongoDBType == StandardMongo,
	}

	err = mgohacks.EnsureTTLIndex(c, ttlIndex)
	if err != nil {
		return err
	}

	apiIndex := mgo.Index{
		Key:        []string{"timestamp"},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(apiIndex)
	if err != nil {
		return err
	}

	orgIndex := mgo.Index{
		Key:        []string{"orgid"},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	return c.EnsureIndex(orgIndex)
}

func (m *MongoAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	m.log.Debug("Attempting to write ", len(data), " records")

	if m.dbSession == nil {
		m.log.Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(ctx, data)
	} else {
		// calculate aggregates
		analyticsPerOrg := analytics.AggregateData(data, m.dbConf.TrackAllPaths, m.dbConf.IgnoreTagPrefixList, m.dbConf.StoreAnalyticsPerMinute)

		// put aggregated data into MongoDB
		for orgID, filteredData := range analyticsPerOrg {
			collectionName, collErr := m.GetCollectionName(orgID)
			if collErr != nil {
				m.log.Info("No OrgID for AnalyticsRecord, skipping")
				continue
			}

			thisSession := m.dbSession.Copy()
			defer thisSession.Close()

			analyticsCollection := thisSession.DB("").C(collectionName)
			indexCreateErr := m.ensureIndexes(analyticsCollection)

			if indexCreateErr != nil {
				m.log.Error(indexCreateErr)
			}

			query := bson.M{
				"orgid":     filteredData.OrgID,
				"timestamp": filteredData.TimeStamp,
			}

			if len(m.dbConf.IgnoreAggregationsList) > 0 {
				filteredData.DiscardAggregations(m.dbConf.IgnoreAggregationsList)
			}

			updateDoc := filteredData.AsChange()

			change := mgo.Change{
				Update:    updateDoc,
				ReturnNew: true,
				Upsert:    true,
			}

			doc := analytics.AnalyticsRecordAggregate{}
			_, err := analyticsCollection.Find(query).Apply(change, &doc)

			if err != nil {
				m.log.WithField("query", query).Error("UPSERT Failure: ", err)
				return m.HandleWriteErr(err)
			}

			// We have the new doc back, lets fix the averages
			avgUpdateDoc := doc.AsTimeUpdate()
			avgChange := mgo.Change{
				Update:    avgUpdateDoc,
				ReturnNew: true,
			}
			withTimeUpdate := analytics.AnalyticsRecordAggregate{}
			_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &withTimeUpdate)

			if m.dbConf.ThresholdLenTagList != -1 && (len(withTimeUpdate.Tags) > m.dbConf.ThresholdLenTagList) {
				m.printAlert(withTimeUpdate, m.dbConf.ThresholdLenTagList)
			}

			if avgErr != nil {
				m.log.WithField("query", query).Error("AvgUpdate Failure: ", avgErr)
				return m.HandleWriteErr(avgErr)
			}

			m.log.WithFields(logrus.Fields{
				"collection": collectionName,
			}).Debug("Wrote aggregated data for ", len(data), " records")

			if m.dbConf.UseMixedCollection {
				thisData := analytics.AnalyticsRecordAggregate{}
				err := analyticsCollection.Find(query).One(&thisData)
				if err != nil {
					m.log.WithField("query", query).Error("Couldn't find query doc:", err)
				} else {
					m.doMixedWrite(thisData, query)
				}
			}
		}
	}
	m.log.Info("Purged ", len(data), " records...")

	return nil
}

func (m *MongoAggregatePump) doMixedWrite(changeDoc analytics.AnalyticsRecordAggregate, query bson.M) {
	thisSession := m.dbSession.Copy()
	defer thisSession.Close()
	analyticsCollection := thisSession.DB("").C(analytics.AgggregateMixedCollectionName)
	m.ensureIndexes(analyticsCollection)

	if len(m.dbConf.IgnoreAggregationsList) > 0 {
		changeDoc.DiscardAggregations(m.dbConf.IgnoreAggregationsList)
	}

	avgChange := mgo.Change{
		Update:    changeDoc,
		ReturnNew: true,
		Upsert:    true,
	}

	m.log.WithFields(logrus.Fields{
		"collection": analytics.AgggregateMixedCollectionName,
	}).Debug("Attempt to upsert aggregated doc")

	final := analytics.AnalyticsRecordAggregate{}
	_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &final)

	if avgErr != nil {
		m.log.WithFields(logrus.Fields{
			"collection": analytics.AgggregateMixedCollectionName,
		}).Error("Mixed coll upsert failure: ", avgErr)
		m.HandleWriteErr(avgErr)
	}
	m.log.WithFields(logrus.Fields{
		"collection": analytics.AgggregateMixedCollectionName,
	}).Info("Completed upserting")
}

func (m *MongoAggregatePump) HandleWriteErr(err error) error {
	if err != nil {
		m.log.Error("Problem inserting or updating to mongo collection: ", err)
		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			m.log.Warning("--> Detected connection failure, reconnecting")
			m.connect()
		}
	}
	return err
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoAggregatePump) WriteUptimeData(data []interface{}) {
	m.log.Warning("Mongo Aggregate should not be writing uptime data!")
}
