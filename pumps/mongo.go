package pumps

import (
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/vmihailenco/msgpack.v2"
	"strings"
	"time"
)

const tenMB int = 10485760

type MongoPump struct {
	dbSession *mgo.Session
	dbConf    *MongoConf
}

var mongoPrefix string = "mongo-pump"
var mongoPumpPrefix string = "PMP_MONGO"

type MongoConf struct {
	CollectionName          string `mapstructure:"collection_name"`
	MongoURL                string `mapstructure:"mongo_url"`
	MaxInsertBatchSizeBytes int    `mapstructure:"max_insert_batch_size_bytes"`
	MaxDocumentSizeBytes    int    `mapstructure:"max_document_size_bytes"`
	DevMode 		bool   `mapstructure:"dev_mode"`
	DevOrg 			string `mapstructure:"dev_org"`
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

	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.MongoURL)
	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("MongoDB Col: ", m.dbConf.CollectionName)

	return nil
}

func (m *MongoPump) connect() {
	var err error
	m.dbSession, err = mgo.Dial(m.dbConf.MongoURL)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Error("Mongo connection failed:", err)
		time.Sleep(5)
		m.connect()
	}
}

func (m *MongoPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": mongoPrefix,
	}).Debug("Writing ", len(data), " records")

	if m.dbSession == nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(data)
	} else {
		collectionName := m.dbConf.CollectionName
		if m.dbConf.CollectionName == "" {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Fatal("No collection name!")
		}

		for _, dataSet := range m.AccumulateSet(data) {

			go func() {
				thisSession := m.dbSession.Copy()
				defer thisSession.Close()
				analyticsCollection := thisSession.DB("").C(collectionName)

				log.WithFields(logrus.Fields{
					"prefix": mongoPrefix,
				}).Info("Purging ", len(dataSet), " records")

				err := analyticsCollection.Insert(dataSet...)
				if err != nil {
					log.Error("Problem inserting to mongo collection: ", err)
					if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
						log.Warning("--> Detected connection failure!")
						//m.connect()
					}
				}
			}()

		}

	}

	return nil
}

func (m *MongoPump) AccumulateSet(data []interface{}) [][]interface{} {
	var accumulatorTotal int
	accumulatorTotal = 0
	returnArray := make([][]interface{}, 0)

	thisResultSet := make([]interface{}, 0)
	for i, item := range data {
		thisItem := item.(analytics.AnalyticsRecord)
		sizeBytes := len([]byte(thisItem.RawRequest)) + len([]byte(thisItem.RawRequest))

		skip := false
		if sizeBytes > m.dbConf.MaxDocumentSizeBytes {
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Warning("Document too large, skipping!")
			skip = true
		}

		log.Debug("Size is: ", sizeBytes)

		if !skip {
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

	}

	return returnArray
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoPump) WriteUptimeData(data []interface{}) {
	if m.dbSession == nil {
		log.Debug("Connecting to mongoDB store")
		m.connect()
		m.WriteUptimeData(data)
	} else {
		collectionName := "tyk_uptime_analytics"
		thisSession := m.dbSession.Copy()
		defer thisSession.Close()
		analyticsCollection := thisSession.DB("").C(collectionName)

		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Debug("Uptime Data: ", len(data))

		if len(data) > 0 {
			keys := make([]interface{}, len(data), len(data))

			for i, v := range data {
				decoded := analytics.UptimeReportData{}
				err := msgpack.Unmarshal(v.([]byte), &decoded)
				log.WithFields(logrus.Fields{
					"prefix": mongoPrefix,
				}).Debug("Decoded Record: ", decoded)
				if err != nil {
					log.WithFields(logrus.Fields{
						"prefix": mongoPrefix,
					}).Error("Couldn't unmarshal analytics data:", err)

				} else {
					keys[i] = interface{}(decoded)
				}
			}

			err := analyticsCollection.Insert(keys...)
			log.WithFields(logrus.Fields{
				"prefix": mongoPrefix,
			}).Debug("Wrote data to ", collectionName)

			if err != nil {
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
	}

}
