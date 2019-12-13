package pumps

import (
	"crypto/tls"
	"encoding/base64"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/vmihailenco/msgpack.v2"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	_   = iota // ignore zero iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
	TiB
)

type MongoPump struct {
	dbSession *mgo.Session
	dbConf    *MongoConf
}

var mongoPrefix = "mongo-pump"
var mongoPumpPrefix = "PMP_MONGO"

type MongoConf struct {
	CollectionName             string `json:"collection_name" mapstructure:"collection_name"`
	MongoURL                   string `json:"mongo_url" mapstructure:"mongo_url"`
	MongoUseSSL                bool   `json:"mongo_use_ssl" mapstructure:"mongo_use_ssl"`
	MongoSSLInsecureSkipVerify bool   `json:"mongo_ssl_insecure_skip_verify" mapstructure:"mongo_ssl_insecure_skip_verify"`
	MaxInsertBatchSizeBytes    int    `json:"max_insert_batch_size_bytes" mapstructure:"max_insert_batch_size_bytes"`
	MaxDocumentSizeBytes       int    `json:"max_document_size_bytes" mapstructure:"max_document_size_bytes"`
	CollectionCapMaxSizeBytes  int    `json:"collection_cap_max_size_bytes" mapstructure:"collection_cap_max_size_bytes"`
	CollectionCapEnable        bool   `json:"collection_cap_enable" mapstructure:"collection_cap_enable"`
}

func mongoDialInfo(mongoURL string, useSSL bool, SSLInsecureSkipVerify bool) (dialInfo *mgo.DialInfo, err error) {

	if dialInfo, err = mgo.ParseURL(mongoURL); err != nil {
		return dialInfo, err
	}

	if useSSL {
		dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			tlsConfig := &tls.Config{}
			if SSLInsecureSkipVerify {
				tlsConfig.InsecureSkipVerify = true
			}
			return tls.Dial("tcp", addr.String(), tlsConfig)
		}
	}

	return dialInfo, err
}

func (m *MongoPump) New() Pump {
	newPump := MongoPump{}
	return &newPump
}

func (m *MongoPump) GetName() string {
	return "MongoDB Pump"
}

func (m *MongoPump) Init(config interface{}) error {
	m.dbConf = &MongoConf{}
	err := mapstructure.Decode(config, &m.dbConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	overrideErr := envconfig.Process(mongoPumpPrefix, m.dbConf)
	if overrideErr != nil {
		log.Error("Failed to process environment variables for mongo pump: ", overrideErr)
	}

	if m.dbConf.MaxInsertBatchSizeBytes == 0 {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Info("-- No max batch size set, defaulting to 10MB")
		m.dbConf.MaxInsertBatchSizeBytes = 10 * MiB
	}

	if m.dbConf.MaxDocumentSizeBytes == 0 {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Info("-- No max document size set, defaulting to 10MB")
		m.dbConf.MaxDocumentSizeBytes = 10 * MiB
	}

	m.connect()

	m.capCollection()

	indexCreateErr := m.ensureIndexes()
	if indexCreateErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Error(indexCreateErr)
	}

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.MongoURL)
	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("MongoDB Col: ", m.dbConf.CollectionName)

	return nil
}

func (m *MongoPump) capCollection() (ok bool) {

	var colName = m.dbConf.CollectionName
	var colCapMaxSizeBytes = m.dbConf.CollectionCapMaxSizeBytes
	var colCapEnable = m.dbConf.CollectionCapEnable

	if !colCapEnable {
		return false
	}

	exists, err := m.collectionExists(colName)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Errorf("Unable to determine if collection (%s) exists. Not capping collection: %s", colName, err.Error())

		return false
	}

	if exists {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Warnf("Collection (%s) already exists. Capping could result in data loss. Ignoring", colName)

		return false
	}

	if strconv.IntSize < 64 {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Warn("Pump running < 64bit architecture. Not capping collection as max size would be 2gb")

		return false
	}

	if colCapMaxSizeBytes == 0 {
		defaultBytes := 5
		colCapMaxSizeBytes = defaultBytes * GiB

		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Infof("-- No max collection size set for %s, defaulting to %d", colName, colCapMaxSizeBytes)
	}

	sess := m.dbSession.Copy()
	defer sess.Close()

	err = m.dbSession.DB("").C(colName).Create(&mgo.CollectionInfo{Capped: true, MaxBytes: colCapMaxSizeBytes})
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Errorf("Unable to create capped collection for (%s). %s", colName, err.Error())

		return false
	}

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Infof("Capped collection (%s) created. %d bytes", colName, colCapMaxSizeBytes)

	return true
}

// collectionExists checks to see if a collection name exists in the db.
func (m *MongoPump) collectionExists(name string) (bool, error) {
	sess := m.dbSession.Copy()
	defer sess.Close()

	colNames, err := sess.DB("").CollectionNames()
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Error("Unable to get column names: ", err)

		return false, err
	}

	for _, coll := range colNames {
		if coll == name {
			return true, nil
		}
	}

	return false, nil
}

func (m *MongoPump) ensureIndexes() error {
	var err error

	sess := m.dbSession.Copy()
	defer sess.Close()

	c := sess.DB("").C(m.dbConf.CollectionName)

	orgIndex := mgo.Index{
		Key:        []string{"orgid"},
		Background: true,
	}

	err = c.EnsureIndex(orgIndex)
	if err != nil {
		return err
	}

	apiIndex := mgo.Index{
		Key:        []string{"apiid"},
		Background: true,
	}

	err = c.EnsureIndex(apiIndex)
	if err != nil {
		return err
	}

	logBrowserIndex := mgo.Index{
		Key:        []string{"-timestamp", "orgid", "apiid", "apikey", "responsecode"},
		Background: true,
	}

	err = c.EnsureIndex(logBrowserIndex)
	if err != nil {
		return err
	}

	return nil
}

func (m *MongoPump) connect() {
	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongoDialInfo(m.dbConf.MongoURL, m.dbConf.MongoUseSSL, m.dbConf.MongoSSLInsecureSkipVerify)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Panic("Mongo URL is invalid: ", err)
	}

	dialInfo.Timeout = time.Second * 5
	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Fatal("Mongo connection failed:", err)
	}
}

func (m *MongoPump) WriteData(data []interface{}) error {

	collectionName := m.dbConf.CollectionName
	if collectionName == "" {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Fatal("No collection name!")
	}

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("Writing ", len(data), " records")

	for m.dbSession == nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Debug("Connecting to analytics store")
		m.connect()
	}

	for _, dataSet := range m.AccumulateSet(data) {
		go func(dataSet []interface{}) {
			sess := m.dbSession.Copy()
			defer sess.Close()

			analyticsCollection := sess.DB("").C(collectionName)

			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Info("Purging ", len(dataSet), " records")

			err := analyticsCollection.Insert(dataSet...)
			if err != nil {
				log.Error("Problem inserting to mongo collection: ", err)
				if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
					log.Warning("--> Detected connection failure!")
				}
			}
		}(dataSet)
	}

	return nil
}

func (m *MongoPump) AccumulateSet(data []interface{}) [][]interface{} {

	accumulatorTotal := 0
	returnArray := make([][]interface{}, 0)
	thisResultSet := make([]interface{}, 0)

	for i, item := range data {
		thisItem := item.(analytics.AnalyticsRecord)
		if thisItem.ResponseCode == -1 {
			continue
		}

		// Add 1 KB for metadata as average
		sizeBytes := len(thisItem.RawRequest) + len(thisItem.RawResponse) + 1024

		log.Debug("Size is: ", sizeBytes)

		if sizeBytes > m.dbConf.MaxDocumentSizeBytes {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Warning("Document too large, not writing raw request and raw response!")

			thisItem.RawRequest = ""
			thisItem.RawResponse = base64.StdEncoding.EncodeToString([]byte("Document too large, not writing raw request and raw response!"))
		}

		if (accumulatorTotal + sizeBytes) < m.dbConf.MaxInsertBatchSizeBytes {
			accumulatorTotal += sizeBytes
		} else {
			log.Debug("Created new chunk entry")
			if len(thisResultSet) > 0 {
				returnArray = append(returnArray, thisResultSet)
			}

			thisResultSet = make([]interface{}, 0)
			accumulatorTotal = sizeBytes
		}

		log.Debug("Accumulator is: ", accumulatorTotal)
		thisResultSet = append(thisResultSet, thisItem)

		log.Debug(accumulatorTotal, " of ", m.dbConf.MaxInsertBatchSizeBytes)
		// Append the last element if the loop is about to end
		if i == (len(data) - 1) {
			log.Debug("Appending last entry")
			returnArray = append(returnArray, thisResultSet)
		}
	}

	return returnArray
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoPump) WriteUptimeData(data []interface{}) {

	for m.dbSession == nil {
		log.Debug("Connecting to mongoDB store")
		m.connect()
	}

	collectionName := "tyk_uptime_analytics"
	sess := m.dbSession.Copy()
	defer sess.Close()

	analyticsCollection := sess.DB("").C(collectionName)

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("Uptime Data: ", len(data))

	if len(data) == 0 {
		return
	}

	keys := make([]interface{}, len(data))

	for i, v := range data {
		decoded := analytics.UptimeReportData{}

		if err := msgpack.Unmarshal(v.([]byte), &decoded); err != nil {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Error("Couldn't unmarshal analytics data:", err)

			continue
		}

		keys[i] = interface{}(decoded)

		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Debug("Decoded Record: ", decoded)
	}

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("Writing data to ", collectionName)

	if err := analyticsCollection.Insert(keys...); err != nil {

		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Error("Problem inserting to mongo collection: ", err)

		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Warning("--> Detected connection failure, reconnecting")

			m.connect()
		}
	}
}
