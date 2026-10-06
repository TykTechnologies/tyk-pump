package pumps

import (
	"context"
	"os"
	"testing"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	legacybson "gopkg.in/mgo.v2/bson"
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
	var assertRaw func(*testing.T, mcpContextCase)
	if pump.dbConf.MongoDriverType == persistent.OfficialMongo {
		var rawResults []bson.Raw
		require.NoError(t, pump.store.Query(context.Background(), collection, &rawResults, nil))
		require.Len(t, rawResults, len(cases))
		rawByAPI := make(map[string]bson.Raw, len(rawResults))
		for _, raw := range rawResults {
			rawByAPI[raw.Lookup("apiid").StringValue()] = raw
		}
		assertRaw = func(t *testing.T, tc mcpContextCase) {
			t.Helper()
			require.Contains(t, rawByAPI, tc.Record.APIID)
			raw := rawByAPI[tc.Record.APIID]
			assert.Equal(t, tc.Record.MCPStats.JSONRPCMethod, raw.Lookup("jsonrpcmethod").StringValue())
			assert.Equal(t, tc.Record.MCPStats.EffectiveProtocolVersion, raw.Lookup("effective_protocol_version").StringValue())
			assert.Equal(t, tc.Record.MCPStats.DeclaredProtocolVersion, raw.Lookup("declared_protocol_version").StringValue())
			assert.Equal(t, tc.Record.MCPStats.ProtocolVersionSource, raw.Lookup("protocol_version_source").StringValue())
			assert.Equal(t, tc.Record.MCPStats.JSONRPCErrorCode, raw.Lookup("jsonrpc_error_code").AsInt64())
			assert.Equal(t, bson.TypeInt64, raw.Lookup("jsonrpc_error_code").Type,
				"the persisted producer contract must remain int64 even for narrow values")
			_, err := raw.LookupErr("jsonrpc_method")
			assert.Error(t, err, "do not rename historical BSON method")
		}
	} else {
		var rawResults []legacybson.RawD
		require.NoError(t, pump.store.Query(context.Background(), collection, &rawResults, nil))
		require.Len(t, rawResults, len(cases))
		rawByAPI := make(map[string]map[string]legacybson.Raw, len(rawResults))
		for _, document := range rawResults {
			fields := make(map[string]legacybson.Raw, len(document))
			for _, element := range document {
				fields[element.Name] = element.Value
			}
			var apiID string
			require.NoError(t, fields["apiid"].Unmarshal(&apiID))
			rawByAPI[apiID] = fields
		}
		assertRaw = func(t *testing.T, tc mcpContextCase) {
			t.Helper()
			require.Contains(t, rawByAPI, tc.Record.APIID)
			fields := rawByAPI[tc.Record.APIID]
			assertLegacyString := func(name, expected string) {
				var actual string
				require.NoError(t, fields[name].Unmarshal(&actual))
				assert.Equal(t, expected, actual)
			}
			assertLegacyString("jsonrpcmethod", tc.Record.MCPStats.JSONRPCMethod)
			assertLegacyString("effective_protocol_version", tc.Record.MCPStats.EffectiveProtocolVersion)
			assertLegacyString("declared_protocol_version", tc.Record.MCPStats.DeclaredProtocolVersion)
			assertLegacyString("protocol_version_source", tc.Record.MCPStats.ProtocolVersionSource)
			var code int64
			require.NoError(t, fields["jsonrpc_error_code"].Unmarshal(&code))
			assert.Equal(t, tc.Record.MCPStats.JSONRPCErrorCode, code)
			assert.Equal(t, byte(0x12), fields["jsonrpc_error_code"].Kind,
				"the persisted producer contract must remain int64 even for narrow values")
			assert.NotContains(t, fields, "jsonrpc_method", "do not rename historical BSON method")
		}
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require.Contains(t, byAPI, tc.Record.APIID)
			actual := byAPI[tc.Record.APIID]
			assertMCPContextRecord(t, &tc.Record, &actual)
			assertRaw(t, tc)
		})
	}
}
