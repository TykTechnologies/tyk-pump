package pumps

import (
	"context"
	b64 "encoding/base64"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

var (
	mongoAggregatePumpPrefix = "PMP_MONGOAGG"
	mongoAggregateDefaultEnv = PUMPS_ENV_PREFIX + "_MONGOAGGREGATE" + PUMPS_ENV_META_PREFIX
)

var (
	ThresholdLenTagList = 1000
	CommonTagsCount     = 5
)

type MongoAggregatePump struct {
	store  persistent.PersistentStorage
	dbConf *MongoAggregateConf
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
	if l > CommonTagsCount {
		l = CommonTagsCount
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

func (m *MongoAggregatePump) SetDecodingRequest(decoding bool) {
	if decoding {
		log.WithField("pump", m.GetName()).Warn("Decoding request is not supported for Mongo Aggregate pump")
	}
}

func (m *MongoAggregatePump) SetDecodingResponse(decoding bool) {
	if decoding {
		log.WithField("pump", m.GetName()).Warn("Decoding response is not supported for Mongo Aggregate pump")
	}
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

	// we keep this env check for backward compatibility
	overrideErr := envconfig.Process(mongoAggregatePumpPrefix, m.dbConf)
	if overrideErr != nil {
		m.log.Error("Failed to process environment variables for mongo aggregate pump: ", overrideErr)
	}

	if m.dbConf.ThresholdLenTagList == 0 {
		m.dbConf.ThresholdLenTagList = ThresholdLenTagList
	}
	m.SetAggregationTime()

	m.connect()

	m.log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.log.Info(m.GetName() + " Initialized")

	// look for the last record timestamp stored in the collection
	lastTimestampAgggregateRecord, err := m.getLastDocumentTimestamp()

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

	if m.dbConf.MongoDriverType == "" {
		// Default to mgo
		m.dbConf.MongoDriverType = persistent.Mgo
	}

	m.store, err = persistent.NewPersistentStorage(&persistent.ClientOpts{
		ConnectionString:         m.dbConf.MongoURL,
		UseSSL:                   m.dbConf.MongoUseSSL,
		SSLInsecureSkipVerify:    m.dbConf.MongoSSLInsecureSkipVerify,
		SSLAllowInvalidHostnames: m.dbConf.MongoSSLAllowInvalidHostnames,
		SSLCAFile:                m.dbConf.MongoSSLCAFile,
		SSLPEMKeyfile:            m.dbConf.MongoSSLPEMKeyfile,
		SessionConsistency:       m.dbConf.MongoSessionConsistency,
		ConnectionTimeout:        m.timeout,
		Type:                     m.dbConf.MongoDriverType,
		DirectConnection:         m.dbConf.MongoDirectConnection,
	})
	if err != nil {
		m.log.Fatal("Failed to connect to mongo: ", err)
	}
}

func (m *MongoAggregatePump) ensureIndexes(collectionName string) error {
	if m.dbConf.OmitIndexCreation {
		m.log.Debug("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	// We are going to check if the collection exists only when the DB Type is MongoDB. The mgo CollectionNames func leaks cursors on DocDB.
	if m.dbConf.MongoDBType == StandardMongo {
		exists, errExists := m.collectionExists(collectionName)
		if errExists == nil && exists {
			m.log.Debug("Collection ", collectionName, " exists, omitting index creation")
			return nil
		}
	}
	d := dbObject{
		tableName: collectionName,
	}
	var err error
	// CosmosDB does not support "expireAt" option
	if m.dbConf.MongoDBType != CosmosDB {
		ttlIndex := model.Index{
			Keys:       []model.DBM{{"expireAt": 1}},
			TTL:        0,
			IsTTLIndex: true,
			Background: m.dbConf.MongoDBType == StandardMongo,
		}
		err = m.store.CreateIndex(context.Background(), d, ttlIndex)
		if err != nil {
			return err
		}
	}

	apiIndex := model.Index{
		Keys:       []model.DBM{{"timestamp": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = m.store.CreateIndex(context.Background(), d, apiIndex)
	if err != nil {
		return err
	}

	orgIndex := model.Index{
		Keys:       []model.DBM{{"orgid": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}
	return m.store.CreateIndex(context.Background(), d, orgIndex)
}

func (m *MongoAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	m.log.Debug("Attempting to write ", len(data), " records")
	// calculate aggregates
	analyticsPerOrg := analytics.AggregateData(data, m.dbConf.TrackAllPaths, m.dbConf.IgnoreTagPrefixList, m.dbConf.MongoURL, m.dbConf.AggregationTime)
	// put aggregated data into MongoDB
	writingAttempts := []bool{false}
	if m.dbConf.UseMixedCollection {
		writingAttempts = append(writingAttempts, true)
	}
	for orgID := range analyticsPerOrg {
		filteredData := analyticsPerOrg[orgID]
		for _, isMixedCollection := range writingAttempts {
			err := m.DoAggregatedWriting(ctx, &filteredData, isMixedCollection)
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
		}
		m.log.Debug("Processed aggregated data for ", orgID)
	}

	m.log.Info("Purged ", len(data), " records...")

	return nil
}

func (m *MongoAggregatePump) DoAggregatedWriting(ctx context.Context, filteredData *analytics.AnalyticsRecordAggregate, mixed bool) error {
	filteredData.Mixed = mixed
	indexCreateErr := m.ensureIndexes(filteredData.TableName())

	if indexCreateErr != nil {
		m.log.Error(indexCreateErr)
	}

	query := model.DBM{
		"orgid":     filteredData.OrgID,
		"timestamp": filteredData.TimeStamp,
	}

	if len(m.dbConf.IgnoreAggregationsList) > 0 {
		filteredData.DiscardAggregations(m.dbConf.IgnoreAggregationsList)
	}

	updateDoc := filteredData.AsChange()
	doc := &analytics.AnalyticsRecordAggregate{
		OrgID: filteredData.OrgID,
		Mixed: mixed,
	}

	m.log.WithFields(logrus.Fields{
		"collection": doc.TableName(),
	}).Debug("Attempt to upsert aggregated doc")

	err := m.store.Upsert(ctx, doc, query, updateDoc)
	if err != nil {
		m.log.WithField("query", query).Error("UPSERT Failure: ", err)
		return err
	}

	// We have the new doc back, lets fix the averages
	avgUpdateDoc := doc.AsTimeUpdate()

	withTimeUpdate := analytics.AnalyticsRecordAggregate{
		OrgID: filteredData.OrgID,
		Mixed: mixed,
	}

	err = m.store.Upsert(ctx, &withTimeUpdate, query, avgUpdateDoc)
	if err != nil {
		m.log.WithField("query", query).Error("AvgUpdate Failure: ", err)
		return err
	}
	if m.dbConf.ThresholdLenTagList != -1 && (len(withTimeUpdate.Tags) > m.dbConf.ThresholdLenTagList) {
		m.printAlert(withTimeUpdate, m.dbConf.ThresholdLenTagList)
	}

	return nil
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoAggregatePump) collectionExists(name string) (bool, error) {
	return m.store.HasTable(context.Background(), name)
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoAggregatePump) WriteUptimeData(data []interface{}) {
	m.log.Warning("Mongo Aggregate should not be writing uptime data!")
}

// getLastDocumentTimestamp will return the timestamp of the last document in the collection
func (m *MongoAggregatePump) getLastDocumentTimestamp() (time.Time, error) {
	d := dbObject{
		tableName: analytics.AgggregateMixedCollectionName,
	}

	var result model.DBM
	err := m.store.Query(context.Background(), d, &result, model.DBM{"_sort": "-$natural", "_limit": 1})
	if err != nil {
		return time.Time{}, err
	}
	if ts, ok := result["timestamp"].(time.Time); ok {
		return ts, nil
	}
	return time.Time{}, errors.New("timestamp of type: time.Time not found in query result")
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
