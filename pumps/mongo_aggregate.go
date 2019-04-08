package pumps

import (
	b64 "encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/lonelycode/mgohacks"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

var mongoAggregatePumpPrefix = "PMP_MONGOAGG"

type MongoAggregatePump struct {
	dbSession *mgo.Session
	dbConf    *MongoAggregateConf
}

type MongoAggregateConf struct {
	MongoURL                   string `mapstructure:"mongo_url"`
	MongoUseSSL                bool   `mapstructure:"mongo_use_ssl"`
	MongoSSLInsecureSkipVerify bool   `mapstructure:"mongo_ssl_insecure_skip_verify"`
	UseMixedCollection         bool   `mapstructure:"use_mixed_collection"`
}

func (m *MongoAggregatePump) New() Pump {
	newPump := MongoAggregatePump{}
	return &newPump
}

func (m *MongoAggregatePump) doHash(in string) string {
	sEnc := b64.StdEncoding.EncodeToString([]byte(in))
	search := strings.TrimRight(sEnc, "=")
	return search
}

func (m *MongoAggregatePump) GetName() string {
	return "MongoDB Aggregate Pump"
}

func (m *MongoAggregatePump) GetCollectionName(orgid string) (string, error) {
	if orgid == "" {
		return "", errors.New("OrgID cannot be empty")
	}

	return "z_tyk_analyticz_aggregate_" + orgid, nil
}

func (m *MongoAggregatePump) Init(config interface{}) error {
	m.dbConf = &MongoAggregateConf{}
	err := mapstructure.Decode(config, &m.dbConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	overrideErr := envconfig.Process(mongoAggregatePumpPrefix, m.dbConf)
	if overrideErr != nil {
		log.Error("Failed to process environment variables for mongo aggregate pump: ", overrideErr)
	}

	m.connect()

	log.WithFields(logrus.Fields{
		"prefix": analytics.MongoAggregatePrefix,
	}).Debug("MongoDB DB CS: ", m.dbConf.MongoURL)

	return nil
}

func (m *MongoAggregatePump) connect() {
	var err error
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongoDialInfo(m.dbConf.MongoURL, m.dbConf.MongoUseSSL, m.dbConf.MongoSSLInsecureSkipVerify)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": mongoPrefix,
		}).Panic("Mongo URL is invalid: ", err)
	}

	m.dbSession, err = mgo.DialWithInfo(dialInfo)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Error("Mongo connection failed:", err)
		time.Sleep(5 * time.Second)
		m.connect()
	}
}

func (m *MongoAggregatePump) ensureIndexes(c *mgo.Collection) error {
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
		Key:        []string{"timestamp"},
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

	return c.EnsureIndex(orgIndex)
}

func (m *MongoAggregatePump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": analytics.MongoAggregatePrefix,
	}).Info("Writing ", len(data), " records")

	if m.dbSession == nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(data)
	} else {
		// calculate aggregates
		analyticsPerOrg := analytics.AggregateData(data)

		// put aggregated data into MongoDB
		for orgID, filteredData := range analyticsPerOrg {
			collectionName, collErr := m.GetCollectionName(orgID)
			if collErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": analytics.MongoAggregatePrefix,
				}).Info("No OrgID for AnalyticsRecord, skipping")
				continue
			}

			thisSession := m.dbSession.Copy()
			defer thisSession.Close()

			analyticsCollection := thisSession.DB("").C(collectionName)
			indexCreateErr := m.ensureIndexes(analyticsCollection)

			if indexCreateErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": analytics.MongoAggregatePrefix,
				}).Error(indexCreateErr)
			}

			query := bson.M{
				"orgid":     filteredData.OrgID,
				"timestamp": filteredData.TimeStamp,
			}

			updateDoc := filteredData.AsChange()

			change := mgo.Change{
				Update:    updateDoc,
				ReturnNew: true,
				Upsert:    true,
			}

			doc := analytics.AnalyticsRecordAggregate{}
			_, err := analyticsCollection.Find(query).Apply(change, &doc)

			if err != nil {
				log.WithFields(logrus.Fields{
					"prefix": analytics.MongoAggregatePrefix,
				}).Error("UPSERT Failure: ", err)
				return m.HandleWriteErr(err)
			}

			// We have the new doc back, lets fix the averages
			avgUpdateDoc := doc.AsTimeUpdate()
			avgChange := mgo.Change{
				Update:    avgUpdateDoc,
				ReturnNew: true,
			}
			withTimeUpdate := analytics.AnalyticsRecordAggregate{}
			_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &withTimeUpdate)

			if avgErr != nil {
				log.WithFields(logrus.Fields{
					"prefix": analytics.MongoAggregatePrefix,
				}).Error("AvgUpdate Failure: ", avgErr)
				return m.HandleWriteErr(avgErr)
			}

			if m.dbConf.UseMixedCollection {
				thisData := analytics.AnalyticsRecordAggregate{}
				err := analyticsCollection.Find(query).One(&thisData)
				if err != nil {
					log.Error("Couldn't find query doc!")
				} else {
					m.doMixedWrite(thisData, query)
				}

			}
		}
	}

	return nil
}

func (m *MongoAggregatePump) doMixedWrite(changeDoc analytics.AnalyticsRecordAggregate, query bson.M) {
	thisSession := m.dbSession.Copy()
	defer thisSession.Close()
	analyticsCollection := thisSession.DB("").C(analytics.AgggregateMixedCollectionName)
	m.ensureIndexes(analyticsCollection)

	avgChange := mgo.Change{
		Update:    changeDoc,
		ReturnNew: true,
		Upsert:    true,
	}

	final := analytics.AnalyticsRecordAggregate{}
	_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &final)

	if avgErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Error("Mixed coll upsert failure: ", avgErr)
		m.HandleWriteErr(avgErr)
	}
}

func (m *MongoAggregatePump) HandleWriteErr(err error) error {
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": analytics.MongoAggregatePrefix,
		}).Error("Problem inserting or updating to mongo collection: ", err)
		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			log.WithFields(logrus.Fields{
				"prefix": analytics.MongoAggregatePrefix,
			}).Warning("--> Detected connection failure, reconnecting")
			m.connect()
		}
	}
	return err
}

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (m *MongoAggregatePump) WriteUptimeData(data []interface{}) {
	log.WithFields(logrus.Fields{
		"prefix": analytics.MongoAggregatePrefix,
	}).Warning("Mongo Aggregate should not be writing uptime data!")
}
