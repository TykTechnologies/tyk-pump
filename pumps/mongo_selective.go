package pumps

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/pumps/common"
	"github.com/TykTechnologies/tyk-pump/pumps/mongo"
	"github.com/kelseyhightower/envconfig"
	"github.com/lonelycode/mgohacks"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/vmihailenco/msgpack.v2"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type MongoSelectivePump struct {
	dbSession *mgo.Session
	dbConf    *MongoSelectiveConf
	common.Pump
}

var mongoSelectivePrefix = "mongo-pump-selective"
var mongoSelectivePumpPrefix = "PMP_MONGOSEL"
var mongoSelectiveDefaultEnv = common.PUMPS_ENV_PREFIX + "_MONGOSELECTIVE" + common.PUMPS_ENV_META_PREFIX

// @PumpConf MongoSelective
type MongoSelectiveConf struct {
	// TYKCONFIGEXPAND
	mongo.BaseMongoConf
	// Maximum insert batch size for mongo selective pump. If the batch we are writing surpass this value, it will be send in multiple batchs.
	// Defaults to 10Mb.
	MaxInsertBatchSizeBytes int `json:"max_insert_batch_size_bytes" mapstructure:"max_insert_batch_size_bytes"`
	// Maximum document size. If the document exceed this value, it will be skipped.
	// Defaults to 10Mb.
	MaxDocumentSizeBytes int `json:"max_document_size_bytes" mapstructure:"max_document_size_bytes"`
}

func (m *MongoSelectivePump) GetName() string {
	return "MongoDB Selective Pump"
}

func (m *MongoSelectivePump) GetEnvPrefix() string {
	return m.dbConf.EnvPrefix
}

func (m *MongoSelectivePump) GetCollectionName(orgid string) (string, error) {
	if orgid == "" {
		return "", errors.New("OrgID cannot be empty")
	}
	return "z_tyk_analyticz_" + orgid, nil
}

func (m *MongoSelectivePump) Init(config interface{}) error {
	m.dbConf = &MongoSelectiveConf{}
	m.Log = log.WithField("prefix", mongoSelectivePrefix)

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
	}

	if err != nil {
		m.Log.Fatal("Failed to decode configuration: ", err)
	}

	m.ProcessEnvVars(m.Log, m.dbConf, mongoSelectiveDefaultEnv)

	//we keep this env check for backward compatibility
	overrideErr := envconfig.Process(mongoSelectivePumpPrefix, m.dbConf)
	if overrideErr != nil {
		m.Log.Error("Failed to process environment variables for mongo selective pump: ", overrideErr)
	}

	if m.dbConf.MaxInsertBatchSizeBytes == 0 {
		m.Log.Info("-- No max batch size set, defaulting to 10MB")
		m.dbConf.MaxInsertBatchSizeBytes = 10 * mongo.MiB
	}

	if m.dbConf.MaxDocumentSizeBytes == 0 {
		m.Log.Info("-- No max document size set, defaulting to 10MB")
		m.dbConf.MaxDocumentSizeBytes = 10 * mongo.MiB
	}

	m.connect()

	m.Log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.Log.Info(m.GetName() + " Initialized")

	return nil
}

func (m *MongoSelectivePump) connect() {
	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongo.DialInfo(m.dbConf.BaseMongoConf)
	if err != nil {
		m.Log.Panic("Mongo URL is invalid: ", err)
	}

	if m.Timeout > 0 {
		dialInfo.Timeout = time.Second * time.Duration(m.Timeout)
	}

	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	for err != nil {
		m.Log.WithError(err).WithField("dialinfo", m.dbConf.BaseMongoConf.GetBlurredURL()).Error("Mongo connection failed. Retrying.")
		time.Sleep(5 * time.Second)
		m.dbSession, err = mgo.DialWithInfo(dialInfo)
	}

	if err == nil && m.dbConf.MongoDBType == 0 {
		m.dbConf.MongoDBType = mongo.GetMongoType(m.dbSession)
	}
}

func (m *MongoSelectivePump) ensureIndexes(c *mgo.Collection) error {
	if m.dbConf.OmitIndexCreation {
		m.Log.Debug("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	if m.dbConf.MongoDBType == mongo.StandardMongo {
		exists, errExists := m.collectionExists(c.Name)
		if errExists == nil && exists {
			m.Log.Debug("Collection ", c.Name, " exists, omitting index creation")
			return nil
		}
	}

	var err error
	ttlIndex := mgo.Index{
		Key:         []string{"expireAt"},
		ExpireAfter: 0,
		Background:  m.dbConf.MongoDBType == mongo.StandardMongo,
	}

	err = mgohacks.EnsureTTLIndex(c, ttlIndex)
	if err != nil {
		return err
	}

	apiIndex := mgo.Index{
		Key:        []string{"apiid"},
		Background: m.dbConf.MongoDBType == mongo.StandardMongo,
	}

	err = c.EnsureIndex(apiIndex)
	if err != nil {
		return err
	}

	logBrowserIndex := mgo.Index{
		Name:       "logBrowserIndex",
		Key:        []string{"-timestamp", "apiid", "apikey", "responsecode"},
		Background: m.dbConf.MongoDBType == mongo.StandardMongo,
	}

	err = c.EnsureIndex(logBrowserIndex)
	if err != nil && !strings.Contains(err.Error(), "already exists with a different name") {
		return err
	}

	return nil
}

func (m *MongoSelectivePump) WriteData(ctx context.Context, data []interface{}) error {
	m.Log.Debug("Attempting to write ", len(data), " records...")

	if m.dbSession == nil {
		m.Log.Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(ctx, data)
	} else {
		analyticsPerOrg := make(map[string][]interface{})

		for _, v := range data {
			orgID := v.(analytics.AnalyticsRecord).OrgID
			collectionName, collErr := m.GetCollectionName(orgID)
			skip := false
			if collErr != nil {
				m.Log.Warning("No OrgID for AnalyticsRecord, skipping")
				skip = true
			}

			if !skip {
				_, found := analyticsPerOrg[collectionName]
				if !found {
					analyticsPerOrg[collectionName] = []interface{}{v}
				} else {
					analyticsPerOrg[collectionName] = append(analyticsPerOrg[collectionName], v)
				}
			}
		}

		for col_name, filtered_data := range analyticsPerOrg {

			for _, dataSet := range m.AccumulateSet(filtered_data) {
				thisSession := m.dbSession.Copy()
				defer thisSession.Close()
				analyticsCollection := thisSession.DB("").C(col_name)

				indexCreateErr := m.ensureIndexes(analyticsCollection)
				if indexCreateErr != nil {
					m.Log.WithField("collection", col_name).Error(indexCreateErr)
				}

				err := analyticsCollection.Insert(dataSet...)
				if err != nil {
					m.Log.WithField("collection", col_name).Error("Problem inserting to mongo collection: ", err)
					if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
						m.Log.Warning("--> Detected connection failure, reconnecting")
						m.connect()
					}
				}
			}

		}

	}
	m.Log.Info("Purged ", len(data), " records...")

	return nil
}

func (m *MongoSelectivePump) AccumulateSet(data []interface{}) [][]interface{} {
	accumulatorTotal := 0
	returnArray := make([][]interface{}, 0)

	thisResultSet := make([]interface{}, 0)
	for i, item := range data {
		thisItem := item.(analytics.AnalyticsRecord)
		if thisItem.ResponseCode == -1 {
			continue
		}
		sizeBytes := len([]byte(thisItem.RawRequest)) + len([]byte(thisItem.RawRequest))

		skip := false
		if sizeBytes > m.dbConf.MaxDocumentSizeBytes {
			m.Log.Warning("Document too large, skipping!")
			skip = true
		}

		m.Log.Debug("Size is: ", sizeBytes)

		if !skip {
			if (accumulatorTotal + sizeBytes) < m.dbConf.MaxInsertBatchSizeBytes {
				accumulatorTotal += sizeBytes
			} else {
				m.Log.Debug("Created new chunk entry")
				if len(thisResultSet) > 0 {
					returnArray = append(returnArray, thisResultSet)
				}

				thisResultSet = make([]interface{}, 0)
				accumulatorTotal = sizeBytes
			}
			thisResultSet = append(thisResultSet, thisItem)

			m.Log.Debug(accumulatorTotal, " of ", m.dbConf.MaxInsertBatchSizeBytes)
			// Append the last element if the loop is about to end
			if i == (len(data) - 1) {
				m.Log.Debug("Appending last entry")
				returnArray = append(returnArray, thisResultSet)
			}
		}

	}

	return returnArray
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoSelectivePump) WriteUptimeData(data []interface{}) {
	if m.dbSession == nil {
		m.Log.Debug("Connecting to mongoDB store")
		m.connect()
		m.WriteUptimeData(data)
	} else {
		m.Log.Info("MONGO Selective Should not be writing uptime data!")
		collectionName := "tyk_uptime_analytics"
		thisSession := m.dbSession.Copy()
		defer thisSession.Close()
		analyticsCollection := thisSession.DB("").C(collectionName)

		m.Log.Debug("Uptime Data: ", len(data))

		if len(data) > 0 {
			keys := make([]interface{}, len(data))

			for i, v := range data {
				decoded := analytics.UptimeReportData{}
				// ToDo: should this work with serializer?
				err := msgpack.Unmarshal(v.([]byte), &decoded)
				m.Log.Debug("Decoded Record: ", decoded)
				if err != nil {
					m.Log.Error("Couldn't unmarshal analytics data:", err)
				} else {
					keys[i] = interface{}(decoded)
				}
			}

			err := analyticsCollection.Insert(keys...)
			m.Log.Debug("Wrote data to ", collectionName)

			if err != nil {
				m.Log.WithField("collection", collectionName).Error("Problem inserting to mongo collection: ", err)
				if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
					m.Log.Warning("--> Detected connection failure, reconnecting")
					m.connect()
				}
			}
		}
	}
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoSelectivePump) collectionExists(name string) (bool, error) {
	sess := m.dbSession.Copy()
	defer sess.Close()

	colNames, err := sess.DB("").CollectionNames()
	if err != nil {
		m.Log.Error("Unable to get collection names: ", err)

		return false, err
	}

	for _, coll := range colNames {
		if coll == name {
			return true, nil
		}
	}

	return false, nil
}
