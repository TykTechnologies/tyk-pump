package pumps

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"

	"gopkg.in/vmihailenco/msgpack.v2"
)

const (
	_   = iota // ignore zero iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
	TiB
)

type MongoPump struct {
	IsUptime bool
	store    persistent.PersistentStorage
	dbConf   *MongoConf
	CommonPumpConfig
}

var (
	mongoPrefix     = "mongo-pump"
	mongoPumpPrefix = "PMP_MONGO"
	mongoDefaultEnv = PUMPS_ENV_PREFIX + "_MONGO" + PUMPS_ENV_META_PREFIX
)

type MongoType int

const (
	StandardMongo MongoType = iota
	AWSDocumentDB
	CosmosDB
)

const (
	AWSDBError    = 303
	CosmosDBError = 115
)

type BaseMongoConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The full URL to your MongoDB instance, this can be a clustered instance if necessary and
	// should include the database and username / password data.
	MongoURL string `json:"mongo_url" mapstructure:"mongo_url"`
	// Set to true to enable Mongo SSL connection.
	MongoUseSSL bool `json:"mongo_use_ssl" mapstructure:"mongo_use_ssl"`
	// Allows the use of self-signed certificates when connecting to an encrypted MongoDB database.
	MongoSSLInsecureSkipVerify bool `json:"mongo_ssl_insecure_skip_verify" mapstructure:"mongo_ssl_insecure_skip_verify"`
	// Ignore hostname check when it differs from the original (for example with SSH tunneling).
	// The rest of the TLS verification will still be performed.
	MongoSSLAllowInvalidHostnames bool `json:"mongo_ssl_allow_invalid_hostnames" mapstructure:"mongo_ssl_allow_invalid_hostnames"`
	// Path to the PEM file with trusted root certificates
	MongoSSLCAFile string `json:"mongo_ssl_ca_file" mapstructure:"mongo_ssl_ca_file"`
	// Path to the PEM file which contains both client certificate and private key. This is
	// required for Mutual TLS.
	MongoSSLPEMKeyfile string `json:"mongo_ssl_pem_keyfile" mapstructure:"mongo_ssl_pem_keyfile"`
	// Specifies the mongo DB Type. If it's 0, it means that you are using standard mongo db, if it's 1 it means you are using AWS Document DB, if it's 2, it means you are using CosmosDB.
	// Defaults to Standard mongo (0).
	MongoDBType MongoType `json:"mongo_db_type" mapstructure:"mongo_db_type"`
	// Set to true to disable the default tyk index creation.
	OmitIndexCreation bool `json:"omit_index_creation" mapstructure:"omit_index_creation"`
	// Set the consistency mode for the session, it defaults to `Strong`. The valid values are: strong, monotonic, eventual.
	MongoSessionConsistency string `json:"mongo_session_consistency" mapstructure:"mongo_session_consistency"`
	// MongoDriverType is the type of the driver (library) to use. The valid values are: “mongo-go” and “mgo”.
	// Default to “mongo-go”. Check out this guide to [learn about different MongoDB drivers Tyk Pump support](https://github.com/TykTechnologies/tyk-pump#driver-type).
	MongoDriverType string `json:"driver" mapstructure:"driver"`
	// MongoDirectConnection informs whether to establish connections only with the specified seed servers,
	// or to obtain information for the whole cluster and establish connections with further servers too.
	// If true, the client will only connect to the host provided in the ConnectionString
	// and won't attempt to discover other hosts in the cluster. Useful when network restrictions
	// prevent discovery, such as with SSH tunneling. Default is false.
	MongoDirectConnection bool `json:"mongo_direct_connection" mapstructure:"mongo_direct_connection"`
}
type dbObject struct {
	tableName string
}

func (d dbObject) TableName() string {
	return d.tableName
}

// GetObjectID is a dummy function to satisfy the interface
func (dbObject) GetObjectID() model.ObjectID {
	return ""
}

// SetObjectID is a dummy function to satisfy the interface
func (dbObject) SetObjectID(model.ObjectID) {
	// empty
}

func createDBObject(tableName string) dbObject {
	return dbObject{tableName: tableName}
}

func (b *BaseMongoConf) GetBlurredURL() string {
	// mongo uri match with regex ^(mongodb\S*(+srv)*:(?:\/{2})?)((\w+?):(\w+?)@|:?@?)(\S+?):(\d+)(\/(\S+?))?(\?replicaSet=(\S+?))?$
	// but we need only a segment, so regex explanation: https://regex101.com/r/C4GQvi/1
	regex := `^(mongodb\S*(srv)*:(?:\/{2})?)((...+?):(...+?)@)`
	re := regexp.MustCompile(regex)

	blurredUrl := re.ReplaceAllString(b.MongoURL, "***:***@")
	return blurredUrl
}

// @PumpConf Mongo
type MongoConf struct {
	// TYKCONFIGEXPAND
	BaseMongoConf

	// Specifies the mongo collection name.
	CollectionName string `json:"collection_name" mapstructure:"collection_name"`
	// Maximum insert batch size for mongo selective pump. If the batch we are writing surpass this value, it will be send in multiple batchs.
	// Defaults to 10Mb.
	MaxInsertBatchSizeBytes int `json:"max_insert_batch_size_bytes" mapstructure:"max_insert_batch_size_bytes"`
	// Maximum document size. If the document exceed this value, it will be skipped.
	// Defaults to 10Mb.
	MaxDocumentSizeBytes int `json:"max_document_size_bytes" mapstructure:"max_document_size_bytes"`
	// Amount of bytes of the capped collection in 64bits architectures.
	// Defaults to 5GB.
	CollectionCapMaxSizeBytes int `json:"collection_cap_max_size_bytes" mapstructure:"collection_cap_max_size_bytes"`
	// Enable collection capping. It's used to set a maximum size of the collection.
	CollectionCapEnable bool `json:"collection_cap_enable" mapstructure:"collection_cap_enable"`
}

func parsePrivateKey(der []byte) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("Found unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("Failed to parse private key")
}

func (m *MongoPump) New() Pump {
	newPump := MongoPump{}
	return &newPump
}

func (m *MongoPump) GetName() string {
	return "MongoDB Pump"
}

func (m *MongoPump) GetEnvPrefix() string {
	return m.dbConf.EnvPrefix
}

func (m *MongoPump) SetDecodingRequest(decoding bool) {
	if decoding {
		log.WithField("pump", m.GetName()).Warn("Decoding request is not supported for Mongo pump")
	}
}

func (m *MongoPump) SetDecodingResponse(decoding bool) {
	if decoding {
		log.WithField("pump", m.GetName()).Warn("Decoding response is not supported for Mongo pump")
	}
}

func (m *MongoPump) Init(config interface{}) error {
	m.dbConf = &MongoConf{}
	m.log = log.WithField("prefix", mongoPrefix)

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
		m.log.WithFields(logrus.Fields{
			"url":             m.dbConf.GetBlurredURL(),
			"collection_name": m.dbConf.CollectionName,
		}).Info("Init")
		if err != nil {
			panic(m.dbConf.BaseMongoConf)
		}
	}
	if err != nil {
		m.log.Fatal("Failed to decode configuration: ", err)
	}

	// we check for the environment configuration if this pumps is not the uptime pump
	if !m.IsUptime {
		processPumpEnvVars(m, m.log, m.dbConf, mongoDefaultEnv)

		// we keep this env check for backward compatibility
		overrideErr := envconfig.Process(mongoPumpPrefix, m.dbConf)
		if overrideErr != nil {
			m.log.Error("Failed to process environment variables for mongo pump: ", overrideErr)
		}
	} else if m.dbConf.MongoURL == "" {
		m.log.Debug("Trying to set uptime pump with PMP_MONGO env vars")
		// we keep this env check for backward compatibility
		overrideErr := envconfig.Process(mongoPumpPrefix, m.dbConf)
		if overrideErr != nil {
			m.log.Error("Failed to process environment variables for mongo pump: ", overrideErr)
		}

		m.dbConf.CollectionName = "tyk_uptime_analytics"
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

	m.capCollection()

	indexCreateErr := m.ensureIndexes(m.dbConf.CollectionName)
	if indexCreateErr != nil {
		m.log.Error(indexCreateErr)
	}

	m.log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.log.Debug("MongoDB Col: ", m.dbConf.CollectionName)

	m.log.Info(m.GetName() + " Initialized")

	return nil
}

func (m *MongoPump) capCollection() (ok bool) {
	colName := m.dbConf.CollectionName
	colCapMaxSizeBytes := m.dbConf.CollectionCapMaxSizeBytes
	colCapEnable := m.dbConf.CollectionCapEnable

	if !colCapEnable {
		return false
	}

	exists, err := m.collectionExists(colName)
	if err != nil {
		m.log.Errorf("Unable to determine if collection (%s) exists. Not capping collection: %s", colName, err.Error())

		return false
	}

	if exists {
		m.log.Warnf("Collection (%s) already exists. Capping could result in data loss. Ignoring", colName)

		return false
	}

	if strconv.IntSize < 64 {
		m.log.Warn("Pump running < 64bit architecture. Not capping collection as max size would be 2gb")

		return false
	}

	if colCapMaxSizeBytes == 0 {
		defaultBytes := 5
		colCapMaxSizeBytes = defaultBytes * GiB

		m.log.Infof("-- No max collection size set for %s, defaulting to %d", colName, colCapMaxSizeBytes)
	}

	d := dbObject{
		tableName: colName,
	}

	err = m.store.Migrate(context.Background(), []model.DBObject{d}, model.DBM{"capped": true, "maxBytes": colCapMaxSizeBytes})
	if err != nil {
		m.log.Errorf("Unable to create capped collection for (%s). %s", colName, err.Error())

		return false
	}

	m.log.Infof("Capped collection (%s) created. %d bytes", colName, colCapMaxSizeBytes)

	return true
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoPump) collectionExists(name string) (bool, error) {
	return m.store.HasTable(context.Background(), name)
}

func (m *MongoPump) ensureIndexes(collectionName string) error {
	if m.dbConf.OmitIndexCreation {
		m.log.Debug("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	if m.dbConf.MongoDBType == StandardMongo {
		exists, errExists := m.collectionExists(collectionName)
		if errExists == nil && exists {
			m.log.Info("Collection ", collectionName, " exists, omitting index creation..")
			return nil
		}
	}

	var err error

	orgIndex := model.Index{
		Keys:       []model.DBM{{"orgid": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	d := createDBObject(collectionName)

	err = m.store.CreateIndex(context.Background(), d, orgIndex)
	if err != nil {
		return err
	}

	apiIndex := model.Index{
		Keys:       []model.DBM{{"apiid": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = m.store.CreateIndex(context.Background(), d, apiIndex)
	if err != nil {
		return err
	}

	logBrowserIndex := model.Index{
		Name:       "logBrowserIndex",
		Keys:       []model.DBM{{"timestamp": -1}, {"orgid": 1}, {"apiid": 1}, {"apikey": 1}, {"responsecode": 1}},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}
	return m.store.CreateIndex(context.Background(), d, logBrowserIndex)
}

func (m *MongoPump) connect() {
	if m.dbConf.MongoDriverType == "" {
		// Default to mgo
		m.dbConf.MongoDriverType = persistent.Mgo
	}

	store, err := persistent.NewPersistentStorage(&persistent.ClientOpts{
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
		m.log.Fatal("Failed to connect: ", err)
	}

	m.store = store
}

func (m *MongoPump) WriteData(ctx context.Context, data []interface{}) error {
	collectionName := m.dbConf.CollectionName
	if collectionName == "" {
		m.log.Fatal("No collection name!")
	}

	m.log.Debug("Attempting to write ", len(data), " records...")

	accumulateSet := m.AccumulateSet(data, false)

	errCh := make(chan error, len(accumulateSet))
	for _, dataSet := range accumulateSet {
		go func(errCh chan error, dataSet ...model.DBObject) {
			m.log.WithFields(logrus.Fields{
				"collection":        collectionName,
				"number of records": len(dataSet),
			}).Debug("Attempt to purge records")

			err := m.store.Insert(context.Background(), dataSet...)
			if err != nil {
				m.log.WithFields(logrus.Fields{"collection": collectionName, "number of records": len(dataSet)}).Error("Problem inserting to mongo collection: ", err)
				errCh <- err
			}
			errCh <- nil
			m.log.WithFields(logrus.Fields{
				"collection":        collectionName,
				"number of records": len(dataSet),
			}).Info("Completed purging the records")
		}(errCh, dataSet...)
	}

	for range accumulateSet {
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}
	m.log.Info("Purged ", len(data), " records...")

	return nil
}

// AccumulateSet groups data items into chunks based on the max batch size limit while handling graph analytics records separately.
// It returns a 2D array of DBObjects.
func (m *MongoPump) AccumulateSet(data []interface{}, isForGraphRecords bool) [][]model.DBObject {
	accumulatorTotal := 0
	returnArray := make([][]model.DBObject, 0)
	thisResultSet := make([]model.DBObject, 0)

	for i, item := range data {
		// Process the current item and determine if it should be skipped
		thisItem, skip := m.shouldProcessItem(item, isForGraphRecords)
		if skip {
			continue
		}

		// If collection name is not set, we'll use the default one
		thisItem.CollectionName = m.dbConf.CollectionName

		// Calculate the size of the current item
		sizeBytes := m.getItemSizeBytes(thisItem)

		// Handle large documents that exceed the max document size limit
		m.handleLargeDocuments(thisItem, sizeBytes, isForGraphRecords)

		// Accumulate the item and update the accumulator total, result set, and return array
		accumulatorTotal, thisResultSet, returnArray = m.accumulate(thisResultSet, returnArray, thisItem, sizeBytes, accumulatorTotal, i == (len(data)-1))
	}

	// Append the remaining result set to the return array if it's not empty
	if len(thisResultSet) > 0 && len(returnArray) == 0 {
		returnArray = append(returnArray, thisResultSet)
	}
	return returnArray
}

// shouldProcessItem checks if the item should be processed based on its ResponseCode and if it's a graph record.
// It returns the processed item and a boolean indicating if the item should be skipped.
func (m *MongoPump) shouldProcessItem(item interface{}, isForGraphRecords bool) (records *analytics.AnalyticsRecord, shouldSKip bool) {
	thisItem, ok := item.(analytics.AnalyticsRecord)
	if !ok {
		m.log.Error("Couldn't convert item to analytics.AnalyticsRecord")
		return nil, true
	}
	if thisItem.ResponseCode == -1 {
		return &thisItem, true
	}

	isGraphRecord := thisItem.IsGraphRecord()
	if isForGraphRecords && !isGraphRecord {
		return &thisItem, true
	}
	return &thisItem, false
}

// getItemSizeBytes calculates the size of the item in bytes, including an additional 1 KB for metadata.
func (m *MongoPump) getItemSizeBytes(thisItem *analytics.AnalyticsRecord) int {
	// Add 1 KB for metadata as average
	return len(thisItem.RawRequest) + len(thisItem.RawResponse) + 1024
}

// handleLargeDocuments checks if the item size exceeds the max document size limit and modifies the item if necessary.
func (m *MongoPump) handleLargeDocuments(thisItem *analytics.AnalyticsRecord, sizeBytes int, isGraphRecord bool) {
	if sizeBytes > m.dbConf.MaxDocumentSizeBytes && !isGraphRecord {
		m.log.Warning("Document too large, not writing raw request and raw response!")

		thisItem.RawRequest = ""
		thisItem.RawResponse = base64.StdEncoding.EncodeToString([]byte("Document too large, not writing raw request and raw response!"))
	}
}

// accumulate processes the given item and updates the accumulator total, result set, and return array.
// It manages chunking the data into separate sets based on the max batch size limit, and appends the last item when necessary.
func (m *MongoPump) accumulate(thisResultSet []model.DBObject, returnArray [][]model.DBObject, thisItem *analytics.AnalyticsRecord, sizeBytes, accumulatorTotal int, isLastItem bool) (int, []model.DBObject, [][]model.DBObject) {
	if (accumulatorTotal + sizeBytes) <= m.dbConf.MaxInsertBatchSizeBytes {
		accumulatorTotal += sizeBytes
	} else {
		m.log.Debug("Created new chunk entry")
		if len(thisResultSet) > 0 {
			returnArray = append(returnArray, thisResultSet)
		}

		thisResultSet = make([]model.DBObject, 0)
		accumulatorTotal = sizeBytes
	}

	m.log.Debug("Accumulator is: ", accumulatorTotal)
	thisResultSet = append(thisResultSet, thisItem)

	m.log.Debug(accumulatorTotal, " of ", m.dbConf.MaxInsertBatchSizeBytes)
	if isLastItem {
		m.log.Debug("Appending last entry")
		returnArray = append(returnArray, thisResultSet)
	}

	return accumulatorTotal, thisResultSet, returnArray
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoPump) WriteUptimeData(data []interface{}) {
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

	m.log.Debug("Writing data to ", m.dbConf.CollectionName)

	if err := m.store.Insert(context.Background(), keys...); err != nil {
		m.log.Error("Problem inserting to mongo collection: ", err)
	}
}
