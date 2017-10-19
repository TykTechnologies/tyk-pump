package pumps

import (
	"crypto/tls"
	"net"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/vmihailenco/msgpack.v2"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"gopkg.in/mgo.v2/bson"
)

const tenMB int = 10 << 20

type MongoPump struct {
	dbSession *mgo.Session
	dbConf    *MongoConf
}

var mongoPrefix = "mongo-pump"
var mongoPumpPrefix = "PMP_MONGO"

type MongoConf struct {
	CollectionName             string `mapstructure:"collection_name"`
	MongoURL                   string `mapstructure:"mongo_url"`
	MongoUseSSL                bool   `mapstructure:"mongo_use_ssl"`
	MongoSSLInsecureSkipVerify bool   `mapstructure:"mongo_ssl_insecure_skip_verify"`
	MaxInsertBatchSizeBytes    int    `mapstructure:"max_insert_batch_size_bytes"`
	MaxDocumentSizeBytes       int    `mapstructure:"max_document_size_bytes"`
	CollectionCapMaxSizeBytes  int    `mapstructure:"collection_cap_max_size_bytes"`
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
		m.dbConf.MaxInsertBatchSizeBytes = tenMB
	}

	if m.dbConf.MaxDocumentSizeBytes == 0 {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Info("-- No max document size set, defaulting to 10MB")
		m.dbConf.MaxDocumentSizeBytes = tenMB
	}

	m.connect()

	m.capCollection()

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.MongoURL)
	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("MongoDB Col: ", m.dbConf.CollectionName)

	return nil
}

func (m *MongoPump) capCollection() {

	var colName = m.dbConf.CollectionName
	var colCapMaxSizeBytes = m.dbConf.CollectionCapMaxSizeBytes

	exists, err := m.collectionExists(colName)
	if err != nil {
		// TODO Handle Error

		return
	}

	if !exists {
		if err := m.createCappedCollection(colName, colCapMaxSizeBytes); err != nil {
			// TODO handle error

			return
		}

		return
	}

	// If colCapMaxSizeBytes == 0, uncap the collection and return
	if colCapMaxSizeBytes == 0 {
		m.uncapCollection(colName)

		return
	}

	m.resizeCappedCollection(colName, colCapMaxSizeBytes)
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

func (m *MongoPump) createCappedCollection(name string, maxBytes int) error {
	sess := m.dbSession.Copy()
	defer sess.Close()

	// No need to create collection if MaxBytes are 0
	if maxBytes == 0 {
		return nil
	}

	return sess.DB("").C(name).Create(&mgo.CollectionInfo{Capped: true, MaxBytes: maxBytes})
}

func (m *MongoPump) uncapCollection(name string) {
	// TODO - Check if capped

	// If not capped, return

	// If capped, do uncap logic:

	// db.collection.copyTo("collection_temp")
	// db.collection.drop()
	// db.collection_temp.renameCollection("collection")
}

func (m *MongoPump) resizeCappedCollection(name string, maxBytes int) {
	var collStats bson.M
	sess := m.dbSession.Copy()
	defer sess.Close()

	if err := sess.DB("").Run(bson.D{{Name: "collStats", Value: name}}, &collStats); err != nil {
		// TODO handle error

		return
	}

	if maxBytes < collStats["size"] {
		// TODO Write warning log advising that resizing collection ignored due to data loss

		return
	}

	// Collection safe to resize
	var doc bson.M
	err := sess.DB("").Run(bson.D{
		{Name: "convertToCapped", Value: name},
		{Name: "size", Value: maxBytes},
	}, &doc)
	if err != nil {
		// TODO - Handle Error

		return
	}
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

	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	// TODO - Should this not bail after a while?
	for err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Error("Mongo connection failed:", err)

		time.Sleep(5)
		m.dbSession, err = mgo.DialWithInfo(dialInfo)
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
		sizeBytes := len([]byte(thisItem.RawRequest)) + len([]byte(thisItem.RawRequest))

		log.Debug("Size is: ", sizeBytes)

		if sizeBytes > m.dbConf.MaxDocumentSizeBytes {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Warning("Document too large, skipping!")
			continue
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
