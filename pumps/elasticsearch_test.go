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

func TestGetMapping_BasicFields(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	record := analytics.AnalyticsRecord{
		TimeStamp:     ts,
		Method:        "GET",
		Path:          "/api/v1/users",
		RawPath:       "/api/v1/users?page=1",
		ResponseCode:  200,
		IPAddress:     "10.0.0.1",
		APIKey:        "key123",
		APIVersion:    "v1",
		APIName:       "users-api",
		APIID:         "api1",
		OrgID:         "org1",
		OauthID:       "oauth1",
		RequestTime:   42,
		Alias:         "users",
		ContentLength: 1024,
		Tags:          []string{"tag1", "tag2"},
	}

	mapping, id := getMapping(record, false, false, false)

	assert.Empty(t, id, "ID should be empty when generateID is false")
	assert.Equal(t, ts, mapping["@timestamp"])
	assert.Equal(t, "GET", mapping["http_method"])
	assert.Equal(t, "/api/v1/users", mapping["request_uri"])
	assert.Equal(t, "/api/v1/users?page=1", mapping["request_uri_full"])
	assert.Equal(t, 200, mapping["response_code"])
	assert.Equal(t, "10.0.0.1", mapping["ip_address"])
	assert.Equal(t, "key123", mapping["api_key"])
	assert.Equal(t, "v1", mapping["api_version"])
	assert.Equal(t, "users-api", mapping["api_name"])
	assert.Equal(t, "api1", mapping["api_id"])
	assert.Equal(t, "org1", mapping["org_id"])
	assert.Equal(t, "oauth1", mapping["oauth_id"])
	assert.Equal(t, int64(42), mapping["request_time_ms"])
	assert.Equal(t, "users", mapping["alias"])
	assert.Equal(t, int64(1024), mapping["content_length"])
	assert.Equal(t, []string{"tag1", "tag2"}, mapping["tags"])

	// Non-extended stats should NOT include raw_request/response
	assert.NotContains(t, mapping, "raw_request")
	assert.NotContains(t, mapping, "raw_response")
	assert.NotContains(t, mapping, "user_agent")
}

func TestGetMapping_GenerateID(t *testing.T) {
	record := analytics.AnalyticsRecord{
		TimeStamp:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Method:      "POST",
		Path:        "/mcp",
		IPAddress:   "10.0.0.1",
		APIID:       "api1",
		OauthID:     "o1",
		RequestTime: 100,
		Alias:       "test",
	}

	_, id1 := getMapping(record, false, true, false)
	assert.NotEmpty(t, id1)

	// Same record produces same ID (deterministic)
	_, id2 := getMapping(record, false, true, false)
	assert.Equal(t, id1, id2)

	// Different record produces different ID
	record.RequestTime = 200
	_, id3 := getMapping(record, false, true, false)
	assert.NotEqual(t, id1, id3)
}

func TestGetMapping_MCPWithExtendedAndGenerateID(t *testing.T) {
	record := analytics.AnalyticsRecord{
		TimeStamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Method:       "POST",
		Path:         "/mcp",
		APIID:        "api1",
		ResponseCode: 200,
		RawRequest:   "cmF3LXJlcXVlc3Q=", // "raw-request" in base64
		UserAgent:    "mcp-client/1.0",
		MCPStats: analytics.MCPStats{
			IsMCP:         true,
			JSONRPCMethod: "tools/call",
			PrimitiveType: "tool",
			PrimitiveName: "weather",
		},
	}

	mapping, id := getMapping(record, true, true, true)

	// MCP fields present
	assert.Equal(t, "tools/call", mapping[esMCPMethod])
	assert.Equal(t, "tool", mapping[esMCPPrimitiveType])
	assert.Equal(t, "weather", mapping[esMCPPrimitiveName])

	// Extended stats present with base64 decode
	assert.Equal(t, "raw-request", mapping["raw_request"])
	assert.Equal(t, "mcp-client/1.0", mapping["user_agent"])

	// ID generated
	assert.NotEmpty(t, id)
}

func TestGetIndexName_NoRolling(t *testing.T) {
	conf := &ElasticsearchConf{
		IndexName: "tyk_analytics",
	}
	assert.Equal(t, "tyk_analytics", getIndexName(conf))
}

func TestGetIndexName_Rolling(t *testing.T) {
	conf := &ElasticsearchConf{
		IndexName:    "tyk_analytics",
		RollingIndex: true,
	}
	today := time.Now().Format("2006.01.02")
	assert.Equal(t, "tyk_analytics-"+today, getIndexName(conf))
}

func TestGetMapping_ExtendedStatistics(t *testing.T) {
	record := analytics.AnalyticsRecord{
		APIID:       "api1",
		RawRequest:  "cmF3LXJlcXVlc3Q=", // base64 "raw-request"
		RawResponse: "cmF3LXJlc3BvbnNl", // base64 "raw-response"
		UserAgent:   "test-agent",
	}

	t.Run("extended stats with base64 decode", func(t *testing.T) {
		mapping, _ := getMapping(record, true, false, true)
		assert.Equal(t, "raw-request", mapping["raw_request"])
		assert.Equal(t, "raw-response", mapping["raw_response"])
		assert.Equal(t, "test-agent", mapping["user_agent"])
	})

	t.Run("extended stats without base64 decode", func(t *testing.T) {
		mapping, _ := getMapping(record, true, false, false)
		assert.Equal(t, record.RawRequest, mapping["raw_request"])
		assert.Equal(t, record.RawResponse, mapping["raw_response"])
	})

	t.Run("generate ID returns non-empty hash", func(t *testing.T) {
		_, id := getMapping(record, false, true, false)
		assert.NotEmpty(t, id)
	})
}
