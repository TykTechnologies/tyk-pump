package pumps

import (
	"testing"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
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
