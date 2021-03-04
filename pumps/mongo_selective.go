package pumps

import (
	"context"
	"errors"
	"strings"
	"time"

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
	CommonPumpConfig
}

var mongoSelectivePrefix = "mongo-pump-selective"
var mongoSelectivePumpPrefix = "PMP_MONGOSEL"

type MongoSelectiveConf struct {
	BaseMongoConf
	MaxInsertBatchSizeBytes int `mapstructure:"max_insert_batch_size_bytes"`
	MaxDocumentSizeBytes    int `mapstructure:"max_document_size_bytes"`
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
	m.log = log.WithField("prefix", mongoSelectivePrefix)

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
	}

	if err != nil {
		m.log.Fatal("Failed to decode configuration: ", err)
	}

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
	var dialInfo *mgo.DialInfo

	dialInfo, err = mongoDialInfo(m.dbConf.BaseMongoConf)
	if err != nil {
		m.log.Panic("Mongo URL is invalid: ", err)
	}

	if m.timeout > 0 {
		dialInfo.Timeout = time.Second * time.Duration(m.timeout)
	}

	m.dbSession, err = mgo.DialWithInfo(dialInfo)

	for err != nil {
		m.log.WithError(err).WithField("dialinfo", dialInfo).Error("Mongo connection failed. Retrying.")
		time.Sleep(5 * time.Second)
		m.dbSession, err = mgo.DialWithInfo(dialInfo)
	}

	if err == nil && m.dbConf.MongoDBType == 0 {
		m.dbConf.MongoDBType = mongoType(m.dbSession)
	}
}

func (m *MongoSelectivePump) ensureIndexes(c *mgo.Collection) error {
	var err error
	ttlIndex := mgo.Index{
		Key:         []string{"expireAt"},
		ExpireAfter: 0,
		Background:  m.dbConf.MongoDBType == StandardMongo,
	}

	err = mgohacks.EnsureTTLIndex(c, ttlIndex)
	if err != nil {
		return err
	}

	apiIndex := mgo.Index{
		Key:        []string{"apiid"},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(apiIndex)
	if err != nil {
		return err
	}

	logBrowserIndex := mgo.Index{
		Name:       "logBrowserIndex",
		Key:        []string{"-timestamp", "apiid", "apikey", "responsecode"},
		Background: m.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(logBrowserIndex)
	if err != nil && !strings.Contains(err.Error(), "already exists with a different name") {
		return err
	}

	return nil
}

func (m *MongoSelectivePump) WriteData(ctx context.Context, data []interface{}) error {
	m.log.Debug("Attempting to write ", len(data), " records...")

	if m.dbSession == nil {
		m.log.Debug("Connecting to analytics store")
		m.connect()
		m.WriteData(ctx, data)
	} else {
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
				thisSession := m.dbSession.Copy()
				defer thisSession.Close()
				analyticsCollection := thisSession.DB("").C(col_name)

				indexCreateErr := m.ensureIndexes(analyticsCollection)
				if indexCreateErr != nil {
					m.log.WithField("collection", col_name).Error(indexCreateErr)
				}

				err := analyticsCollection.Insert(dataSet...)
				if err != nil {
					m.log.WithField("collection", col_name).Error("Problem inserting to mongo collection: ", err)
					if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
						m.log.Warning("--> Detected connection failure, reconnecting")
						m.connect()
					}
				}
			}

		}

	}
	m.log.Info("Purged ", len(data), " records...")

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

				thisResultSet = make([]interface{}, 0)
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
	if m.dbSession == nil {
		m.log.Debug("Connecting to mongoDB store")
		m.connect()
		m.WriteUptimeData(data)
	} else {
		m.log.Info("MONGO Selective Should not be writing uptime data!")
		collectionName := "tyk_uptime_analytics"
		thisSession := m.dbSession.Copy()
		defer thisSession.Close()
		analyticsCollection := thisSession.DB("").C(collectionName)

		m.log.Debug("Uptime Data: ", len(data))

		if len(data) > 0 {
			keys := make([]interface{}, len(data))

			for i, v := range data {
				decoded := analytics.UptimeReportData{}
				err := msgpack.Unmarshal(v.([]byte), &decoded)
				m.log.Debug("Decoded Record: ", decoded)
				if err != nil {
					m.log.Error("Couldn't unmarshal analytics data:", err)

				} else {
					keys[i] = interface{}(decoded)
				}
			}

			err := analyticsCollection.Insert(keys...)
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

}
