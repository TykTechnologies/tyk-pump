package pumps

import (
	"context"
	"testing"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMCPMongoPump_WriteData_EmptyData(t *testing.T) {
	p := &MCPMongoPump{}
	p.dbConf = &MongoConf{CollectionName: "test"}
	p.log = logrus.WithField("prefix", "test")
	err := p.WriteData(context.Background(), []interface{}{})
	assert.NoError(t, err)
}

func TestMCPMongoPump_Init(t *testing.T) {
	conf := defaultConf()
	conf.CollectionName = "test_mcp_init"

	pump := MCPMongoPump{}
	require.NoError(t, pump.Init(conf))
	t.Cleanup(func() {
		_ = pump.store.DropDatabase(context.Background())
	})

	assert.Equal(t, 10*MiB, pump.dbConf.MaxInsertBatchSizeBytes)
	assert.Equal(t, 10*MiB, pump.dbConf.MaxDocumentSizeBytes)
}

func TestMCPMongoPump_WriteData_Roundtrip(t *testing.T) {
	conf := defaultConf()
	conf.CollectionName = "test_mcp_roundtrip"

	pump := MCPMongoPump{}
	require.NoError(t, pump.Init(conf))
	t.Cleanup(func() {
		_ = pump.store.DropDatabase(context.Background())
	})

	records := []interface{}{
		analytics.AnalyticsRecord{
			APIID: "api1", OrgID: "org1", ResponseCode: 200,
			MCPStats: analytics.MCPStats{
				IsMCP: true, JSONRPCMethod: "tools/call",
				PrimitiveType: "tool", PrimitiveName: "get_weather",
			},
		},
		analytics.AnalyticsRecord{
			APIID: "api1", OrgID: "org1", ResponseCode: 500,
			MCPStats: analytics.MCPStats{
				IsMCP: true, JSONRPCMethod: "resources/read",
				PrimitiveType: "resource", PrimitiveName: "docs",
			},
		},
		// non-MCP record — must NOT appear in the collection
		analytics.AnalyticsRecord{
			APIID: "api1", OrgID: "org1", ResponseCode: 200,
		},
	}

	require.NoError(t, pump.WriteData(context.Background(), records))

	// Query back from MongoDB
	var results []analytics.MCPRecord
	d := dbObject{tableName: conf.CollectionName}
	require.NoError(t, pump.store.Query(context.Background(), d, &results, nil))

	require.Len(t, results, 2, "only MCP records should be stored")
	assert.Equal(t, "tools/call", results[0].JSONRPCMethod)
	assert.Equal(t, "tool", results[0].PrimitiveType)
	assert.Equal(t, "get_weather", results[0].PrimitiveName)
	assert.Equal(t, "resources/read", results[1].JSONRPCMethod)
	assert.Equal(t, "resource", results[1].PrimitiveType)
	assert.Equal(t, "docs", results[1].PrimitiveName)
}
