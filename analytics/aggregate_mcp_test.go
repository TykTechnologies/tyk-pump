package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/TykTechnologies/storage/persistent/model"
)

func TestAggregateMCPData_SkipsNonMCPRecords(t *testing.T) {
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "org1",
			APIID: "api1",
			MCPStats: MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "my_tool",
			},
			TimeStamp:    time.Now(),
			ResponseCode: 200,
		},
		AnalyticsRecord{
			OrgID:        "org1",
			APIID:        "api1",
			TimeStamp:    time.Now(),
			ResponseCode: 200,
			// no MCPStats — REST record
		},
		AnalyticsRecord{
			OrgID: "org1",
			APIID: "api1",
			GraphQLStats: GraphQLStats{
				IsGraphQL: true,
			},
			TimeStamp:    time.Now(),
			ResponseCode: 200,
		},
	}

	result := AggregateMCPData(data, "", 60)
	require.Len(t, result, 1)

	agg := result["api1"]
	assert.Equal(t, 1, agg.Total.Hits)
}

func TestAggregateMCPData_AggregatesByMethod(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "tool_a"},
		},
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "tool_b"},
		},
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "resources/read", PrimitiveType: "resource", PrimitiveName: "res_a"},
		},
	}

	result := AggregateMCPData(data, "", 60)
	require.Len(t, result, 1)

	agg := result["api1"]
	assert.Equal(t, 3, agg.Total.Hits)

	require.Contains(t, agg.Methods, "tools/call")
	assert.Equal(t, 2, agg.Methods["tools/call"].Hits)

	require.Contains(t, agg.Methods, "resources/read")
	assert.Equal(t, 1, agg.Methods["resources/read"].Hits)
}

func TestAggregateMCPData_AggregatesByPrimitiveType(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "tool_a"},
		},
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "resources/read", PrimitiveType: "resource", PrimitiveName: "res_a"},
		},
	}

	result := AggregateMCPData(data, "", 60)
	agg := result["api1"]

	require.Contains(t, agg.Primitives, "tool")
	assert.Equal(t, 1, agg.Primitives["tool"].Hits)

	require.Contains(t, agg.Primitives, "resource")
	assert.Equal(t, 1, agg.Primitives["resource"].Hits)
}

func TestAggregateMCPData_AggregatesByPrimitiveName(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "my_tool"},
		},
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "my_tool"},
		},
	}

	result := AggregateMCPData(data, "", 60)
	agg := result["api1"]

	require.Contains(t, agg.Names, "tool_my_tool")
	assert.Equal(t, 2, agg.Names["tool_my_tool"].Hits)
}

func TestMCPRecordAggregate_TableName(t *testing.T) {
	t.Run("returns MCP-specific mixed collection when Mixed=true", func(t *testing.T) {
		agg := MCPRecordAggregate{}
		agg.Mixed = true
		assert.Equal(t, MCPAggregateMixedCollectionName, agg.TableName())
	})

	t.Run("returns org-specific collection when Mixed=false", func(t *testing.T) {
		agg := MCPRecordAggregate{}
		agg.OrgID = "myorg"
		assert.Equal(t, "z_tyk_mcp_analyticz_aggregate_myorg", agg.TableName())
	})
}

func TestMCPRecordAggregate_Dimensions(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "my_tool"},
		},
	}

	result := AggregateMCPData(data, "", 60)
	agg := result["api1"]

	dims := agg.Dimensions()

	dimNames := make(map[string]bool)
	for _, d := range dims {
		dimNames[d.Name] = true
	}

	assert.True(t, dimNames["methods"], "methods dimension must be present")
	assert.True(t, dimNames["primitives"], "primitives dimension must be present")
	assert.True(t, dimNames["names"], "names dimension must be present")
}

// TestMCPAsTimeUpdate_ProducesListsAPIID verifies that AsTimeUpdate() on an
// MCP aggregate produces non-empty lists.apiid. If lists.apiid is empty in the
// MongoDB document, the /api/usage/mcp/ endpoint returns null because the
// $unwind on $lists.apiid produces no results.
func TestMCPAsTimeUpdate_ProducesListsAPIID(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", APIName: "My MCP Proxy",
			TimeStamp: ts, ResponseCode: 200, RequestTime: 100,
			Latency: Latency{Total: 100, Upstream: 50},
			MCPStats: MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "get_products",
			},
		},
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", APIName: "My MCP Proxy",
			TimeStamp: ts, ResponseCode: 429, RequestTime: 10,
			Latency: Latency{Total: 10, Upstream: 0},
			MCPStats: MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "get_products",
			},
		},
	}

	result := AggregateMCPData(data, "", 60)
	require.Len(t, result, 1, "should aggregate into one entry keyed by APIID")
	ag := result["api1"]

	// Verify the APIID dimension map is populated during aggregation.
	require.Contains(t, ag.APIID, "api1",
		"BUG: APIID dimension not populated — /api/usage/mcp/ will return null")
	assert.Equal(t, 2, ag.APIID["api1"].Hits)
	assert.Equal(t, "api1", ag.APIID["api1"].Identifier)
	assert.Equal(t, "My MCP Proxy", ag.APIID["api1"].HumanIdentifier)

	// Verify AsTimeUpdate produces non-empty lists.apiid.
	update := ag.AsTimeUpdate()
	setDoc, ok := update["$set"].(model.DBM)
	require.True(t, ok, "$set must be model.DBM")

	apiidRaw, exists := setDoc["lists.apiid"]
	require.True(t, exists,
		"BUG: lists.apiid missing from $set — /api/usage/mcp/ will return null")

	apiidList, ok := apiidRaw.([]Counter)
	require.True(t, ok, "lists.apiid must be []Counter, got %T", apiidRaw)
	require.NotEmpty(t, apiidList,
		"BUG: lists.apiid is empty — /api/usage/mcp/ endpoint will get no results from $unwind")

	// Verify the Counter in lists.apiid has correct identifiers.
	found := false
	for _, c := range apiidList {
		if c.Identifier == "api1" {
			found = true
			assert.Equal(t, "My MCP Proxy", c.HumanIdentifier)
			assert.Equal(t, 2, c.Hits)
			break
		}
	}
	assert.True(t, found, "lists.apiid must contain a Counter with Identifier='api1'")

	// Also verify lists.names is populated (primitives endpoint uses this).
	namesRaw, exists := setDoc["lists.names"]
	require.True(t, exists, "lists.names must exist")
	namesList, ok := namesRaw.([]Counter)
	require.True(t, ok)
	require.NotEmpty(t, namesList, "lists.names must not be empty")
}

// TestMCPAsTimeUpdate_ProducesErrorListForNames verifies that AsTimeUpdate
// correctly populates errorlist for each entry in lists.names. The
// /api/activity/mcp/primitives/errors/ endpoint $unwinds lists.names.errorlist
// and if errorlist is empty, no error data appears.
func TestMCPAsTimeUpdate_ProducesErrorListForNames(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "get_products"},
		},
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 429,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "get_products"},
		},
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts, ResponseCode: 429,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "get_products"},
		},
	}

	result := AggregateMCPData(data, "", 60)
	ag := result["api1"]

	// Verify ErrorMap is populated on the Names counter.
	nameKey := "tool_get_products"
	require.Contains(t, ag.Names, nameKey)
	nameCounter := ag.Names[nameKey]
	assert.Equal(t, 3, nameCounter.Hits)
	assert.Equal(t, 2, nameCounter.ErrorTotal)
	require.Contains(t, nameCounter.ErrorMap, "429",
		"BUG: ErrorMap not populated for Names dimension — errors endpoint won't show status codes")
	assert.Equal(t, 2, nameCounter.ErrorMap["429"])

	// Verify AsTimeUpdate produces errorlist for the names entry.
	update := ag.AsTimeUpdate()
	setDoc := update["$set"].(model.DBM)

	// The SetErrorList function writes "names.<key>.errorlist" to $set.
	errorlistKey := "names." + nameKey + ".errorlist"
	errorlistRaw, exists := setDoc[errorlistKey]
	require.True(t, exists,
		"BUG: %s missing from $set — errors endpoint won't find error codes", errorlistKey)

	errorlist, ok := errorlistRaw.([]ErrorData)
	require.True(t, ok, "errorlist must be []ErrorData, got %T", errorlistRaw)
	require.NotEmpty(t, errorlist, "errorlist must not be empty")

	// Verify the errorlist contains code 429 with count 2.
	found := false
	for _, ed := range errorlist {
		if ed.Code == "429" {
			found = true
			assert.Equal(t, 2, ed.Count)
		}
	}
	assert.True(t, found, "errorlist must contain code 429")
}

// TestMCPUpsertReadback_EmptyDocProducesEmptyLists proves the bug: upsertMCPAggregate
// creates a fresh MCPRecordAggregate (doc) with only OrgID/Mixed set, then calls
// store.Upsert which should read back the MongoDB document into doc. If the BSON
// readback doesn't populate doc.APIID (the dimension map), then doc.AsTimeUpdate()
// produces empty lists.apiid, causing /api/usage/mcp/ to return null.
//
// This test simulates the scenario by calling AsTimeUpdate on an empty aggregate.
func TestMCPUpsertReadback_EmptyDocProducesEmptyLists(t *testing.T) {
	// This is what upsertMCPAggregate creates BEFORE the Upsert readback:
	doc := &MCPRecordAggregate{
		AnalyticsRecordAggregate: AnalyticsRecordAggregate{OrgID: "org1", Mixed: false},
	}

	// If the MongoDB readback doesn't populate the maps, AsTimeUpdate
	// produces empty lists — proving the endpoint returns null.
	update := doc.AsTimeUpdate()
	setDoc := update["$set"].(model.DBM)

	apiidList := setDoc["lists.apiid"].([]Counter)
	assert.Empty(t, apiidList,
		"Without readback, lists.apiid is empty — proves /api/usage/mcp/ returns null")

	namesList := setDoc["lists.names"].([]Counter)
	assert.Empty(t, namesList,
		"Without readback, lists.names is empty — but primitives endpoint still works because it queries the names dimension map directly")
}

// TestMCPBSONRoundTrip_APIIDMapSurvivesReadback proves/disproves a bug in the
// BSON decode path: after the first upsert in upsertMCPAggregate, the MongoDB
// document is decoded back into *MCPRecordAggregate. If the BSON decoder fails
// to populate the embedded AnalyticsRecordAggregate.APIID map, then
// AsTimeUpdate() writes empty lists.apiid, and /api/usage/mcp/ returns null.
//
// This test creates a BSON document mimicking what MongoDB produces after the
// first upsert, then decodes it into MCPRecordAggregate and checks if APIID
// and Names are both populated.
func TestMCPBSONRoundTrip_APIIDMapSurvivesReadback(t *testing.T) {
	// Simulate the MongoDB document after the first upsert ($inc/$set).
	// This is what FindOneAndUpdate returns.
	mongoDoc := bson.M{
		"_id":       "test_id",
		"orgid":     "org1",
		"timestamp": time.Now(),
		"timeid": bson.M{
			"year": 2024, "month": 6, "day": 15, "hour": 10,
		},
		"total": bson.M{
			"hits": 2, "success": 1, "errortotal": 1,
			"totalrequesttime": 200.0, "totallatency": int64(200),
			"totalupstreamlatency": int64(100),
			"maxlatency":           int64(100), "maxupstreamlatency": int64(50),
			"identifier": "", "humanidentifier": "",
			"errormap": bson.M{"429": 1},
		},
		// This is the APIID dimension map written by AsChange/$inc
		"apiid": bson.M{
			"api1": bson.M{
				"hits": 2, "success": 1, "errortotal": 1,
				"totalrequesttime": 200.0, "totallatency": int64(200),
				"totalupstreamlatency": int64(100),
				"maxlatency":           int64(100), "maxupstreamlatency": int64(50),
				"identifier": "api1", "humanidentifier": "My MCP Proxy",
				"errormap": bson.M{"429": 1},
			},
		},
		// MCP-specific dimension maps
		"methods": bson.M{
			"tools/call": bson.M{
				"hits": 2, "success": 1, "errortotal": 1,
				"identifier": "tools/call", "humanidentifier": "tools/call",
			},
		},
		"names": bson.M{
			"tool_get_products": bson.M{
				"hits": 2, "success": 1, "errortotal": 1,
				"identifier": "tool_get_products", "humanidentifier": "get_products",
				"errormap": bson.M{"429": 1},
			},
		},
		"primitives": bson.M{
			"tool": bson.M{
				"hits": 2, "success": 1, "errortotal": 1,
				"identifier": "tool", "humanidentifier": "tool",
			},
		},
	}

	// Marshal to BSON bytes, then unmarshal into MCPRecordAggregate
	// (simulating what the mongo driver's Decode does).
	bsonBytes, err := bson.Marshal(mongoDoc)
	require.NoError(t, err, "failed to marshal simulated MongoDB document")

	var doc MCPRecordAggregate
	err = bson.Unmarshal(bsonBytes, &doc)
	require.NoError(t, err, "failed to unmarshal into MCPRecordAggregate")

	// Verify MCP-specific maps are populated (these work in production).
	assert.NotEmpty(t, doc.Names,
		"Names map should be populated after BSON decode")
	assert.NotEmpty(t, doc.Methods,
		"Methods map should be populated after BSON decode")

	// This is the critical assertion: APIID map from the embedded
	// AnalyticsRecordAggregate must also be populated.
	assert.NotEmpty(t, doc.APIID,
		"BUG: APIID map is empty after BSON decode — this causes /api/usage/mcp/ to return null "+
			"because AsTimeUpdate() produces empty lists.apiid")

	if len(doc.APIID) > 0 {
		assert.Contains(t, doc.APIID, "api1")
		assert.Equal(t, 2, doc.APIID["api1"].Hits)
		assert.Equal(t, "My MCP Proxy", doc.APIID["api1"].HumanIdentifier)
	}
}

func TestAggregateData_SkipsMCPRecords(t *testing.T) {
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: time.Now(), ResponseCode: 200,
		},
		AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: time.Now(), ResponseCode: 200,
			MCPStats: MCPStats{IsMCP: true, JSONRPCMethod: "tools/call"},
		},
	}

	result := AggregateData(data, false, []string{}, "", 60)
	require.Len(t, result, 1)

	// Only the REST record should be aggregated — MCP record must be excluded
	assert.Equal(t, 1, result["org1"].Total.Hits)
}
