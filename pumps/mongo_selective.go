package pumps

import (
	"context"
	"errors"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"

	"gopkg.in/vmihailenco/msgpack.v2"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/dbm"
	"github.com/TykTechnologies/storage/persistent/id"
	"github.com/TykTechnologies/storage/persistent/index"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type MongoSelectivePump struct {
	store  persistent.PersistentStorage
	dbConf *MongoSelectiveConf
	CommonPumpConfig
}

func (MongoSelectivePump) GetObjectID() id.ObjectId {
	return ""
}

func (MongoSelectivePump) SetObjectID(id.ObjectId) {}

func (m MongoSelectivePump) TableName() string {
	return ""
}

var mongoSelectivePrefix = "mongo-pump-selective"
var mongoSelectivePumpPrefix = "PMP_MONGOSEL"
var mongoSelectiveDefaultEnv = PUMPS_ENV_PREFIX + "_MONGOSELECTIVE" + PUMPS_ENV_META_PREFIX

// @PumpConf MongoSelective
type MongoSelectiveConf struct {
	// TYKCONFIGEXPAND
	BaseMongoConf
	// Maximum insert batch size for mongo selective pump. If the batch we are writing surpass this value, it will be send in multiple batchs.
	// Defaults to 10Mb.
	MaxInsertBatchSizeBytes int `json:"max_insert_batch_size_bytes" mapstructure:"max_insert_batch_size_bytes"`
	// Maximum document size. If the document exceed this value, it will be skipped.
	// Defaults to 10Mb.
	MaxDocumentSizeBytes int `json:"max_document_size_bytes" mapstructure:"max_document_size_bytes"`
}

func (m *MongoSelectivePump) New() Pump {
	newPump := MongoSelectivePump{}
	return &newPump
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
	m.log = log.WithField("prefix", mongoSelectivePrefix)

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
	}

	if err != nil {
		m.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(m, m.log, m.dbConf, mongoSelectiveDefaultEnv)

	//we keep this env check for backward compatibility
	overrideErr := envconfig.Process(mongoSelectivePumpPrefix, m.dbConf)
	if overrideErr != nil {
		m.log.Error("Failed to process environment variables for mongo selective pump: ", overrideErr)
	}

	if m.dbConf.MaxInsertBatchSizeBytes == 0 {
		m.log.Info("-- No max batch size set, defaulting to 10MB")
		m.dbConf.MaxInsertBatchSizeBytes = 10 * MiB
	}

	if m.dbConf.MaxDocumentSizeBytes == 0 {
		m.log.Info("-- No max document size set, defaulting to 10MB")
		m.dbConf.MaxDocumentSizeBytes = 10 * MiB
	}

	m.connect()

	m.log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.log.Info(m.GetName() + " Initialized")

	return nil
}

func (m *MongoSelectivePump) connect() {
	var err error
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
	})
	if err != nil {
		m.log.Fatal("Failed to connect to mongo: ", err)
	}
}

func (m *MongoSelectivePump) ensureIndexes(collectionName string) error {
	if m.dbConf.OmitIndexCreation {
		m.log.Debug("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	if m.dbConf.MongoDBType == StandardMongo {
		exists, errExists := m.collectionExists(collectionName)
		if errExists == nil && exists {
			m.log.Debug("Collection ", collectionName, " exists, omitting index creation")
			return nil
		}
	}

	var err error
	// CosmosDB does not support "expireAt" option
	if m.dbConf.MongoDBType != CosmosDB {
		ttlIndex := index.Index{
			Keys:       []dbm.DBM{{"expireAt": 1}},
			IsTTLIndex: true,
			TTL:        0,
			Background: m.dbConf.MongoDBType == StandardMongo,
		}

		err = m.store.CreateIndex(context.Background(), m, ttlIndex)
	}

	apiIndex := index.Index{
		Keys:       []dbm.DBM{{"apiid": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = m.store.CreateIndex(context.Background(), m, apiIndex)
	if err != nil {
		return err
	}

	logBrowserIndex := index.Index{
		Name:       "logBrowserIndex",
		Keys:       []dbm.DBM{{"-timestamp": 1}, {"apiid": 1}, {"apikey": 1}, {"responsecode": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = m.store.CreateIndex(context.Background(), m, logBrowserIndex)
	if err != nil && !strings.Contains(err.Error(), "already exists with a different name") {
		return err
	}

	return nil
}

func (m *MongoSelectivePump) WriteData(ctx context.Context, data []interface{}) error {
	m.log.Debug("Attempting to write ", len(data), " records...")

	analyticsPerOrg := make(map[string][]interface{})

	for _, v := range data {
		orgID := v.(analytics.AnalyticsRecord).OrgID
		collectionName, collErr := m.GetCollectionName(orgID)
		skip := false
		if collErr != nil {
			m.log.Warning("No OrgID for AnalyticsRecord, skipping")
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
			indexCreateErr := m.ensureIndexes(m.TableName())
			if indexCreateErr != nil {
				m.log.WithField("collection", col_name).Error(indexCreateErr)
			}

			err := m.store.Insert(context.Background(), dataSet...)
			if err != nil {
				m.log.WithField("collection", col_name).Error("Problem inserting to mongo collection: ", err)
				if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
					m.log.Warning("--> Detected connection failure, reconnecting")
					m.connect()
				}
			}
		}

	}

	m.log.Info("Purged ", len(data), " records...")

	return nil
}

func (m *MongoSelectivePump) AccumulateSet(data []interface{}) [][]id.DBObject {
	accumulatorTotal := 0
	returnArray := make([][]id.DBObject, 0)

	thisResultSet := make([]id.DBObject, 0)
	for i, item := range data {
		thisItem := item.(analytics.AnalyticsRecord)
		if thisItem.ResponseCode == -1 {
			continue
		}
		// Add 1 KB for metadata as average
		sizeBytes := len([]byte(thisItem.RawRequest)) + len([]byte(thisItem.RawResponse)) + 1024

		skip := false
		if sizeBytes > m.dbConf.MaxDocumentSizeBytes {
			m.log.Warning("Document too large, skipping!")
			skip = true
		}

		m.log.Debug("Size is: ", sizeBytes)

		if !skip {
			if (accumulatorTotal + sizeBytes) < m.dbConf.MaxInsertBatchSizeBytes {
				accumulatorTotal += sizeBytes
			} else {
				m.log.Debug("Created new chunk entry")
				if len(thisResultSet) > 0 {
					returnArray = append(returnArray, thisResultSet)
				}

				thisResultSet = make([]id.DBObject, 0)
				accumulatorTotal = sizeBytes
			}
			thisResultSet = append(thisResultSet, thisItem)

			m.log.Debug(accumulatorTotal, " of ", m.dbConf.MaxInsertBatchSizeBytes)
			// Append the last element if the loop is about to end
			if i == (len(data) - 1) {
				m.log.Debug("Appending last entry")
				returnArray = append(returnArray, thisResultSet)
			}
		}

	}

	return returnArray
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoSelectivePump) WriteUptimeData(data []interface{}) {
	m.log.Info("MONGO Selective Should not be writing uptime data!")
	m.log.Debug("Uptime Data: ", len(data))
	collectionName := "tyk_uptime_analytics"

	if len(data) > 0 {
		keys := make([]id.DBObject, len(data))

		for i, v := range data {
			decoded := analytics.UptimeReportData{}
			// ToDo: should this work with serializer?
			err := msgpack.Unmarshal(v.([]byte), &decoded)
			m.log.Debug("Decoded Record: ", decoded)
			if err != nil {
				m.log.Error("Couldn't unmarshal analytics data:", err)
			} else {
				keys[i] = decoded
			}
		}

		err := m.store.Insert(context.Background(), keys...)
		m.log.Debug("Wrote data to ", collectionName)

		if err != nil {
			m.log.WithField("collection", collectionName).Error("Problem inserting to mongo collection: ", err)
			if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
				m.log.Warning("--> Detected connection failure, reconnecting")
				m.connect()
			}
		}

	}
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoSelectivePump) collectionExists(name string) (bool, error) {
	return m.store.HasTable(context.Background(), name)
}
