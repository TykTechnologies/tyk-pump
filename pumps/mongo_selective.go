package pumps

import (
	"context"
	"errors"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"

	"gopkg.in/vmihailenco/msgpack.v2"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type MongoSelectivePump struct {
	store  persistent.PersistentStorage
	dbConf *MongoSelectiveConf
	CommonPumpConfig
}

var (
	mongoSelectivePrefix     = "mongo-pump-selective"
	mongoSelectivePumpPrefix = "PMP_MONGOSEL"
	mongoSelectiveDefaultEnv = PUMPS_ENV_PREFIX + "_MONGOSELECTIVE" + PUMPS_ENV_META_PREFIX
)

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

func (m *MongoSelectivePump) SetDecodingRequest(decoding bool) {
	if decoding {
		log.WithField("pump", m.GetName()).Warn("Decoding request is not supported for Mongo Selective pump")
	}
}

func (m *MongoSelectivePump) SetDecodingResponse(decoding bool) {
	if decoding {
		log.WithField("pump", m.GetName()).Warn("Decoding response is not supported for Mongo Selective pump")
	}
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

	// we keep this env check for backward compatibility
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
	d := dbObject{
		tableName: collectionName,
	}

	apiIndex := model.Index{
		Keys:       []model.DBM{{"apiid": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = m.store.CreateIndex(context.Background(), d, apiIndex)
	if err != nil {
		return err
	}

	// CosmosDB does not support "expireAt" option
	if m.dbConf.MongoDBType != CosmosDB {
		ttlIndex := model.Index{
			Keys:       []model.DBM{{"expireAt": 1}},
			IsTTLIndex: true,
			TTL:        0,
			Background: m.dbConf.MongoDBType == StandardMongo,
		}

		err = m.store.CreateIndex(context.Background(), d, ttlIndex)
		if err != nil {
			return err
		}
	}

	logBrowserIndex := model.Index{
		Name:       "logBrowserIndex",
		Keys:       []model.DBM{{"timestamp": -1}, {"apiid": 1}, {"apikey": 1}, {"responsecode": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = m.store.CreateIndex(context.Background(), d, logBrowserIndex)
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

	for colName, filteredData := range analyticsPerOrg {
		for _, dataSet := range m.AccumulateSet(filteredData, colName) {
			indexCreateErr := m.ensureIndexes(colName)
			if indexCreateErr != nil {
				m.log.WithField("collection", colName).Error(indexCreateErr)
			}
			err := m.store.Insert(context.Background(), dataSet...)
			if err != nil {
				m.log.WithField("collection", colName).Error("Problem inserting to mongo collection: ", err)
			}
		}
	}

	m.log.Info("Purged ", len(data), " records...")

	return nil
}

// AccumulateSet organizes analytics data into a set of chunks based on their size.
func (m *MongoSelectivePump) AccumulateSet(data []interface{}, collectionName string) [][]model.DBObject {
	accumulatorTotal := 0
	returnArray := make([][]model.DBObject, 0)
	thisResultSet := make([]model.DBObject, 0)

	// Process each item in the data array.
	for i, item := range data {
		thisItem, skip := m.processItem(item)
		if skip {
			continue
		}

		thisItem.CollectionName = collectionName

		sizeBytes := m.getItemSizeBytes(thisItem)
		accumulatorTotal, thisResultSet, returnArray = m.accumulate(thisResultSet, returnArray, thisItem, sizeBytes, accumulatorTotal, i == (len(data)-1))
	}

	return returnArray
}

// processItem checks if the item should be skipped or processed.
func (m *MongoSelectivePump) processItem(item interface{}) (*analytics.AnalyticsRecord, bool) {
	thisItem, ok := item.(analytics.AnalyticsRecord)
	if !ok {
		m.log.Warning("Couldn't convert item to analytics.AnalyticsRecord, skipping")
		return &thisItem, true
	}

	// Skip item if the response code is -1.
	if thisItem.ResponseCode == -1 {
		return &thisItem, true
	}

	return &thisItem, false
}

// getItemSizeBytes calculates the size of the analytics item in bytes and checks if it's within the allowed limit.
func (m *MongoSelectivePump) getItemSizeBytes(thisItem *analytics.AnalyticsRecord) int {
	// Add 1 KB for metadata as average.
	sizeBytes := len([]byte(thisItem.RawRequest)) + len([]byte(thisItem.RawResponse)) + 1024

	// Skip item if its size exceeds the maximum allowed document size.
	if sizeBytes > m.dbConf.MaxDocumentSizeBytes {
		m.log.Warning("Document too large, skipping!")
		return -1
	}

	m.log.Debug("Size is:", sizeBytes)
	return sizeBytes
}

// accumulate processes the given item and updates the accumulator total, result set, and return array.
// It manages chunking the data into separate sets based on the max batch size limit, and appends the last item when necessary.
func (m *MongoSelectivePump) accumulate(thisResultSet []model.DBObject, returnArray [][]model.DBObject, thisItem *analytics.AnalyticsRecord, sizeBytes, accumulatorTotal int, isLastItem bool) (int, []model.DBObject, [][]model.DBObject) {
	// If the item size is invalid (negative), return the current state
	if sizeBytes < 0 {
		return accumulatorTotal, thisResultSet, returnArray
	}

	// If the current accumulator total plus the item size is within the max batch size limit,
	// add the item size to the accumulator total
	if (accumulatorTotal + sizeBytes) < m.dbConf.MaxInsertBatchSizeBytes {
		accumulatorTotal += sizeBytes
	} else {
		// If the item size exceeds the max batch size limit,
		// create a new chunk entry and reset the accumulator total and result set
		m.log.Debug("Created new chunk entry")
		if len(thisResultSet) > 0 {
			returnArray = append(returnArray, thisResultSet)
		}

		thisResultSet = make([]model.DBObject, 0)
		accumulatorTotal = sizeBytes
	}

	thisResultSet = append(thisResultSet, thisItem)

	m.log.Debug(accumulatorTotal, " of ", m.dbConf.MaxInsertBatchSizeBytes)

	if isLastItem {
		m.log.Debug("Appending last entry")
		returnArray = append(returnArray, thisResultSet)
	}

	return accumulatorTotal, thisResultSet, returnArray
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoSelectivePump) WriteUptimeData(data []interface{}) {
	m.log.Info("MONGO Selective Should not be writing uptime data!")
	m.log.Debug("Uptime Data: ", len(data))

	if len(data) == 0 {
		return
	}

	keys := make([]model.DBObject, len(data))

	for i, v := range data {
		decoded := analytics.UptimeReportData{}

		if err := msgpack.Unmarshal([]byte(v.(string)), &decoded); err != nil {
			// ToDo: should this work with serializer?
			m.log.Error("Couldn't unmarshal analytics data:", err)
			continue
		}

		keys[i] = &decoded

		m.log.Debug("Decoded Record: ", decoded)
	}

	m.log.Debug("Writing data to ", analytics.UptimeSQLTable)

	if err := m.store.Insert(context.Background(), keys...); err != nil {
		m.log.Error("Problem inserting to mongo collection: ", err)
	}
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoSelectivePump) collectionExists(name string) (bool, error) {
	return m.store.HasTable(context.Background(), name)
}
