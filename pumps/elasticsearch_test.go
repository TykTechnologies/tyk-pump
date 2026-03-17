package pumps

import (
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetMapping_MCPFieldsForMCPRecord verifies that MCP records produce
// mcp_method, mcp_primitive_type, and mcp_primitive_name fields in the ES mapping.
func TestGetMapping_MCPFieldsForMCPRecord(t *testing.T) {
	record := analytics.AnalyticsRecord{
		APIID:        "api1",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
		MCPStats: analytics.MCPStats{
			IsMCP:         true,
			JSONRPCMethod: "tools/call",
			PrimitiveType: "tool",
			PrimitiveName: "get_weather",
		},
	}

	mapping, _ := getMapping(record, false, false, false)

	require.Contains(t, mapping, esMCPMethod, "mcp_method field must be present for MCP records")
	require.Contains(t, mapping, esMCPPrimitiveType, "mcp_primitive_type field must be present for MCP records")
	require.Contains(t, mapping, esMCPPrimitiveName, "mcp_primitive_name field must be present for MCP records")
	assert.Equal(t, "tools/call", mapping[esMCPMethod])
	assert.Equal(t, "tool", mapping[esMCPPrimitiveType])
	assert.Equal(t, "get_weather", mapping[esMCPPrimitiveName])
}

// TestGetMapping_NoMCPFieldsForNonMCPRecord verifies backward compatibility:
// non-MCP records must not include any MCP fields in the ES mapping.
func TestGetMapping_NoMCPFieldsForNonMCPRecord(t *testing.T) {
	record := analytics.AnalyticsRecord{
		APIID:        "api1",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
	}

	mapping, _ := getMapping(record, false, false, false)

	assert.NotContains(t, mapping, esMCPMethod, "mcp_method must not appear for non-MCP records")
	assert.NotContains(t, mapping, esMCPPrimitiveType, "mcp_primitive_type must not appear for non-MCP records")
	assert.NotContains(t, mapping, esMCPPrimitiveName, "mcp_primitive_name must not appear for non-MCP records")
}

// TestGetIndexNameForRecord_MCPIndexSet verifies that MCP records are routed to
// the configured MCPIndexName when it is set.
func TestGetIndexNameForRecord_MCPIndexSet(t *testing.T) {
	conf := &ElasticsearchConf{
		IndexName:    "tyk_analytics",
		MCPIndexName: "tyk_mcp_analytics",
	}
	mcpRecord := analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}}
	restRecord := analytics.AnalyticsRecord{}

	assert.Equal(t, "tyk_mcp_analytics", getIndexNameForRecord(conf, mcpRecord))
	assert.Equal(t, "tyk_analytics", getIndexNameForRecord(conf, restRecord))
}

// TestGetIndexNameForRecord_MCPIndexNotSet verifies backward compatibility:
// when MCPIndexName is empty, all records (MCP and REST) use the default index.
func TestGetIndexNameForRecord_MCPIndexNotSet(t *testing.T) {
	conf := &ElasticsearchConf{
		IndexName: "tyk_analytics",
	}
	mcpRecord := analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}}
	restRecord := analytics.AnalyticsRecord{}

	assert.Equal(t, "tyk_analytics", getIndexNameForRecord(conf, mcpRecord))
	assert.Equal(t, "tyk_analytics", getIndexNameForRecord(conf, restRecord))
}

// TestGetIndexNameForRecord_RollingIndex verifies that rolling index date suffix
// is applied to both the default index and the MCP index.
func TestGetIndexNameForRecord_RollingIndex(t *testing.T) {
	conf := &ElasticsearchConf{
		IndexName:    "tyk_analytics",
		MCPIndexName: "tyk_mcp_analytics",
		RollingIndex: true,
	}
	today := time.Now().Format("2006.01.02")
	mcpRecord := analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}}
	restRecord := analytics.AnalyticsRecord{}

	assert.Equal(t, "tyk_mcp_analytics-"+today, getIndexNameForRecord(conf, mcpRecord))
	assert.Equal(t, "tyk_analytics-"+today, getIndexNameForRecord(conf, restRecord))
}

func TestElasticsearchPump_TLSConfig_ErrorCases(t *testing.T) {
	t.Run("should return wrapped error with invalid cert file", func(t *testing.T) {
		pump := &ElasticsearchPump{}
		pump.log = log.WithField("prefix", "test")
		pump.esConf = &ElasticsearchConf{
			ElasticsearchURL: "https://localhost:9200",
			IndexName:        "test",
			Version:          "7",
			UseSSL:           true,
			SSLCertFile:      "/nonexistent/cert.pem",
			SSLKeyFile:       "/nonexistent/key.pem",
		}

		operator, err := pump.getOperator()

		assert.Error(t, err)
		assert.Nil(t, operator)
		assert.Contains(t, err.Error(), "failed to configure TLS for Elasticsearch connection")
	})

	t.Run("should return wrapped error with invalid CA file", func(t *testing.T) {
		pump := &ElasticsearchPump{}
		pump.log = log.WithField("prefix", "test")
		pump.esConf = &ElasticsearchConf{
			ElasticsearchURL: "https://localhost:9200",
			IndexName:        "test",
			Version:          "7",
			UseSSL:           true,
			SSLCAFile:        "/nonexistent/ca.pem",
		}

		operator, err := pump.getOperator()

		assert.Error(t, err)
		assert.Nil(t, operator)
		assert.Contains(t, err.Error(), "failed to configure TLS for Elasticsearch connection")
	})
}
