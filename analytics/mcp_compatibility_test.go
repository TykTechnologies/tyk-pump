package analytics

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestMCPContextJSONBSONCompatibility(t *testing.T) {
	data, err := os.ReadFile("testdata/mcp_context_cases.json")
	require.NoError(t, err)
	var cases []struct {
		Name string `json:"name"`
		Record AnalyticsRecord `json:"record"`
	}
	require.NoError(t, json.Unmarshal(data, &cases))
	require.Len(t, cases, 9)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.Record)
			require.NoError(t, err)
			var decoded AnalyticsRecord
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			assert.Equal(t, tc.Record.MCPStats, decoded.MCPStats)
			assert.Equal(t, tc.Record.APIID, decoded.APIID)
			assert.Equal(t, tc.Record.OrgID, decoded.OrgID)

			converted := tc.Record.ToMCPRecord()
			assert.Equal(t, tc.Record.MCPStats.JSONRPCMethod, converted.JSONRPCMethod)
			assert.Equal(t, tc.Record.MCPStats.PrimitiveType, converted.PrimitiveType)
			assert.Equal(t, tc.Record.MCPStats.PrimitiveName, converted.PrimitiveName)
			assert.Equal(t, tc.Record.MCPStats.JSONRPCErrorCode, converted.JSONRPCErrorCode)
			assert.Equal(t, tc.Record.MCPStats.EffectiveProtocolVersion, converted.EffectiveProtocolVersion)
			assert.Equal(t, tc.Record.MCPStats.DeclaredProtocolVersion, converted.DeclaredProtocolVersion)
			assert.Equal(t, tc.Record.MCPStats.ProtocolVersionSource, converted.ProtocolVersionSource)
			assert.Equal(t, tc.Record.APIID, converted.AnalyticsRecord.APIID)
			assert.Equal(t, tc.Record.OrgID, converted.AnalyticsRecord.OrgID)

			for _, codec := range []struct {
				name string
				marshal func(interface{}) ([]byte, error)
				unmarshal func([]byte, interface{}) error
			}{{"json", json.Marshal, json.Unmarshal}, {"bson", bson.Marshal, bson.Unmarshal}} {
				t.Run(codec.name, func(t *testing.T) {
					encoded, err := codec.marshal(converted)
					require.NoError(t, err)
					var restored MCPRecord
					require.NoError(t, codec.unmarshal(encoded, &restored))
					assert.Equal(t, converted.JSONRPCMethod, restored.JSONRPCMethod)
					assert.Equal(t, converted.PrimitiveType, restored.PrimitiveType)
					assert.Equal(t, converted.PrimitiveName, restored.PrimitiveName)
					assert.Equal(t, converted.JSONRPCErrorCode, restored.JSONRPCErrorCode)
					assert.Equal(t, converted.EffectiveProtocolVersion, restored.EffectiveProtocolVersion)
					assert.Equal(t, converted.DeclaredProtocolVersion, restored.DeclaredProtocolVersion)
					assert.Equal(t, converted.ProtocolVersionSource, restored.ProtocolVersionSource)
					assert.Equal(t, converted.AnalyticsRecord.APIID, restored.AnalyticsRecord.APIID)
					assert.Equal(t, converted.AnalyticsRecord.OrgID, restored.AnalyticsRecord.OrgID)
				})
			}
			encoded, err = bson.Marshal(converted)
			require.NoError(t, err)
			raw := bson.Raw(encoded)
			assert.Equal(t, converted.JSONRPCMethod, raw.Lookup("jsonrpcmethod").StringValue())
			assert.Equal(t, int64(converted.JSONRPCErrorCode), raw.Lookup("jsonrpc_error_code").AsInt64())
			assert.Equal(t, converted.EffectiveProtocolVersion, raw.Lookup("effective_protocol_version").StringValue())
			assert.Equal(t, converted.DeclaredProtocolVersion, raw.Lookup("declared_protocol_version").StringValue())
			assert.Equal(t, converted.ProtocolVersionSource, raw.Lookup("protocol_version_source").StringValue())
			_, err = raw.LookupErr("jsonrpc_method")
			assert.Error(t, err, "historical BSON method name must remain unchanged")
		})
	}
}

func TestMCPLegacyImmutableRecordFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/mcp_legacy_analytics.json")
	require.NoError(t, err)
	var original AnalyticsRecord
	require.NoError(t, json.Unmarshal(data, &original))
	assert.True(t, original.IsMCPRecord())
	for _, codec := range []string{"json", "bson"} {
		t.Run(codec, func(t *testing.T) {
			var restored MCPRecord
			if codec == "json" {
				data, err := os.ReadFile("testdata/mcp_legacy_record.json")
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(data, &restored))
			} else {
				data, err := os.ReadFile("testdata/mcp_legacy_record.bson.hex")
				require.NoError(t, err)
				encoded, err := hex.DecodeString(strings.TrimSpace(string(data)))
				require.NoError(t, err)
				require.NoError(t, bson.Unmarshal(encoded, &restored))
			}
			assert.Equal(t, "legacy-api", restored.AnalyticsRecord.APIID)
			assert.Equal(t, "legacy-org", restored.AnalyticsRecord.OrgID)
			assert.Equal(t, "tools/call", restored.JSONRPCMethod)
			assert.Equal(t, "tool", restored.PrimitiveType)
			assert.Equal(t, "legacy_tool", restored.PrimitiveName)
			assert.Zero(t, restored.JSONRPCErrorCode)
			assert.Empty(t, restored.EffectiveProtocolVersion)
			assert.Empty(t, restored.DeclaredProtocolVersion)
			assert.Empty(t, restored.ProtocolVersionSource)
		})
	}
	assert.Equal(t, "legacy-api", original.APIID)
	assert.Equal(t, "legacy-org", original.OrgID)
	assert.Equal(t, "tools/call", original.MCPStats.JSONRPCMethod)
	assert.Equal(t, "tool", original.MCPStats.PrimitiveType)
	assert.Equal(t, "legacy_tool", original.MCPStats.PrimitiveName)
	assert.Zero(t, original.MCPStats.JSONRPCErrorCode)
	assert.Empty(t, original.MCPStats.EffectiveProtocolVersion)
	assert.Empty(t, original.MCPStats.DeclaredProtocolVersion)
	assert.Empty(t, original.MCPStats.ProtocolVersionSource)
}
