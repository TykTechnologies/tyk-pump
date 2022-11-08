package mongo

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps/common"
	"github.com/TykTechnologies/tyk-pump/pumps/internal/mgo"
	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
)

type Pump struct {
	IsUptime  bool
	dbSession mgo.SessionManager
	dbConf    *Config
	common.Pump
}

var mongoPrefix = "mongo-pump"
var mongoPumpPrefix = "PMP_MONGO"
var mongoDefaultEnv = common.PUMPS_ENV_PREFIX + "_MONGO" + common.PUMPS_ENV_META_PREFIX

func (p *Pump) GetName() string {
	return "MongoDB Pump"
}

func (p *Pump) GetEnvPrefix() string {
	return p.dbConf.EnvPrefix
}

func (p *Pump) Init(config interface{}) error {
	p.dbConf = &Config{}
	p.Log = logger.GetLogger().WithField("prefix", mongoPrefix)

	err := mapstructure.Decode(config, &p.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &p.dbConf.BaseConfig)
		p.Log.WithFields(logrus.Fields{
			"url":             p.dbConf.GetBlurredURL(),
			"collection_name": p.dbConf.CollectionName,
		}).Info("Init")
		if err != nil {
			return errors.New("failed to decode pump base configuration")
		}
	}
	if err != nil {
		p.Log.Error("Failed to decode configuration: ", err)
		return errors.New("failed to decode pump configuration")
	}

	//we check for the environment configuration if this pumps is not the uptime pump
	if !p.IsUptime {
		p.ProcessEnvVars(p.Log, p.dbConf, mongoDefaultEnv)

		//we keep this env check for backward compatibility
		overrideErr := envconfig.Process(mongoPumpPrefix, p.dbConf)
		if overrideErr != nil {
			p.Log.Error("Failed to process environment variables for mongo pump: ", overrideErr)
		}
	} else if p.IsUptime && p.dbConf.MongoURL == "" {
		p.Log.Debug("Trying to set uptime pump with PMP_MONGO env vars")
		//we keep this env check for backward compatibility
		overrideErr := envconfig.Process(mongoPumpPrefix, p.dbConf)
		if overrideErr != nil {
			p.Log.Error("Failed to process environment variables for mongo pump: ", overrideErr)
		}
	}

	if p.dbConf.MaxInsertBatchSizeBytes == 0 {
		p.Log.Info("-- No max batch size set, defaulting to 10MB")
		p.dbConf.MaxInsertBatchSizeBytes = 10 * MiB
	}

	if p.dbConf.MaxDocumentSizeBytes == 0 {
		p.Log.Info("-- No max document size set, defaulting to 10MB")
		p.dbConf.MaxDocumentSizeBytes = 10 * MiB
	}

	p.connect(mgo.NewDialer())

	p.capCollection()

	indexCreateErr := p.ensureIndexes()
	if indexCreateErr != nil {
		p.Log.Error(indexCreateErr)
	}

	p.Log.Debug("MongoDB DB CS: ", p.dbConf.GetBlurredURL())
	p.Log.Debug("MongoDB Col: ", p.dbConf.CollectionName)

	p.Log.Info(p.GetName() + " Initialized")

	return nil
}

func (p *Pump) connect(dialer mgo.Dialer) {

	var err error
	p.dbSession, err = NewSession(dialer, p.dbConf.BaseConfig, p.Timeout)
	for err != nil {
		p.Log.WithError(err).WithField("dialinfo", p.dbConf.BaseConfig.GetBlurredURL()).Error("Mongo connection failed. Retrying.")
		time.Sleep(5 * time.Second)
		p.dbSession, err = NewSession(dialer, p.dbConf.BaseConfig, p.Timeout)
	}

	if err == nil && p.dbConf.MongoDBType == 0 {
		p.dbConf.MongoDBType = GetMongoType(p.dbSession)
	}
}

func (p *Pump) WriteData(ctx context.Context, data []interface{}) error {

	collectionName := p.dbConf.CollectionName
	if collectionName == "" {
		p.Log.Error("No collection name!")
		return errors.New("no collection name")
	}

	p.Log.Debug("Attempting to write ", len(data), " records...")

	accumulateSet := p.AccumulateSet(data)

	errCh := make(chan error, len(accumulateSet))
	for _, dataSet := range accumulateSet {
		go func(dataSet []interface{}, errCh chan error) {
			sess := p.dbSession.Copy()
			defer sess.Close()

			analyticsCollection := sess.DB("").C(collectionName)

			p.Log.WithFields(logrus.Fields{
				"collection":        collectionName,
				"number of records": len(dataSet),
			}).Debug("Attempt to purge records")

			err := analyticsCollection.Insert(dataSet...)
			if err != nil {
				p.Log.WithFields(logrus.Fields{"collection": collectionName, "number of records": len(dataSet)}).Error("Problem inserting to mongo collection: ", err)

				if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
					p.Log.Warning("--> Detected connection failure!")
				}
				errCh <- err
			}
			errCh <- nil
			p.Log.WithFields(logrus.Fields{
				"collection":        collectionName,
				"number of records": len(dataSet),
			}).Info("Completed purging the records")
		}(dataSet, errCh)
	}

	for range accumulateSet {
		err := <-errCh
		if err != nil {
			return err
		}
	}
	p.Log.Info("Purged ", len(data), " records...")

	return nil
}

func (p *Pump) AccumulateSet(data []interface{}) [][]interface{} {

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

		p.Log.Debug("Size is: ", sizeBytes)

		if sizeBytes > p.dbConf.MaxDocumentSizeBytes {
			p.Log.Warning("Document too large, not writing raw request and raw response!")

			thisItem.RawRequest = ""
			thisItem.RawResponse = base64.StdEncoding.EncodeToString([]byte("Document too large, not writing raw request and raw response!"))
		}

		if (accumulatorTotal + sizeBytes) <= p.dbConf.MaxInsertBatchSizeBytes {
			accumulatorTotal += sizeBytes
		} else {
			p.Log.Debug("Created new chunk entry")
			if len(thisResultSet) > 0 {
				returnArray = append(returnArray, thisResultSet)
			}

			thisResultSet = make([]interface{}, 0)
			accumulatorTotal = sizeBytes
		}

		p.Log.Debug("Accumulator is: ", accumulatorTotal)
		thisResultSet = append(thisResultSet, thisItem)

		p.Log.Debug(accumulatorTotal, " of ", p.dbConf.MaxInsertBatchSizeBytes)
		// Append the last element if the loop is about to end
		if i == (len(data) - 1) {
			p.Log.Debug("Appending last entry")
			returnArray = append(returnArray, thisResultSet)
		}
	}

	return returnArray
}

func (p *Pump) capCollection() (ok bool) {

	var colName = p.dbConf.CollectionName
	var colCapMaxSizeBytes = p.dbConf.CollectionCapMaxSizeBytes
	var colCapEnable = p.dbConf.CollectionCapEnable

	if !colCapEnable {
		return false
	}

	exists, err := p.collectionExists(colName)
	if err != nil {
		p.Log.Errorf("Unable to determine if collection (%s) exists. Not capping collection: %s", colName, err.Error())

		return false
	}

	if exists {
		p.Log.Warnf("Collection (%s) already exists. Capping could result in data loss. Ignoring", colName)

		return false
	}

	if strconv.IntSize < 64 {
		p.Log.Warn("Pump running < 64bit architecture. Not capping collection as max size would be 2gb")

		return false
	}

	if colCapMaxSizeBytes == 0 {
		defaultBytes := 5
		colCapMaxSizeBytes = defaultBytes * GiB

		p.Log.Infof("-- No max collection size set for %s, defaulting to %d", colName, colCapMaxSizeBytes)
	}

	sess := p.dbSession.Copy()
	defer sess.Close()

	err = p.dbSession.DB("").C(colName).Create(&mgo.CollectionInfo{Capped: true, MaxBytes: colCapMaxSizeBytes})
	if err != nil {
		p.Log.Errorf("Unable to create capped collection for (%s). %s", colName, err.Error())
		return false
	}

	p.Log.Infof("Capped collection (%s) created. %d bytes", colName, colCapMaxSizeBytes)

	return true
}

// collectionExists checks to see if a collection name exists in the db.
func (p *Pump) collectionExists(name string) (bool, error) {
	sess := p.dbSession.Copy()
	defer sess.Close()

	colNames, err := sess.DB("").CollectionNames()
	if err != nil {
		p.Log.Error("Unable to get collection names: ", err)

		return false, err
	}

	for _, coll := range colNames {
		if coll == name {
			return true, nil
		}
	}

	return false, nil
}

func (p *Pump) ensureIndexes() error {
	if p.dbConf.OmitIndexCreation {
		p.Log.Debug("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	if p.dbConf.MongoDBType == StandardMongo {
		exists, errExists := p.collectionExists(p.dbConf.CollectionName)
		if errExists == nil && exists {
			p.Log.Info("Collection ", p.dbConf.CollectionName, " exists, omitting index creation..")
			return nil
		}
	}

	var err error

	sess := p.dbSession.Copy()
	defer sess.Close()

	c := sess.DB("").C(p.dbConf.CollectionName)

	orgIndex := mgo.Index{
		Key:        []string{"orgid"},
		Background: p.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(orgIndex)
	if err != nil {
		return err
	}

	apiIndex := mgo.Index{
		Key:        []string{"apiid"},
		Background: p.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(apiIndex)
	if err != nil {
		return err
	}

	logBrowserIndex := mgo.Index{
		Name:       "logBrowserIndex",
		Key:        []string{"-timestamp", "orgid", "apiid", "apikey", "responsecode"},
		Background: p.dbConf.MongoDBType == StandardMongo,
	}

	err = c.EnsureIndex(logBrowserIndex)
	if err != nil && !strings.Contains(err.Error(), "already exists with a different name") {
		return err
	}
	return nil
}
