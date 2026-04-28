package pumps

import (
	"context"

	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

const mongoMCPAggregatePrefix = "mongo-mcp-aggregate-pump"

var mongoMCPAggregateDefaultEnv = PUMPS_ENV_PREFIX + "_MONGOMCPAGGREGATE" + PUMPS_ENV_META_PREFIX

// MCPMongoAggregatePump writes aggregated MCP analytics to MongoDB.
// It follows the same double-write pattern as MongoAggregatePump:
// writing to both an org-specific collection and (optionally) a mixed collection.
type MCPMongoAggregatePump struct {
	dbConf *MongoAggregateConf
	CommonPumpConfig
	MongoAggregatePump
}

func (m *MCPMongoAggregatePump) New() Pump {
	return &MCPMongoAggregatePump{}
}

func (m *MCPMongoAggregatePump) GetName() string {
	return "MongoDB MCP Aggregate Pump"
}

func (m *MCPMongoAggregatePump) GetEnvPrefix() string {
	return m.dbConf.EnvPrefix
}

func (m *MCPMongoAggregatePump) SetDecodingRequest(decoding bool) {
	if decoding {
		log.WithField("pump", m.GetName()).Warn("Decoding request is not supported for MCP Mongo Aggregate pump")
	}
}

func (m *MCPMongoAggregatePump) SetDecodingResponse(decoding bool) {
	if decoding {
		log.WithField("pump", m.GetName()).Warn("Decoding response is not supported for MCP Mongo Aggregate pump")
	}
}

func (m *MCPMongoAggregatePump) Init(config interface{}) error {
	m.dbConf = &MongoAggregateConf{}
	m.log = log.WithField("prefix", mongoMCPAggregatePrefix)
	m.MongoAggregatePump.log = m.log

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
	}
	if err != nil {
		m.log.Error("Failed to decode configuration: ", err)
		return err
	}

	// Share the decoded conf with the embedded MongoAggregatePump so that
	// connect(), ensureIndexes(), etc. all work correctly.
	m.MongoAggregatePump.dbConf = m.dbConf

	processPumpEnvVars(m, m.log, m.dbConf, mongoMCPAggregateDefaultEnv)

	if m.dbConf.ThresholdLenTagList == 0 {
		m.dbConf.ThresholdLenTagList = ThresholdLenTagList
	}
	m.MongoAggregatePump.SetAggregationTime()

	m.MongoAggregatePump.connect()

	m.log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.log.Info(m.GetName() + " Initialized")

	return nil
}

func (m *MCPMongoAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	m.log.Debug("Attempting to write ", len(data), " records")

	analyticsPerAPI := analytics.AggregateMCPData(data, m.dbConf.MongoURL, m.dbConf.AggregationTime)

	writingAttempts := []bool{false}
	if m.dbConf.UseMixedCollection {
		writingAttempts = append(writingAttempts, true)
	}

	mcpRecordCount := 0
	for apiID := range analyticsPerAPI {
		ag := analyticsPerAPI[apiID]
		mcpRecordCount += ag.Total.Hits
		for _, isMixedCollection := range writingAttempts {
			err := m.DoMCPAggregatedWriting(ctx, &ag, isMixedCollection)
			if err != nil {
				return err
			}
		}
		m.log.Debug("Processed aggregated MCP data for API ", apiID)
	}

	m.log.Info("Purged ", mcpRecordCount, " records...")
	return nil
}

// mcpDimensionNames is the set of MCP-specific dimension names that require
// their own MongoDB update operators alongside the standard ones.
var mcpDimensionNames = map[string]bool{
	"methods":    true,
	"primitives": true,
	"names":      true,
}

// addMCPDimensionUpdates appends $inc/$set/$max/$min operators for MCP-specific
// dimensions (methods, primitives, names) into an existing MongoDB update document.
// This mirrors what generateBSONFromProperty does for standard dimensions.
func addMCPDimensionUpdates(ag *analytics.MCPRecordAggregate, updateDoc model.DBM) {
	for _, d := range ag.Dimensions() {
		if !mcpDimensionNames[d.Name] {
			continue
		}
		prefix := d.Name + "." + d.Value + "."

		// Counters ($inc)
		updateDoc["$inc"].(model.DBM)[prefix+"hits"] = d.Counter.Hits
		updateDoc["$inc"].(model.DBM)[prefix+"errortotal"] = d.Counter.ErrorTotal
		updateDoc["$inc"].(model.DBM)[prefix+"success"] = d.Counter.Success
		updateDoc["$inc"].(model.DBM)[prefix+"totalrequesttime"] = d.Counter.TotalRequestTime
		updateDoc["$inc"].(model.DBM)[prefix+"totallatency"] = d.Counter.TotalLatency
		updateDoc["$inc"].(model.DBM)[prefix+"totalupstreamlatency"] = d.Counter.TotalUpstreamLatency

		// Error map ($inc)
		for k, v := range d.Counter.ErrorMap {
			updateDoc["$inc"].(model.DBM)[prefix+"errormap."+k] = v
		}

		// Identifiers and metadata ($set)
		updateDoc["$set"].(model.DBM)[prefix+"identifier"] = d.Counter.Identifier
		updateDoc["$set"].(model.DBM)[prefix+"humanidentifier"] = d.Counter.HumanIdentifier
		updateDoc["$set"].(model.DBM)[prefix+"lasttime"] = d.Counter.LastTime

		// Max latency ($max)
		updateDoc["$max"].(model.DBM)[prefix+"maxlatency"] = d.Counter.MaxLatency
		updateDoc["$max"].(model.DBM)[prefix+"maxupstreamlatency"] = d.Counter.MaxUpstreamLatency

		// Min latency ($min) — only when not all requests are errors
		if d.Counter.Hits != d.Counter.ErrorTotal {
			if updateDoc["$min"] == nil {
				updateDoc["$min"] = model.DBM{}
			}
			updateDoc["$min"].(model.DBM)[prefix+"minlatency"] = d.Counter.MinLatency
			updateDoc["$min"].(model.DBM)[prefix+"minupstreamlatency"] = d.Counter.MinUpstreamLatency
		}
	}
}

// upsertMCPAggregate performs the two-step MongoDB upsert for one aggregate document:
// first applying counters ($inc/$set/$max), then recalculating averages and lists.
func (m *MCPMongoAggregatePump) upsertMCPAggregate(ctx context.Context, ag *analytics.MCPRecordAggregate, query, updateDoc model.DBM) error {
	doc := &analytics.MCPRecordAggregate{
		AnalyticsRecordAggregate: analytics.AnalyticsRecordAggregate{OrgID: ag.OrgID, Mixed: ag.Mixed},
	}

	if err := m.MongoAggregatePump.store.Upsert(ctx, doc, query, updateDoc); err != nil {
		m.log.WithField("query", query).Error("UPSERT Failure: ", err)
		return err
	}

	avgUpdateDoc := doc.AsTimeUpdate()
	withTimeUpdate := analytics.MCPRecordAggregate{
		AnalyticsRecordAggregate: analytics.AnalyticsRecordAggregate{OrgID: ag.OrgID, Mixed: ag.Mixed},
	}
	if err := m.MongoAggregatePump.store.Upsert(ctx, &withTimeUpdate, query, avgUpdateDoc); err != nil {
		m.log.WithField("query", query).Error("AvgUpdate Failure: ", err)
		return err
	}

	return nil
}

func (m *MCPMongoAggregatePump) DoMCPAggregatedWriting(ctx context.Context, ag *analytics.MCPRecordAggregate, mixed bool) error {
	ag.Mixed = mixed
	collectionName := ag.TableName()

	if err := m.MongoAggregatePump.ensureIndexes(collectionName); err != nil {
		m.log.Error(err)
	}

	query := model.DBM{
		"orgid":       ag.OrgID,
		"timestamp":   ag.TimeStamp,
		"owner_apiid": ag.OwnerAPIID,
	}

	updateDoc := ag.AnalyticsRecordAggregate.AsChange()
	addMCPDimensionUpdates(ag, updateDoc)

	m.log.WithFields(logrus.Fields{"collection": collectionName}).Debug("Attempt to upsert MCP aggregated doc")

	return m.upsertMCPAggregate(ctx, ag, query, updateDoc)
}
