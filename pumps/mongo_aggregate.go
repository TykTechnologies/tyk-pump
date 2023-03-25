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

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
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

// @PumpConf MongoAggregate
type MongoAggregateConf struct {
	// TYKCONFIGEXPAND
	BaseMongoConf
	// If set to `true` your pump will store analytics to both your organisation defined
	// collections z_tyk_analyticz_aggregate_{ORG ID} and your org-less tyk_analytics_aggregates
	// collection. When set to 'false' your pump will only store analytics to your org defined
	// collection.
	UseMixedCollection bool `json:"use_mixed_collection" mapstructure:"use_mixed_collection"`
	// Specifies if it should store aggregated data for all the endpoints. By default, `false`
	// which means that only store aggregated data for `tracked endpoints`.
	TrackAllPaths bool `json:"track_all_paths" mapstructure:"track_all_paths"`
	// Specifies prefixes of tags that should be ignored.
	IgnoreTagPrefixList []string `json:"ignore_tag_prefix_list" mapstructure:"ignore_tag_prefix_list"`
	// Determines the threshold of amount of tags of an aggregation. If the amount of tags is superior to the threshold,
	// it will print an alert.
	// Defaults to 1000.
	ThresholdLenTagList int `json:"threshold_len_tag_list" mapstructure:"threshold_len_tag_list"`
	// Determines if the aggregations should be made per minute (true) or per hour (false).
	StoreAnalyticsPerMinute bool `json:"store_analytics_per_minute" mapstructure:"store_analytics_per_minute"`
	// Determines the amount of time the aggregations should be made (in minutes). It defaults to the max value is 60 and the minimum is 1.
	// If StoreAnalyticsPerMinute is set to true, this field will be skipped.
	AggregationTime int `json:"aggregation_time" mapstructure:"aggregation_time"`
	// Determines if the self healing will be activated or not.
	// Self Healing allows pump to handle Mongo document's max-size errors by creating a new document when the max-size is reached.
	// It also divide by 2 the AggregationTime field to avoid the same error in the future.
	EnableAggregateSelfHealing bool `json:"enable_aggregate_self_healing" mapstructure:"enable_aggregate_self_healing"`
	// This list determines which aggregations are going to be dropped and not stored in the collection.
	// Posible values are: "APIID","errors","versions","apikeys","oauthids","geo","tags","endpoints","keyendpoints",
	// "oauthendpoints", and "apiendpoints".
	IgnoreAggregationsList []string `json:"ignore_aggregations" mapstructure:"ignore_aggregations"`
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
	m.SetAggregationTime()

	m.connect()

	m.log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.log.Info(m.GetName() + " Initialized")

	// look for the last record timestamp stored in the collection
	lastTimestampAgggregateRecord, err := getLastDocumentTimestamp(m.dbSession, analytics.AgggregateMixedCollectionName)

	// we will set it to the lastDocumentTimestamp map to track the timestamp of different documents of different Mongo Aggregators
	if err != nil {
		m.log.Debug("Last document timestamp not found:", err)
	} else {
		analytics.SetlastTimestampAgggregateRecord(m.dbConf.MongoURL, lastTimestampAgggregateRecord)
	}

	return nil
}

func (m *MongoAggregatePump) connect() {
	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongoDialInfo(m.dbConf.BaseMongoConf)
	if err != nil {
		m.log.Panic("Mongo URL is invalid: ", err)
	}

	timeout := MongoDefaultConnTimeout
	if m.dbConf.ConnectionTimeout > 0 {
		timeout = m.dbConf.ConnectionTimeout
	}
	dialInfo.Timeout = time.Duration(timeout) * time.Second

	m.log.Info("Connecting to Mongo...")

	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	for err != nil {
		m.log.WithError(err).WithField("dialinfo", m.dbConf.BaseMongoConf.GetBlurredURL()).Error("Mongo connection failed. Retrying.")
		time.Sleep(5 * time.Second)
		m.dbSession, err = mgo.DialWithInfo(dialInfo)
	}

	if err == nil && m.dbConf.MongoDBType == 0 {
		m.dbConf.MongoDBType = mongoType(m.dbSession)
	}

	m.dbConf.SetMongoConsistency(m.dbSession)
}

func (m *MongoAggregatePump) ensureIndexes(c *mgo.Collection) error {
	if m.dbConf.OmitIndexCreation {
		m.log.Debug("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	//We are going to check if the collection exists only when the DB Type is MongoDB. The mgo CollectionNames func leaks cursors on DocDB.
	if m.dbConf.MongoDBType == StandardMongo {
		exists, errExists := m.collectionExists(c.Name)
		if errExists == nil && exists {
			m.log.Debug("Collection ", c.Name, " exists, omitting index creation")
			return nil
		}
	}

	var err error
	// CosmosDB does not support "expireAt" option
	if m.dbConf.MongoDBType != CosmosDB {
		ttlIndex := mgo.Index{
			Key:         []string{"expireAt"},
			ExpireAfter: 0,
			Background:  m.dbConf.MongoDBType == StandardMongo,
		}

		err = mgohacks.EnsureTTLIndex(c, ttlIndex)
		if err != nil {
			return err
		}
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
		analyticsPerOrg := analytics.AggregateData(data, m.dbConf.TrackAllPaths, m.dbConf.IgnoreTagPrefixList, m.dbConf.MongoURL, m.dbConf.AggregationTime)
		// put aggregated data into MongoDB
		for orgID, filteredData := range analyticsPerOrg {
			err := m.DoAggregatedWriting(ctx, orgID, filteredData)
			if err != nil {
				// checking if the error is related to the document size and AggregateSelfHealing is enabled
				if shouldSelfHeal := m.ShouldSelfHeal(err); shouldSelfHeal {
					// executing the function again with the new AggregationTime setting
					newErr := m.WriteData(ctx, data)
					if newErr == nil {
						m.log.Info("Self-healing successful")
					}
					return newErr
				}
				return err
			}

			m.log.Debug("Processed aggregated data for ", orgID)
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

func (m *MongoAggregatePump) DoAggregatedWriting(ctx context.Context, orgID string, filteredData analytics.AnalyticsRecordAggregate) error {
	collectionName, collErr := m.GetCollectionName(orgID)
	if collErr != nil {
		m.log.Info("No OrgID for AnalyticsRecord, skipping")
		return nil
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

	if m.dbConf.UseMixedCollection {
		thisData := analytics.AnalyticsRecordAggregate{}
		err := analyticsCollection.Find(query).One(&thisData)
		if err != nil {
			m.log.WithField("query", query).Error("Couldn't find query doc:", err)
		} else {
			m.doMixedWrite(thisData, query)
		}
	}
	return nil
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoAggregatePump) collectionExists(name string) (bool, error) {
	sess := m.dbSession.Copy()
	defer sess.Close()

	colNames, err := sess.DB("").CollectionNames()
	if err != nil {
		m.log.Error("Unable to get collection names: ", err)

		return false, err
	}

	for _, coll := range colNames {
		if coll == name {
			return true, nil
		}
	}

	return false, nil
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoAggregatePump) WriteUptimeData(data []interface{}) {
	m.log.Warning("Mongo Aggregate should not be writing uptime data!")
}

// getLastDocumentTimestamp will return the timestamp of the last document in the collection
func getLastDocumentTimestamp(session *mgo.Session, collectionName string) (time.Time, error) {
	var doc bson.M
	err := session.DB("").C(collectionName).Find(nil).Sort("-$natural").One(&doc)
	if err != nil {
		return time.Time{}, err
	}
	if ts, ok := doc["timestamp"].(time.Time); ok {
		return ts, nil
	}
	return time.Time{}, errors.New("timestamp of type: time.Time not found in bson map")
}

// divideAggregationTime divides by two the analytics stored per minute setting
func (m *MongoAggregatePump) divideAggregationTime() {
	if m.dbConf.AggregationTime == 1 {
		m.log.Debug("Analytics Stored Per Minute is set to 1, unable to divide")
		return
	}
	oldAggTime := m.dbConf.AggregationTime
	m.dbConf.AggregationTime /= 2
	m.log.Warn("Analytics Stored Per Minute dicreased from ", oldAggTime, " to ", m.dbConf.AggregationTime)
}

// ShouldSelfHeal returns true if the pump should self heal
func (m *MongoAggregatePump) ShouldSelfHeal(err error) bool {
	const StandardMongoSizeError = "Size must be between 0 and"
	const CosmosSizeError = "Request size is too large"
	const DocDBSizeError = "Resulting document after update is larger than"

	if m.dbConf.EnableAggregateSelfHealing {
		if strings.Contains(err.Error(), StandardMongoSizeError) || strings.Contains(err.Error(), CosmosSizeError) || strings.Contains(err.Error(), DocDBSizeError) {
			// if the AggregationTime setting is already set to 1, we can't do anything else
			if m.dbConf.AggregationTime == 1 {
				m.log.Warning("AggregationTime is equal to 1 minute, unable to reduce it further. Skipping self-healing.")
				return false
			}
			m.log.Warning("Detected document size failure, attempting to create a new document and reduce the number of analytics stored per minute")
			// dividing the AggregationTime by 2 to reduce the number of analytics stored per minute
			m.divideAggregationTime()
			// resetting the lastDocumentTimestamp, this will create a new document with the current timestamp
			analytics.SetlastTimestampAgggregateRecord(m.dbConf.MongoURL, time.Time{})
			return true
		}
	}
	return false
}

// SetAggregationTime sets the aggregation time for the pump
func (m *MongoAggregatePump) SetAggregationTime() {
	// if StoreAnalyticsPerMinute is set to true, the aggregation time will be set to 1.
	// if not, the aggregation time will be set to the value of the field AggregationTime.
	// if there is no value for AggregationTime, it will be set to 60.

	if m.dbConf.StoreAnalyticsPerMinute {
		m.log.Info("StoreAnalyticsPerMinute is set to true. Pump will aggregate data every minute.")
		m.dbConf.AggregationTime = 1
	} else if m.dbConf.AggregationTime < 1 || m.dbConf.AggregationTime > 60 {
		m.log.Warn("AggregationTime is not set or is not between 1 and 60. Defaulting to 60")
		m.dbConf.AggregationTime = 60
	}
}
