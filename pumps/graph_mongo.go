package pumps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

const mongoGraphPrefix = "mongo-graph-pump"

type GraphMongoPump struct {
	CommonPumpConfig
	MongoPump
}

func (g *GraphMongoPump) New() Pump {
	return &GraphMongoPump{}
}

func (g *GraphMongoPump) GetEnvPrefix() string {
	return g.dbConf.EnvPrefix
}

func (g *GraphMongoPump) GetName() string {
	return "MongoDB Graph Pump"
}

func (g *GraphMongoPump) Init(config interface{}) error {
	g.dbConf = &MongoConf{}
	g.log = log.WithField("prefix", mongoGraphPrefix)
	g.MongoPump.CommonPumpConfig = g.CommonPumpConfig

	err := mapstructure.Decode(config, &g.dbConf)
	if err != nil {
		g.log.WithError(err).Warn("Failed to decode configuration: ")
		return err
	}
	g.log.WithFields(logrus.Fields{
		"url":             g.dbConf.GetBlurredURL(),
		"collection_name": g.dbConf.CollectionName,
	}).Info("Init")

	if err := mapstructure.Decode(config, &g.dbConf.BaseMongoConf); err != nil {
		return err
	}

	if g.dbConf.MaxInsertBatchSizeBytes == 0 {
		g.log.Info("-- No max batch size set, defaulting to 10MB")
		g.dbConf.MaxInsertBatchSizeBytes = 10 * MiB
	}

	if g.dbConf.MaxDocumentSizeBytes == 0 {
		g.log.Info("-- No max document size set, defaulting to 10MB")
		g.dbConf.MaxDocumentSizeBytes = 10 * MiB
	}

	g.connect()

	g.capCollection()

	indexCreateErr := g.ensureIndexes()
	if indexCreateErr != nil {
		g.log.Error(indexCreateErr)
	}

	g.log.Debug("MongoDB DB CS: ", g.dbConf.GetBlurredURL())
	g.log.Debug("MongoDB Col: ", g.dbConf.CollectionName)

	g.log.Info(g.GetName() + " Initialized")

	return nil
}

func (g *GraphMongoPump) WriteData(ctx context.Context, data []interface{}) error {
	collectionName := g.dbConf.CollectionName
	if collectionName == "" {
		g.log.Warn("no collection name")
		return fmt.Errorf("no collection name")
	}

	g.log.Debug("Attempting to write ", len(data), " records...")

	for g.dbSession == nil {
		g.log.Debug("Connecting to analytics store")
		g.connect()
	}
	accumulateSet := g.AccumulateSet(data, true)

	errCh := make(chan error, len(accumulateSet))
	for _, dataSet := range accumulateSet {
		go func(dataSet []interface{}, errCh chan error) {
			sess := g.dbSession.Copy()
			defer sess.Close()

			// make a graph record array with variable length in case there are errors with some conversion
			finalSet := make([]interface{}, 0)
			for _, d := range dataSet {
				r, ok := d.(analytics.AnalyticsRecord)
				if !ok {
					continue
				}

				var (
					gr  analytics.GraphRecord
					err error
				)
				if r.RawRequest == "" || r.RawResponse == "" || r.ApiSchema == "" {
					g.log.Warn("skipping record parsing")
					gr = analytics.GraphRecord{AnalyticsRecord: r}
				} else {
					gr, err = r.ToGraphRecord()
					if err != nil {
						errCh <- err
						g.log.WithError(err).Warn("error converting 1 record to graph record")
						continue
					}
				}

				finalSet = append(finalSet, gr)
			}

			analyticsCollection := sess.DB("").C(collectionName)

			g.log.WithFields(logrus.Fields{
				"collection":        collectionName,
				"number of records": len(finalSet),
			}).Debug("Attempt to purge records")

			err := analyticsCollection.Insert(finalSet...)
			if err != nil {
				g.log.WithFields(logrus.Fields{"collection": collectionName, "number of records": len(finalSet)}).Error("Problem inserting to mongo collection: ", err)

				if strings.Contains(strings.ToLower(err.Error()), "closed explicitly") {
					g.log.Warning("--> Detected connection failure!")
				}
				errCh <- err
				return
			}
			errCh <- nil
			g.log.WithFields(logrus.Fields{
				"collection":        collectionName,
				"number of records": len(finalSet),
			}).Info("Completed purging the records")
		}(dataSet, errCh)
	}

	for range accumulateSet {
		err := <-errCh
		if err != nil {
			return err
		}
	}
	g.log.Info("Purged ", len(data), " records...")

	return nil
}


func (g *GraphMongoPump) GetKVMap() map[string]interface{}{
	var inInterface map[string]interface{}
	inrec, _ := json.Marshal(g.dbConf)
	json.Unmarshal(inrec, &inInterface)

	return inInterface
}