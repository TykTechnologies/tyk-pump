package pumps

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/lonelycode/mgohacks"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/vmihailenco/msgpack.v2"
	"strings"
	"time"
)

type MongoSelectivePump struct {
	dbSession *mgo.Session
	dbConf    *MongoSelectiveConf
}

var mongoSelectivePrefix string = "mongo-pump-selective"

type MongoSelectiveConf struct {
	MongoURL string `mapstructure:"mongo_url"`
}

func (m *MongoSelectivePump) New() Pump {
	newPump := MongoSelectivePump{}
	return &newPump
}

func (m *MongoSelectivePump) GetName() string {
	return "MongoDB Selective Pump"
}

func (m *MongoSelectivePump) GetCollectionName(orgid string) (string, error) {
	if orgid == "" {
		return "", errors.New("OrgID cannot be empty")
	}
	return "z_tyk_analyticz_" + orgid, nil
}

func (m *MongoSelectivePump) Init(config interface{}) error {
	m.dbConf = &MongoSelectiveConf{}
	err := mapstructure.Decode(config, &m.dbConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoSelectivePrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	m.connect()

	log.WithFields(logrus.Fields{
		"prefix": mongoSelectivePrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.MongoURL)

	return nil
}

func (m *MongoSelectivePump) connect() {
	var err error
	m.dbSession, err = mgo.Dial(m.dbConf.MongoURL)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoSelectivePrefix,
		}).Error("Mongo connection failed:", err)
		time.Sleep(5)
		m.connect()
	}
}

func (m *MongoSelectivePump) ensureIndexes(c *mgo.Collection) error {
	var err error
	ttlIndex := mgo.Index{
		Key:         []string{"expireAt"},
		ExpireAfter: 0,
		Background:  true,
	}

	err = mgohacks.EnsureTTLIndex(c, ttlIndex)
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

	orgIndex := mgo.Index{
		Key:        []string{"orgid"},
		Background: true,
	}

	err = c.EnsureIndex(orgIndex)
	if err != nil {
		return err
	}

	idOrgIndex := mgo.Index{
		Key:        []string{"_id", "orgid"},
		Background: true,
	}

	err = c.EnsureIndex(idOrgIndex)
	if err != nil {
		return err
	}

	idOrgApiIndex := mgo.Index{
		Key:        []string{"_id", "orgid", "apiid"},
		Background: true,
	}

	err = c.EnsureIndex(idOrgApiIndex)
	if err != nil {
		return err
	}

	idOrgErrIndex := mgo.Index{
		Key:        []string{"_id", "orgid", "responsecode"},
		Background: true,
	}

	err = c.EnsureIndex(idOrgErrIndex)
	if err != nil {
		return err
	}

	return nil
}

func (m *MongoSelectivePump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": mongoSelectivePrefix,
	}).Debug("Writing ", len(data), " records")

	if m.dbSession == nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoSelectivePrefix,
		}).Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(data)
	} else {
		analyticsPerOrg := make(map[string][]interface{})

		for _, v := range data {
			orgID := v.(analytics.AnalyticsRecord).OrgID
			collectionName, collErr := m.GetCollectionName(orgID)
			skip := false
			if collErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoSelectivePrefix,
				}).Warning("No OrgID for AnalyticsRecord, skipping")
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
			analyticsCollection := m.dbSession.DB("").C(col_name)

			indexCreateErr := m.ensureIndexes(analyticsCollection)
			if indexCreateErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoSelectivePrefix,
				}).Error(indexCreateErr)
			}

			err := analyticsCollection.Insert(filtered_data...)
			if err != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoSelectivePrefix,
				}).Error("Problem inserting to mongo collection: ", err)
				if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
					log.WithFields(logrus.Fields{
						"prefix": mongoSelectivePrefix,
					}).Warning("--> Detected connection failure, reconnecting")
					m.connect()
				}
			}
		}

	}

	return nil
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoSelectivePump) WriteUptimeData(data []interface{}) {
	if m.dbSession == nil {
		log.Debug("Connecting to mongoDB store")
		m.connect()
		m.WriteUptimeData(data)
	} else {
		log.Info("MONGO SAelective Should not be writing uptime data!")
		collectionName := "tyk_uptime_analytics"
		analyticsCollection := m.dbSession.DB("").C(collectionName)

		log.WithFields(logrus.Fields{
			"prefix": mongoSelectivePrefix,
		}).Debug("Uptime Data: ", len(data))

		if len(data) > 0 {
			keys := make([]interface{}, len(data), len(data))

			for i, v := range data {
				decoded := analytics.UptimeReportData{}
				err := msgpack.Unmarshal(v.([]byte), &decoded)
				log.WithFields(logrus.Fields{
					"prefix": mongoSelectivePrefix,
				}).Debug("Decoded Record: ", decoded)
				if err != nil {
					log.WithFields(logrus.Fields{
						"prefix": mongoSelectivePrefix,
					}).Error("Couldn't unmarshal analytics data:", err)

				} else {
					keys[i] = interface{}(decoded)
				}
			}

			err := analyticsCollection.Insert(keys...)
			log.WithFields(logrus.Fields{
				"prefix": mongoSelectivePrefix,
			}).Debug("Wrote data to ", collectionName)

			if err != nil {
				log.WithFields(logrus.Fields{
					"prefix": mongoSelectivePrefix,
				}).Error("Problem inserting to mongo collection: ", err)
				if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
					log.WithFields(logrus.Fields{
						"prefix": mongoSelectivePrefix,
					}).Warning("--> Detected connection failure, reconnecting")
					m.connect()
				}
			}
		}
	}

}
