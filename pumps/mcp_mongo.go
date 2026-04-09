package pumps

import (
	"context"
	"fmt"
	"strings"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

const mongoMCPPrefix = "mongo-mcp-pump"

var mongoMCPDefaultEnv = PUMPS_ENV_PREFIX + "_MONGOMCP" + PUMPS_ENV_META_PREFIX

// MCPMongoPump writes raw MCP analytics records to a dedicated MongoDB collection.
type MCPMongoPump struct {
	CommonPumpConfig
	MongoPump
}

func (g *MCPMongoPump) New() Pump {
	return &MCPMongoPump{}
}

func (g *MCPMongoPump) GetEnvPrefix() string {
	return g.dbConf.EnvPrefix
}

func (g *MCPMongoPump) GetName() string {
	return "MongoDB MCP Pump"
}

func (g *MCPMongoPump) SetDecodingRequest(decoding bool) {
	if decoding {
		log.WithField("pump", g.GetName()).Warn("Decoding request is not supported for MCP Mongo pump")
	}
}

func (g *MCPMongoPump) SetDecodingResponse(decoding bool) {
	if decoding {
		log.WithField("pump", g.GetName()).Warn("Decoding response is not supported for MCP Mongo pump")
	}
}

func (g *MCPMongoPump) Init(config interface{}) error {
	g.dbConf = &MongoConf{}
	g.log = log.WithField("prefix", mongoMCPPrefix)
	g.MongoPump.CommonPumpConfig = g.CommonPumpConfig

	err := mapstructure.Decode(config, &g.dbConf)
	if err != nil {
		g.log.Error("Failed to decode configuration: ", err)
		return err
	}
	g.log.WithFields(logrus.Fields{
		"url":             g.dbConf.GetBlurredURL(),
		"collection_name": g.dbConf.CollectionName,
	}).Info("Init")

	if err := mapstructure.Decode(config, &g.dbConf.BaseMongoConf); err != nil {
		return err
	}
	processPumpEnvVars(g, g.log, g.dbConf, mongoMCPDefaultEnv)

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

	indexCreateErr := g.ensureIndexes(g.dbConf.CollectionName)
	if indexCreateErr != nil {
		g.log.Error(indexCreateErr)
	}

	g.log.Debug("MongoDB DB CS: ", g.dbConf.GetBlurredURL())
	g.log.Debug("MongoDB Col: ", g.dbConf.CollectionName)
	g.log.Info(g.GetName() + " Initialized")

	return nil
}

// filterMCPData returns only the MCP analytics records from data.
func filterMCPData(data []interface{}) []interface{} {
	mcpData := make([]interface{}, 0, len(data))
	for _, d := range data {
		if rec, ok := d.(analytics.AnalyticsRecord); ok && rec.IsMCPRecord() {
			mcpData = append(mcpData, d)
		}
	}
	return mcpData
}

// convertToMCPObjects converts a batch of DBObjects to MCPRecord objects,
// assigning new ObjectIDs in the process.
func convertToMCPObjects(dataSet []model.DBObject) []model.DBObject {
	finalSet := make([]model.DBObject, 0, len(dataSet))
	for _, d := range dataSet {
		r, ok := d.(*analytics.AnalyticsRecord)
		if !ok {
			continue
		}
		r.SetObjectID(model.NewObjectID())
		mr := r.ToMCPRecord()
		finalSet = append(finalSet, &mr)
	}
	return finalSet
}

// insertMCPDataSet converts and inserts one accumulated batch into MongoDB.
func (g *MCPMongoPump) insertMCPDataSet(dataSet []model.DBObject, collectionName string, errCh chan error) {
	finalSet := convertToMCPObjects(dataSet)

	g.log.WithFields(logrus.Fields{
		"collection":        collectionName,
		"number of records": len(finalSet),
	}).Debug("Attempt to purge records")

	err := g.store.Insert(context.Background(), finalSet...)
	if err != nil {
		g.log.WithFields(logrus.Fields{
			"collection":        collectionName,
			"number of records": len(finalSet),
		}).Error("Problem inserting to mongo collection: ", err)
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
}

func (g *MCPMongoPump) WriteData(ctx context.Context, data []interface{}) error {
	collectionName := g.dbConf.CollectionName
	if collectionName == "" {
		g.log.Warn("no collection name")
		return fmt.Errorf("no collection name")
	}

	g.log.Debug("Attempting to write ", len(data), " records...")

	mcpData := filterMCPData(data)
	if len(mcpData) == 0 {
		g.log.Debug("no MCP records to write")
		return nil
	}

	accumulateSet := g.AccumulateSet(mcpData, false)
	errCh := make(chan error, len(accumulateSet))
	for _, dataSet := range accumulateSet {
		go g.insertMCPDataSet(dataSet, collectionName, errCh)
	}

	for range accumulateSet {
		if err := <-errCh; err != nil {
			return err
		}
	}

	g.log.Info("Purged ", len(mcpData), " records...")
	return nil
}
