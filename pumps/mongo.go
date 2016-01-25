package pumps

import (
	"github.com/Sirupsen/logrus"
	"github.com/lonelycode/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/vmihailenco/msgpack.v2"
	"strings"
	"time"
)

type MongoPump struct {
	dbSession *mgo.Session
	dbConf    *MongoConf
}

var mongoPrefix string = "mongo-pump"

type MongoConf struct {
	CollectionName string `mapstructure:"collection_name"`
	MongoURL       string `mapstructure:"mongo_url"`
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
		analyticsCollection := m.dbSession.DB("").C(collectionName)

		err := analyticsCollection.Insert(data...)
		if err != nil {
			log.Error("Problem inserting to mongo collection: ", err)
			if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
				log.Warning("--> Detected connection failure, reconnecting")
				m.connect()
			}
		}
	}

	return nil
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoPump) WriteUptimeData(data []interface{}) {
	if m.dbSession == nil {
		log.Debug("Connecting to mongoDB store")
		m.connect()
		m.WriteUptimeData(data)
	} else {
		collectionName := "tyk_uptime_analytics"
		analyticsCollection := m.dbSession.DB("").C(collectionName)

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
				if strings.Contains(err.Error(), "Closed explicitly") {
					log.WithFields(logrus.Fields{
						"prefix": mongoPrefix,
					}).Warning("--> Detected connection failure, reconnecting")
					m.connect()
				}
			}
		}
	}

}
