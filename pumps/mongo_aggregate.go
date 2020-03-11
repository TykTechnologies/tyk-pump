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
var THRESHOLD_LEN_TAG_LIST = 1000
var COMMON_TAGS_COUNT = 5

type MongoAggregatePump struct {
	dbSession *mgo.Session
	dbConf    *MongoAggregateConf
	filters   analytics.AnalyticsFilters
	timeout   int
}

type MongoAggregateConf struct {
	BaseMongoConf
	UseMixedCollection      bool     `mapstructure:"use_mixed_collection"`
	TrackAllPaths           bool     `mapstructure:"track_all_paths"`
	IgnoreTagPrefixList     []string `mapstructure:"ignore_tag_prefix_list"`
	ThresholdLenTagList     int      `mapstructure:"threshold_len_tag_list"`
	StoreAnalyticsPerMinute bool     `mapstructure:"store_analytics_per_minute"`
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

func printAlert(doc analytics.AnalyticsRecordAggregate, thresholdLenTagList int) {
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

	log.Warnf("WARNING: Found more than %v tag entries per document, which may cause performance issues with aggregate logs. List of most common tag-prefix: [%v]. You can ignore these tags using ignore_tag_prefix_list option", thresholdLenTagList, strings.Join(listOfCommonPrefix[:l], ", "))
}

func (m *MongoAggregatePump) doHash(in string) string {
	sEnc := b64.StdEncoding.EncodeToString([]byte(in))
	search := strings.TrimRight(sEnc, "=")
	return search
}

func (m *MongoAggregatePump) GetName() string {
	return "MongoDB Aggregate Pump"
}

func (m *MongoAggregatePump) GetCollectionName(orgid string) (string, error) {
	if orgid == "" {
		return "", errors.New("OrgID cannot be empty")
	}

	return "z_tyk_analyticz_aggregate_" + orgid, nil
}

func (m *MongoAggregatePump) Init(config interface{}) error {
	m.dbConf = &MongoAggregateConf{}
	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
	}

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	overrideErr := envconfig.Process(mongoAggregatePumpPrefix, m.dbConf)
	if overrideErr != nil {
		log.Error("Failed to process environment variables for mongo aggregate pump: ", overrideErr)
	}

	if m.dbConf.ThresholdLenTagList == 0 {
		m.dbConf.ThresholdLenTagList = THRESHOLD_LEN_TAG_LIST
	}

	m.connect()

	log.WithFields(logrus.Fields{
		"prefix": analytics.MongoAggregatePrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.MongoURL)

	return nil
}

func (m *MongoAggregatePump) connect() {
	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongoDialInfo(m.dbConf.BaseMongoConf)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Panic("Mongo URL is invalid: ", err)
	}

	dialInfo.Timeout = time.Second * 5
	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	for err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).WithError(err).WithField("dialinfo", dialInfo).Error("Mongo connection failed. Retrying.")
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
	log.WithFields(logrus.Fields{
		"prefix": analytics.MongoAggregatePrefix,
	}).Debug("Writing ", len(data), " records")

	if m.dbSession == nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(ctx, data)
	} else {
		// calculate aggregates
		analyticsPerOrg := analytics.AggregateData(data, m.dbConf.TrackAllPaths, m.dbConf.IgnoreTagPrefixList, m.dbConf.StoreAnalyticsPerMinute)

		// put aggregated data into MongoDB
		for orgID, filteredData := range analyticsPerOrg {
			collectionName, collErr := m.GetCollectionName(orgID)
			if collErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": analytics.MongoAggregatePrefix,
				}).Info("No OrgID for AnalyticsRecord, skipping")
				continue
			}

			thisSession := m.dbSession.Copy()
			defer thisSession.Close()

			analyticsCollection := thisSession.DB("").C(collectionName)
			indexCreateErr := m.ensureIndexes(analyticsCollection)

			if indexCreateErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": analytics.MongoAggregatePrefix,
				}).Error(indexCreateErr)
			}

			query := bson.M{
				"orgid":     filteredData.OrgID,
				"timestamp": filteredData.TimeStamp,
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
				log.WithFields(logrus.Fields{
					"prefix": analytics.MongoAggregatePrefix,
				}).Error("UPSERT Failure: ", err)
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
				printAlert(withTimeUpdate, m.dbConf.ThresholdLenTagList)
			}

			if avgErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": analytics.MongoAggregatePrefix,
				}).Error("AvgUpdate Failure: ", avgErr)
				return m.HandleWriteErr(avgErr)
			}

			if m.dbConf.UseMixedCollection {
				thisData := analytics.AnalyticsRecordAggregate{}
				err := analyticsCollection.Find(query).One(&thisData)
				if err != nil {
					log.Error("Couldn't find query doc!")
				} else {
					m.doMixedWrite(thisData, query)
				}

			}
		}
	}

	return nil
}

func (m *MongoAggregatePump) doMixedWrite(changeDoc analytics.AnalyticsRecordAggregate, query bson.M) {
	thisSession := m.dbSession.Copy()
	defer thisSession.Close()
	analyticsCollection := thisSession.DB("").C(analytics.AgggregateMixedCollectionName)
	m.ensureIndexes(analyticsCollection)

	avgChange := mgo.Change{
		Update:    changeDoc,
		ReturnNew: true,
		Upsert:    true,
	}

	final := analytics.AnalyticsRecordAggregate{}
	_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &final)

	if avgErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Error("Mixed coll upsert failure: ", avgErr)
		m.HandleWriteErr(avgErr)
	}
}

func (m *MongoAggregatePump) HandleWriteErr(err error) error {
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Error("Problem inserting or updating to mongo collection: ", err)
		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			log.WithFields(logrus.Fields{
				"prefix": analytics.MongoAggregatePrefix,
			}).Warning("--> Detected connection failure, reconnecting")
			m.connect()
		}
	}
	return err
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoAggregatePump) WriteUptimeData(data []interface{}) {
	log.WithFields(logrus.Fields{
		"prefix": analytics.MongoAggregatePrefix,
	}).Warning("Mongo Aggregate should not be writing uptime data!")
}

func (m *MongoAggregatePump) SetFilters(filters analytics.AnalyticsFilters) {
	m.filters = filters
}
func (m *MongoAggregatePump) GetFilters() analytics.AnalyticsFilters {
	return m.filters
}
func (m *MongoAggregatePump) SetTimeout(timeout int) {
	m.timeout = timeout
}

func (m *MongoAggregatePump) GetTimeout() int {
	return m.timeout
}
