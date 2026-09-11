package pumps

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mcpContextCase struct {
	Name   string                    `json:"name"`
	Record analytics.AnalyticsRecord `json:"record"`
}

func loadMCPContextCases(t *testing.T) []mcpContextCase {
	t.Helper()
	data, err := os.ReadFile("../analytics/testdata/mcp_context_cases.json")
	require.NoError(t, err)
	var cases []mcpContextCase
	require.NoError(t, json.Unmarshal(data, &cases))
	require.Len(t, cases, 9)
	for i := range cases {
		cases[i].Record.TimeStamp = time.Date(2026, 9, 11, 12, 0, i, 0, time.UTC)
	}
	return cases
}

func assertMCPContextRecord(t *testing.T, expected *analytics.AnalyticsRecord, actual analytics.MCPRecord) {
	t.Helper()
	assert.Equal(t, expected.APIID, actual.AnalyticsRecord.APIID)
	assert.Equal(t, expected.OrgID, actual.AnalyticsRecord.OrgID)
	assert.Equal(t, expected.ResponseCode, actual.AnalyticsRecord.ResponseCode)
	assert.Equal(t, expected.MCPStats.JSONRPCMethod, actual.JSONRPCMethod)
	assert.Equal(t, expected.MCPStats.PrimitiveType, actual.PrimitiveType)
	assert.Equal(t, expected.MCPStats.PrimitiveName, actual.PrimitiveName)
	assert.Equal(t, expected.MCPStats.EffectiveProtocolVersion, actual.EffectiveProtocolVersion)
	assert.Equal(t, expected.MCPStats.DeclaredProtocolVersion, actual.DeclaredProtocolVersion)
	assert.Equal(t, expected.MCPStats.ProtocolVersionSource, actual.ProtocolVersionSource)
	assert.Equal(t, expected.MCPStats.JSONRPCErrorCode, actual.JSONRPCErrorCode)
}

func TestElasticsearchMCPContextCompatibility(t *testing.T) {
	for _, tc := range append(loadMCPContextCases(t), loadMCPSignedCodeCases(t)...) {
		t.Run(tc.Name, func(t *testing.T) {
			mapping, _ := getMapping(tc.Record, false, false, false)
			assert.Equal(t, tc.Record.MCPStats.JSONRPCMethod, mapping[esMCPMethod])
			assert.Equal(t, tc.Record.MCPStats.PrimitiveType, mapping[esMCPPrimitiveType])
			assert.Equal(t, tc.Record.MCPStats.PrimitiveName, mapping[esMCPPrimitiveName])
			assert.Equal(t, tc.Record.MCPStats.EffectiveProtocolVersion, mapping[esMCPEffectiveProtocolVersion])
			assert.Equal(t, tc.Record.MCPStats.DeclaredProtocolVersion, mapping[esMCPDeclaredProtocolVersion])
			assert.Equal(t, tc.Record.MCPStats.ProtocolVersionSource, mapping[esMCPProtocolVersionSource])
			assert.Equal(t, tc.Record.MCPStats.JSONRPCErrorCode, mapping[esMCPJSONRPCErrorCode])
		})
	}
	t.Run("non-MCP", func(t *testing.T) {
		mapping, _ := getMapping(analytics.AnalyticsRecord{APIID: "ordinary"}, false, false, false)
		for _, key := range []string{esMCPMethod, esMCPPrimitiveType, esMCPPrimitiveName, esMCPEffectiveProtocolVersion, esMCPDeclaredProtocolVersion, esMCPProtocolVersionSource, esMCPJSONRPCErrorCode} {
			assert.NotContains(t, mapping, key)
		}
	})
}

func loadMCPSignedCodeCases(t *testing.T) []mcpContextCase {
	t.Helper()
	var cases []mcpContextCase
	for name, code := range map[string]int64{"zero": 0, "reserved-denial": -33002, "min32": math.MinInt32, "max32": math.MaxInt32, "below32": -2147483649, "above32": 2147483648, "min64": math.MinInt64, "max64": math.MaxInt64} {
		data, err := os.ReadFile("../analytics/testdata/mcp_signed_codes/" + name + ".json")
		require.NoError(t, err)
		var record analytics.AnalyticsRecord
		err = json.Unmarshal(data, &record)
		if code < int64(math.MinInt) || code > int64(math.MaxInt) {
			require.Error(t, err, "wide JSON must not silently narrow on a native32 target")
			continue
		}
		require.NoError(t, err)
		require.Equal(t, code, int64(record.MCPStats.JSONRPCErrorCode))
		require.Empty(t, record.APIKey)
		record.APIID = "mcp-signed-" + name
		record.OrgID = "mcp-compatibility"
		cases = append(cases, mcpContextCase{Name: "signed-" + name, Record: record})
	}
	return cases
}
