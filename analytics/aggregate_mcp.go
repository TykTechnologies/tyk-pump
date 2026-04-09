package analytics

import (
	"fmt"

	"github.com/TykTechnologies/storage/persistent/model"
)

const (
	MCPAggregateMixedCollectionName = "tyk_mcp_analytics_aggregate"
	AggregateMCPSQLTable            = "tyk_mcp_aggregated"
)

// MCPRecordAggregate holds aggregated MCP analytics grouped by API.
// It embeds AnalyticsRecordAggregate for all standard dimensions and adds
// MCP-specific dimension maps for method, primitive type, and primitive name.
type MCPRecordAggregate struct {
	AnalyticsRecordAggregate `bson:",inline"`

	Methods    map[string]*Counter // keyed by JSONRPCMethod
	Primitives map[string]*Counter // keyed by PrimitiveType
	Names      map[string]*Counter // keyed by PrimitiveName
}

// MCPSQLAnalyticsRecordAggregate is the SQL representation of an MCP aggregate record.
type MCPSQLAnalyticsRecordAggregate struct {
	ID string `gorm:"primaryKey"`

	OrgID          string `json:"org_id"`
	Dimension      string `json:"dimension"`
	DimensionValue string `json:"dimension_value"`
	APIID          string `json:"api_id"`

	Counter `json:"counter" gorm:"embedded"`
	Code    `json:"code" gorm:"embedded"`

	TimeStamp int64 `json:"timestamp"`
}

func (m *MCPSQLAnalyticsRecordAggregate) TableName() string {
	return AggregateMCPSQLTable
}

// TableName returns the MongoDB collection name for this aggregate.
func (a *MCPRecordAggregate) TableName() string {
	if a.Mixed {
		return MCPAggregateMixedCollectionName
	}
	return "z_tyk_mcp_analyticz_aggregate_" + a.OrgID
}

// Dimensions returns all dimensions for MCP records, including MCP-specific
// dimension maps. This is required for AsChange() and AsTimeUpdate() to work.
func (a *MCPRecordAggregate) Dimensions() []Dimension {
	dims := a.AnalyticsRecordAggregate.Dimensions()

	for key, inc := range a.Methods {
		dims = append(dims, Dimension{Name: "methods", Value: key, Counter: fnLatencySetter(inc)})
	}

	for key, inc := range a.Primitives {
		dims = append(dims, Dimension{Name: "primitives", Value: key, Counter: fnLatencySetter(inc)})
	}

	for key, inc := range a.Names {
		dims = append(dims, Dimension{Name: "names", Value: key, Counter: fnLatencySetter(inc)})
	}

	return dims
}

// NewMCPRecordAggregate creates a new MCPRecordAggregate with all maps initialized.
func NewMCPRecordAggregate() MCPRecordAggregate {
	return MCPRecordAggregate{
		AnalyticsRecordAggregate: AnalyticsRecordAggregate{}.New(),
		Methods:                  make(map[string]*Counter),
		Primitives:               make(map[string]*Counter),
		Names:                    make(map[string]*Counter),
	}
}

// initMCPAggregateForRecord creates and initialises a new MCPRecordAggregate
// seeded from the first record seen for a given API.
func initMCPAggregateForRecord(record AnalyticsRecord, dbIdentifier string, aggregationTime int) MCPRecordAggregate {
	agg := NewMCPRecordAggregate()
	asTime := record.TimeStamp
	agg.TimeStamp = setAggregateTimestamp(dbIdentifier, asTime, aggregationTime)
	agg.ExpireAt = record.ExpireAt
	agg.TimeID.Year = asTime.Year()
	agg.TimeID.Month = int(asTime.Month())
	agg.TimeID.Day = asTime.Day()
	agg.TimeID.Hour = asTime.Hour()
	agg.OrgID = record.OrgID
	agg.LastTime = record.TimeStamp
	agg.Total.ErrorMap = make(map[string]int)
	return agg
}

// incrementMCPDimensions updates the MCP-specific dimension counters (method,
// primitive type, primitive name) for a single record.
func (a *MCPRecordAggregate) incrementMCPDimensions(counter Counter, rec MCPRecord) {
	if method := rec.JSONRPCMethod; method != "" {
		c := incrementOrSetUnit(&counter, a.Methods[method])
		a.Methods[method] = c
		a.Methods[method].Identifier = method
		a.Methods[method].HumanIdentifier = method
	}

	if primType := rec.PrimitiveType; primType != "" {
		c := incrementOrSetUnit(&counter, a.Primitives[primType])
		a.Primitives[primType] = c
		a.Primitives[primType].Identifier = primType
		a.Primitives[primType].HumanIdentifier = primType
	}

	if primName := rec.PrimitiveName; primName != "" {
		label := primName
		if rec.PrimitiveType != "" {
			label = fmt.Sprintf("%s_%s", rec.PrimitiveType, primName)
		}
		c := incrementOrSetUnit(&counter, a.Names[label])
		a.Names[label] = c
		a.Names[label].Identifier = label
		a.Names[label].HumanIdentifier = primName
	}
}

// AsTimeUpdate builds the MongoDB $set document for recalculating averages and lists.
// It extends the base AsTimeUpdate with MCP-specific lists for methods, primitives, and names.
func (a *MCPRecordAggregate) AsTimeUpdate() model.DBM {
	newUpdate := a.AnalyticsRecordAggregate.AsTimeUpdate()

	newUpdate["$set"].(model.DBM)["lists.methods"] = a.AnalyticsRecordAggregate.getRecords("methods", a.Methods, newUpdate)
	newUpdate["$set"].(model.DBM)["lists.primitives"] = a.AnalyticsRecordAggregate.getRecords("primitives", a.Primitives, newUpdate)
	newUpdate["$set"].(model.DBM)["lists.names"] = a.AnalyticsRecordAggregate.getRecords("names", a.Names, newUpdate)

	return newUpdate
}

// AggregateMCPData collects MCP records into a map of MCPRecordAggregate keyed by APIID.
func AggregateMCPData(data []interface{}, dbIdentifier string, aggregationTime int) map[string]MCPRecordAggregate {
	aggregateMap := make(map[string]MCPRecordAggregate)

	for _, item := range data {
		record, ok := item.(AnalyticsRecord)
		if !ok || !record.IsMCPRecord() {
			continue
		}

		mcpRec := record.ToMCPRecord()

		aggregate, found := aggregateMap[record.APIID]
		if !found {
			aggregate = initMCPAggregateForRecord(record, dbIdentifier, aggregationTime)
		}

		var counter Counter
		aggregate.AnalyticsRecordAggregate, counter = incrementAggregate(&aggregate.AnalyticsRecordAggregate, &mcpRec.AnalyticsRecord, false, nil)
		aggregate.incrementMCPDimensions(counter, mcpRec)

		aggregateMap[record.APIID] = aggregate
	}

	return aggregateMap
}
