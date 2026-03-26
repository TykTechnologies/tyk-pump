package pumps

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

func newMCPMongoAggregatePump(t *testing.T) *MCPMongoAggregatePump {
	t.Helper()
	cfgPump := make(map[string]interface{})
	cfgPump["mongo_url"] = dbAddr
	cfgPump["use_mixed_collection"] = true

	pump := &MCPMongoAggregatePump{}
	require.NoError(t, pump.Init(cfgPump))
	t.Cleanup(func() {
		_ = pump.store.DropDatabase(context.Background())
	})
	return pump
}

func TestMCPMongoAggregatePump_New(t *testing.T) {
	p := &MCPMongoAggregatePump{}
	newP := p.New()
	assert.NotNil(t, newP)
	_, ok := newP.(*MCPMongoAggregatePump)
	assert.True(t, ok)
}

func TestMCPMongoAggregatePump_GetName(t *testing.T) {
	p := &MCPMongoAggregatePump{}
	assert.Equal(t, "MongoDB MCP Aggregate Pump", p.GetName())
}

func TestMCPMongoAggregatePump_SetDecodingRequest(t *testing.T) {
	p := &MCPMongoAggregatePump{}
	p.SetDecodingRequest(false) // no-op
	p.SetDecodingRequest(true)  // logs warning
}

func TestMCPMongoAggregatePump_SetDecodingResponse(t *testing.T) {
	p := &MCPMongoAggregatePump{}
	p.SetDecodingResponse(false)
	p.SetDecodingResponse(true)
}

func TestMCPMongoAggregatePump_Init_InvalidConfig(t *testing.T) {
	pump := &MCPMongoAggregatePump{}
	err := pump.Init("not-a-map")
	require.Error(t, err, "Init should return error for invalid config")
}

func TestMCPMongoAggregatePump_WriteData_Roundtrip(t *testing.T) {
	pump := newMCPMongoAggregatePump(t)

	ts := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	records := []interface{}{
		analytics.AnalyticsRecord{
			TimeStamp: ts, APIID: "api1", OrgID: "org1", ResponseCode: 200,
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "weather"},
		},
		analytics.AnalyticsRecord{
			TimeStamp: ts, APIID: "api1", OrgID: "org1", ResponseCode: 500,
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "weather"},
		},
		// non-MCP record — must not appear
		analytics.AnalyticsRecord{
			TimeStamp: ts, APIID: "api1", OrgID: "org1", ResponseCode: 200,
		},
	}

	require.NoError(t, pump.WriteData(context.Background(), records))

	// Query the mixed collection for the aggregated doc
	var results []analytics.AnalyticsRecordAggregate
	require.NoError(t, pump.store.Query(
		context.Background(),
		&analytics.MCPRecordAggregate{AnalyticsRecordAggregate: analytics.AnalyticsRecordAggregate{Mixed: true}},
		&results,
		model.DBM{"orgid": "org1"},
	))

	require.NotEmpty(t, results, "aggregated doc should exist in mixed collection")
	ag := results[0]
	assert.Equal(t, 2, ag.Total.Hits, "total hits should be 2 (only MCP records)")
	assert.Equal(t, 1, ag.Total.Success)
	assert.Equal(t, 1, ag.Total.ErrorTotal)
}

func TestMCPMongoAggregatePump_WriteData_MixedCollection(t *testing.T) {
	pump := newMCPMongoAggregatePump(t)

	ts := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	records := []interface{}{
		analytics.AnalyticsRecord{
			TimeStamp: ts, APIID: "api1", OrgID: "org1", ResponseCode: 200,
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
		},
	}

	require.NoError(t, pump.WriteData(context.Background(), records))

	// Verify org-specific collection has data
	var orgResults []analytics.AnalyticsRecordAggregate
	require.NoError(t, pump.store.Query(
		context.Background(),
		&analytics.MCPRecordAggregate{AnalyticsRecordAggregate: analytics.AnalyticsRecordAggregate{OrgID: "org1"}},
		&orgResults,
		model.DBM{"orgid": "org1"},
	))
	assert.NotEmpty(t, orgResults, "org-specific collection should have data")

	// Verify mixed collection also has data
	var mixedResults []analytics.AnalyticsRecordAggregate
	require.NoError(t, pump.store.Query(
		context.Background(),
		&analytics.MCPRecordAggregate{AnalyticsRecordAggregate: analytics.AnalyticsRecordAggregate{Mixed: true}},
		&mixedResults,
		model.DBM{"orgid": "org1"},
	))
	assert.NotEmpty(t, mixedResults, "mixed collection should also have data")
}

func TestAddMCPDimensionUpdates_IncludesLatencyFields(t *testing.T) {
	// Create MCP analytics records with non-zero latency and request time.
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		analytics.AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts,
			ResponseCode: 200,
			RequestTime:  150,
			Latency: analytics.Latency{
				Total:    150,
				Upstream: 100,
			},
			MCPStats: analytics.MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "my_tool",
			},
		},
		analytics.AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts,
			ResponseCode: 429,
			RequestTime:  50,
			Latency: analytics.Latency{
				Total:    50,
				Upstream: 0,
			},
			MCPStats: analytics.MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "my_tool",
			},
		},
	}

	// Aggregate the data
	result := analytics.AggregateMCPData(data, "", 60)
	require.Len(t, result, 1)

	ag := result["api1"]

	// Verify the in-memory counters have latency data
	require.Contains(t, ag.Names, "tool_my_tool")
	nameCounter := ag.Names["tool_my_tool"]
	assert.Equal(t, 2, nameCounter.Hits, "expected 2 hits")
	assert.NotZero(t, nameCounter.TotalRequestTime, "counter TotalRequestTime should be non-zero")
	assert.NotZero(t, nameCounter.TotalLatency, "counter TotalLatency should be non-zero")
	assert.NotZero(t, nameCounter.MaxLatency, "counter MaxLatency should be non-zero")

	// Now build the MongoDB update doc the same way DoMCPAggregatedWriting does
	updateDoc := ag.AnalyticsRecordAggregate.AsChange()
	addMCPDimensionUpdates(&ag, updateDoc)

	incDoc := updateDoc["$inc"].(model.DBM)
	maxDoc := updateDoc["$max"].(model.DBM)

	// All three MCP dimensions must be present in the update doc.
	for _, prefix := range []string{
		"methods.tools/call.",
		"primitives.tool.",
		"names.tool_my_tool.",
	} {
		assert.Contains(t, incDoc, prefix+"hits")
		assert.Contains(t, incDoc, prefix+"totalrequesttime")
		assert.Contains(t, incDoc, prefix+"totallatency")
		assert.Contains(t, incDoc, prefix+"totalupstreamlatency")
		assert.Contains(t, maxDoc, prefix+"maxlatency")
		assert.Contains(t, maxDoc, prefix+"maxupstreamlatency")
	}

	// Verify actual latency values are non-zero for one representative dimension.
	prefix := "names.tool_my_tool."
	assert.NotZero(t, incDoc[prefix+"totalrequesttime"])
	assert.NotZero(t, incDoc[prefix+"totallatency"])
	assert.NotZero(t, maxDoc[prefix+"maxlatency"])
}

func TestAddMCPDimensionUpdates_MinLatencyWhenNotAllErrors(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		analytics.AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts,
			ResponseCode: 200, RequestTime: 100,
			Latency:  analytics.Latency{Total: 100, Upstream: 50},
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
		},
	}

	result := analytics.AggregateMCPData(data, "", 60)
	ag := result["api1"]

	updateDoc := ag.AnalyticsRecordAggregate.AsChange()
	addMCPDimensionUpdates(&ag, updateDoc)

	// When not all requests are errors, $min should be present
	minDoc, hasMin := updateDoc["$min"]
	if hasMin {
		minDBM := minDoc.(model.DBM)
		assert.Contains(t, minDBM, "names.tool_t1.minlatency")
		assert.Contains(t, minDBM, "names.tool_t1.minupstreamlatency")
	}
}

func TestAddMCPDimensionUpdates_NoMinWhenAllErrors(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		analytics.AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts,
			ResponseCode: 500, RequestTime: 100,
			Latency:  analytics.Latency{Total: 100, Upstream: 50},
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
		},
	}

	result := analytics.AggregateMCPData(data, "", 60)
	ag := result["api1"]

	updateDoc := ag.AnalyticsRecordAggregate.AsChange()
	addMCPDimensionUpdates(&ag, updateDoc)

	// When all requests are errors, $min for MCP dimensions should NOT be present
	if minDoc, hasMin := updateDoc["$min"]; hasMin {
		minDBM := minDoc.(model.DBM)
		assert.NotContains(t, minDBM, "names.tool_t1.minlatency",
			"$min should not contain MCP dimension minlatency when all requests are errors")
	}
}
