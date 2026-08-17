package analytics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestAnalyticsRecord_IsMCPRecord(t *testing.T) {
	t.Run("returns false for empty record", func(t *testing.T) {
		record := AnalyticsRecord{}
		assert.False(t, record.IsMCPRecord())
	})

	t.Run("returns false when IsMCP is false", func(t *testing.T) {
		record := AnalyticsRecord{
			MCPStats: MCPStats{IsMCP: false},
		}
		assert.False(t, record.IsMCPRecord())
	})

	t.Run("returns true when IsMCP is true", func(t *testing.T) {
		record := AnalyticsRecord{
			MCPStats: MCPStats{IsMCP: true},
		}
		assert.True(t, record.IsMCPRecord())
	})
}

func TestAnalyticsRecord_MCPStatsJSONMarshal(t *testing.T) {
	record := AnalyticsRecord{
		MCPStats: MCPStats{
			IsMCP:                    true,
			JSONRPCMethod:            "tools/call",
			PrimitiveType:            "tool",
			PrimitiveName:            "my_tool",
			EffectiveProtocolVersion: "2026-07-28",
			DeclaredProtocolVersion:  "2026-07-28",
			ProtocolVersionSource:    "header_body",
		},
	}

	data, err := json.Marshal(record)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	mcpStats, ok := result["mcp_stats"]
	require.True(t, ok, "mcp_stats must be present in JSON output")

	statsMap, ok := mcpStats.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, statsMap["is_mcp"])
	assert.Equal(t, "tools/call", statsMap["jsonrpc_method"])
	assert.Equal(t, "tool", statsMap["primitive_type"])
	assert.Equal(t, "my_tool", statsMap["primitive_name"])
	assert.Equal(t, "2026-07-28", statsMap["effective_protocol_version"])
	assert.Equal(t, "2026-07-28", statsMap["declared_protocol_version"])
	assert.Equal(t, "header_body", statsMap["protocol_version_source"])

	var oldRecord AnalyticsRecord
	require.NoError(t, json.Unmarshal([]byte(`{"mcp_stats":{"is_mcp":true,"jsonrpc_method":"tools/call"}}`), &oldRecord))
	assert.Empty(t, oldRecord.MCPStats.EffectiveProtocolVersion)
	assert.Empty(t, oldRecord.MCPStats.DeclaredProtocolVersion)
	assert.Empty(t, oldRecord.MCPStats.ProtocolVersionSource)
}

func TestAnalyticsRecord_ToMCPRecord(t *testing.T) {
	t.Run("returns zero-value for non-MCP record", func(t *testing.T) {
		record := AnalyticsRecord{
			APIID: "api1",
			OrgID: "org1",
		}
		mcpRecord := record.ToMCPRecord()
		assert.Empty(t, mcpRecord.JSONRPCMethod)
		assert.Empty(t, mcpRecord.PrimitiveType)
		assert.Empty(t, mcpRecord.PrimitiveName)
		assert.Empty(t, mcpRecord.AnalyticsRecord.APIID)
	})

	t.Run("converts MCP record with all identity fields", func(t *testing.T) {
		ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		record := AnalyticsRecord{
			APIID:     "api1",
			OrgID:     "org1",
			TimeStamp: ts,
			MCPStats: MCPStats{
				IsMCP:                    true,
				JSONRPCMethod:            "tools/call",
				PrimitiveType:            "tool",
				PrimitiveName:            "my_tool",
				EffectiveProtocolVersion: "2026-07-28",
				DeclaredProtocolVersion:  "2026-07-28",
				ProtocolVersionSource:    "header_body",
			},
		}

		mcpRecord := record.ToMCPRecord()

		assert.Equal(t, "tools/call", mcpRecord.JSONRPCMethod)
		assert.Equal(t, "tool", mcpRecord.PrimitiveType)
		assert.Equal(t, "my_tool", mcpRecord.PrimitiveName)
		assert.Equal(t, "2026-07-28", mcpRecord.EffectiveProtocolVersion)
		assert.Equal(t, "2026-07-28", mcpRecord.DeclaredProtocolVersion)
		assert.Equal(t, "header_body", mcpRecord.ProtocolVersionSource)
		assert.Equal(t, "api1", mcpRecord.AnalyticsRecord.APIID)
		assert.Equal(t, "org1", mcpRecord.AnalyticsRecord.OrgID)
		assert.Equal(t, ts, mcpRecord.AnalyticsRecord.TimeStamp)
	})

	t.Run("MCPRecord uses AnalyticsRecord TableName when MCPSQLTableName is empty", func(t *testing.T) {
		MCPSQLTableName = ""
		record := AnalyticsRecord{
			MCPStats: MCPStats{IsMCP: true},
		}
		mr := record.ToMCPRecord()
		assert.Equal(t, SQLTable, mr.TableName())
	})

	t.Run("MCPRecord uses MCPSQLTableName when set", func(t *testing.T) {
		MCPSQLTableName = "custom_mcp_table"
		record := AnalyticsRecord{
			MCPStats: MCPStats{IsMCP: true},
		}
		mr := record.ToMCPRecord()
		assert.Equal(t, "custom_mcp_table", mr.TableName())
		MCPSQLTableName = "" // reset
	})
}

func TestMCPRecord_BSONRoundTripProtocolContext(t *testing.T) {
	record := MCPRecord{
		JSONRPCMethod:            "tools/call",
		EffectiveProtocolVersion: "2026-07-28",
		DeclaredProtocolVersion:  "2026-07-28",
		ProtocolVersionSource:    "header_body",
		AnalyticsRecord: AnalyticsRecord{
			MCPStats: MCPStats{IsMCP: true},
		},
	}

	encoded, err := bson.Marshal(record)
	require.NoError(t, err)

	var document bson.M
	require.NoError(t, bson.Unmarshal(encoded, &document))
	assert.Equal(t, "2026-07-28", document["effective_protocol_version"])
	assert.Equal(t, "2026-07-28", document["declared_protocol_version"])
	assert.Equal(t, "header_body", document["protocol_version_source"])

	var decoded MCPRecord
	require.NoError(t, bson.Unmarshal(encoded, &decoded))
	assert.Equal(t, record.EffectiveProtocolVersion, decoded.EffectiveProtocolVersion)
	assert.Equal(t, record.DeclaredProtocolVersion, decoded.DeclaredProtocolVersion)
	assert.Equal(t, record.ProtocolVersionSource, decoded.ProtocolVersionSource)

	oldEncoded, err := bson.Marshal(bson.M{"jsonrpc_method": "tools/call"})
	require.NoError(t, err)
	var oldRecord MCPRecord
	require.NoError(t, bson.Unmarshal(oldEncoded, &oldRecord))
	assert.Empty(t, oldRecord.EffectiveProtocolVersion)
	assert.Empty(t, oldRecord.DeclaredProtocolVersion)
	assert.Empty(t, oldRecord.ProtocolVersionSource)
}

func TestMCPRecord_GetObjectID(t *testing.T) {
	r := &MCPRecord{}
	assert.Equal(t, model.ObjectID(""), r.GetObjectID())
}

func TestMCPRecord_SetObjectID(t *testing.T) {
	r := &MCPRecord{}
	r.SetObjectID("test-id")
	// SetObjectID is a no-op, so GetObjectID should still return ""
	assert.Equal(t, model.ObjectID(""), r.GetObjectID())
}
