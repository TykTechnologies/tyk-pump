package pumps

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestFilterMCPData(t *testing.T) {
	mcpRecord := analytics.AnalyticsRecord{
		APIID: "api1",
		OrgID: "org1",
		MCPStats: analytics.MCPStats{
			IsMCP:         true,
			JSONRPCMethod: "tools/call",
			PrimitiveType: "tool",
			PrimitiveName: "my_tool",
		},
	}

	restRecord := analytics.AnalyticsRecord{
		APIID: "api1",
		OrgID: "org1",
	}

	t.Run("filters only MCP records", func(t *testing.T) {
		data := []interface{}{mcpRecord, restRecord, mcpRecord}
		result := filterMCPData(data)
		assert.Len(t, result, 2)
	})

	t.Run("returns empty slice for no MCP records", func(t *testing.T) {
		data := []interface{}{restRecord, restRecord}
		result := filterMCPData(data)
		assert.Empty(t, result)
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := filterMCPData([]interface{}{})
		assert.Empty(t, result)
	})

	t.Run("skips non-AnalyticsRecord types", func(t *testing.T) {
		data := []interface{}{mcpRecord, "string", 42, nil}
		result := filterMCPData(data)
		assert.Len(t, result, 1)
	})
}

func TestConvertToMCPObjects(t *testing.T) {
	t.Run("converts AnalyticsRecord to MCPRecord", func(t *testing.T) {
		rec := &analytics.AnalyticsRecord{
			APIID: "api1",
			OrgID: "org1",
			MCPStats: analytics.MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "my_tool",
			},
		}

		result := convertToMCPObjects([]model.DBObject{rec})
		require.Len(t, result, 1)

		mcpRec, ok := result[0].(*analytics.MCPRecord)
		require.True(t, ok, "result should be *MCPRecord")
		assert.Equal(t, "api1", mcpRec.AnalyticsRecord.APIID)
		assert.Equal(t, "tool", mcpRec.PrimitiveType)
		assert.Equal(t, "my_tool", mcpRec.PrimitiveName)
		assert.Equal(t, "tools/call", mcpRec.JSONRPCMethod)
	})

	t.Run("skips non-AnalyticsRecord types", func(t *testing.T) {
		result := convertToMCPObjects([]model.DBObject{})
		assert.Empty(t, result)
	})
}

func TestMCPMongoPump_WriteData_EmptyCollectionName(t *testing.T) {
	p := &MCPMongoPump{}
	p.dbConf = &MongoConf{CollectionName: ""}
	p.log = logrus.WithField("prefix", "test")
	err := p.WriteData(context.Background(), []interface{}{
		analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no collection name")
}

func TestMCPMongoPump_WriteData_NoMCPRecords(t *testing.T) {
	p := &MCPMongoPump{}
	p.dbConf = &MongoConf{CollectionName: "test"}
	p.log = logrus.WithField("prefix", "test")
	err := p.WriteData(context.Background(), []interface{}{
		analytics.AnalyticsRecord{APIID: "rest-api", ResponseCode: 200},
	})
	assert.NoError(t, err)
}

func TestMCPMongoPump_New(t *testing.T) {
	p := &MCPMongoPump{}
	newP := p.New()
	assert.NotNil(t, newP)
	_, ok := newP.(*MCPMongoPump)
	assert.True(t, ok)
}

func TestMCPMongoPump_GetName(t *testing.T) {
	p := &MCPMongoPump{}
	assert.Equal(t, "MongoDB MCP Pump", p.GetName())
}

func TestMCPMongoPump_SetDecodingRequest(t *testing.T) {
	p := &MCPMongoPump{}
	// Should not panic with false
	p.SetDecodingRequest(false)
	// Should log warning with true (no panic)
	p.SetDecodingRequest(true)
}

func TestMCPMongoPump_SetDecodingResponse(t *testing.T) {
	p := &MCPMongoPump{}
	p.SetDecodingResponse(false)
	p.SetDecodingResponse(true)
}

func TestMCPMongoPump_Init_InvalidConfig(t *testing.T) {
	p := &MCPMongoPump{}
	err := p.Init("not-a-map")
	assert.Error(t, err)
}

func TestMCPMongoPump_WriteData_EmptyData(t *testing.T) {
	p := &MCPMongoPump{}
	p.dbConf = &MongoConf{CollectionName: "test"}
	p.log = logrus.WithField("prefix", "test")
	err := p.WriteData(context.Background(), []interface{}{})
	assert.NoError(t, err)
}

func newMCPMongoPump(t *testing.T) *MCPMongoPump {
	t.Helper()
	oldTableName := analytics.MCPSQLTableName
	analytics.MCPSQLTableName = ""
	t.Cleanup(func() { analytics.MCPSQLTableName = oldTableName })

	conf := defaultConf()
	if mongoURL := os.Getenv("TYK_TEST_MCP_MONGO_URL"); mongoURL != "" {
		conf.MongoURL = mongoURL
	}
	conf.CollectionName = "test_mcp_" + model.NewObjectID().Hex()
	pump := &MCPMongoPump{}
	pump.dbConf = &conf
	pump.log = log.WithField("prefix", mongoMCPPrefix)
	pump.MongoPump.CommonPumpConfig = pump.CommonPumpConfig
	pump.connect()
	t.Cleanup(func() {
		require.NoError(t, pump.store.Drop(context.Background(), dbObject{tableName: conf.CollectionName}))
	})
	return pump
}

func TestMCPMongoPump_WriteData_Roundtrip(t *testing.T) {
	pump := newMCPMongoPump(t)
	cases := append(loadMCPContextCases(t), loadMCPSignedCodeCases(t)...)
	records := make([]interface{}, 0, len(cases)+1)
	for _, tc := range cases {
		records = append(records, tc.Record)
	}
	records = append(records, analytics.AnalyticsRecord{APIID: "non-mcp", OrgID: "mcp-compatibility", ResponseCode: 200})
	require.NoError(t, pump.WriteData(context.Background(), records))
	collection := dbObject{tableName: pump.dbConf.CollectionName}
	var results []analytics.MCPRecord
	require.NoError(t, pump.store.Query(context.Background(), collection, &results, nil))
	require.Len(t, results, len(cases), "non-MCP record must not be persisted")
	byAPI := make(map[string]analytics.MCPRecord, len(results))
	for _, result := range results {
		require.NotContains(t, byAPI, result.AnalyticsRecord.APIID, "no duplicate identity")
		byAPI[result.AnalyticsRecord.APIID] = result
	}
	var rawResults []bson.Raw
	require.NoError(t, pump.store.Query(context.Background(), collection, &rawResults, nil))
	require.Len(t, rawResults, len(cases))
	rawByAPI := make(map[string]bson.Raw, len(rawResults))
	for _, raw := range rawResults {
		rawByAPI[raw.Lookup("apiid").StringValue()] = raw
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require.Contains(t, byAPI, tc.Record.APIID)
			assertMCPContextRecord(t, &tc.Record, byAPI[tc.Record.APIID])
			require.Contains(t, rawByAPI, tc.Record.APIID)
			raw := rawByAPI[tc.Record.APIID]
			assert.Equal(t, tc.Record.MCPStats.JSONRPCMethod, raw.Lookup("jsonrpcmethod").StringValue())
			assert.Equal(t, tc.Record.MCPStats.EffectiveProtocolVersion, raw.Lookup("effective_protocol_version").StringValue())
			assert.Equal(t, tc.Record.MCPStats.DeclaredProtocolVersion, raw.Lookup("declared_protocol_version").StringValue())
			assert.Equal(t, tc.Record.MCPStats.ProtocolVersionSource, raw.Lookup("protocol_version_source").StringValue())
			assert.Equal(t, int64(tc.Record.MCPStats.JSONRPCErrorCode), raw.Lookup("jsonrpc_error_code").AsInt64())
			code := int64(tc.Record.MCPStats.JSONRPCErrorCode)
			expectedType := bson.TypeInt64
			if code >= math.MinInt32 && code <= math.MaxInt32 {
				expectedType = bson.TypeInt32
			}
			assert.Equal(t, expectedType, raw.Lookup("jsonrpc_error_code").Type)
			_, err := raw.LookupErr("jsonrpc_method")
			assert.Error(t, err, "do not rename historical BSON method")
		})
	}
}
