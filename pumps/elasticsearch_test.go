package pumps

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File-level MC/DC witness rows: these requirements are genuinely exercised
// by covered tests in this file (per-test // MCDC blocks below). Rows copied
// verbatim from `proof mcdc show`; this header gives every // Verifies: link
// in the file a matching witness row.
//
// MCDC SW-REQ-069: index_eq_mcp=F, is_mcp_record=F, mcp_index_configured=T => TRUE
// MCDC SW-REQ-069: index_eq_mcp=F, is_mcp_record=T, mcp_index_configured=F => TRUE
// MCDC SW-REQ-069: index_eq_mcp=T, is_mcp_record=T, mcp_index_configured=T => TRUE

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
// Verifies: INT-REQ-006
// MCDC INT-REQ-006: mapping_per_implementation=T, record_dispatched_to_backend=T => TRUE
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
// Verifies: SW-REQ-069
// MCDC SW-REQ-069: index_eq_mcp=T, is_mcp_record=T, mcp_index_configured=T => TRUE
// MCDC SW-REQ-069: index_eq_mcp=F, is_mcp_record=F, mcp_index_configured=T => TRUE
//
// With mcp_index_configured=T: the MCP record resolves to MCPIndexName
// (index_eq_mcp=T -> satisfied row 4) and the REST record resolves to IndexName
// (is_mcp_record=F, index_eq_mcp=F -> vacuous TRUE row 1).
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
// Verifies: SW-REQ-069
// MCDC SW-REQ-069: index_eq_mcp=F, is_mcp_record=T, mcp_index_configured=F => TRUE
//
// With mcp_index_configured=F the MCP record stays in IndexName
// (is_mcp_record=T, index_eq_mcp=F -> antecedent false -> vacuous TRUE row 2).
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
// Verifies: SW-REQ-069
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

// Verifies: SW-REQ-068
// MCDC SW-REQ-068: v3_operator_constructed=F, version_eq_3=F => TRUE
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

// SW-REQ-068:tls_verification_explicit:nominal
func TestElasticsearchPump_TLSConfig_EnvSkipVerify(t *testing.T) {
	t.Setenv("TEST_ES_SSLINSECURESKIPVERIFY", "true")

	conf := &ElasticsearchConf{}
	require.NoError(t, envconfig.Process("TEST_ES", conf))
	require.True(t, conf.SSLInsecureSkipVerify)

	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)

	tlsConf, err := NewTLSConfig(TLSConfig{
		InsecureSkipVerify: conf.SSLInsecureSkipVerify,
	}, logrus.NewEntry(logger).WithField("prefix", "test"))
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	assert.True(t, tlsConf.InsecureSkipVerify)
	assert.Contains(t, logBuffer.String(), "ssl_insecure_skip_verify is set to true")
}

// SW-REQ-068:cert_chain_validated:nominal
func TestElasticsearchPump_GetOperatorPassesTLSConfigFields(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "elasticsearch.go", nil, 0)
	require.NoError(t, err)

	var foundGetOperator bool
	var foundTLSConfigCall bool
	var sawLogger bool
	fields := map[string]string{}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "getOperator" {
			continue
		}
		foundGetOperator = true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			fun, ok := call.Fun.(*ast.Ident)
			if !ok || fun.Name != "NewTLSConfig" {
				return true
			}
			foundTLSConfigCall = true

			if logger, ok := call.Args[1].(*ast.SelectorExpr); ok && logger.Sel.Name == "log" {
				if recv, ok := logger.X.(*ast.Ident); ok && recv.Name == "e" {
					sawLogger = true
				}
			}

			lit, ok := call.Args[0].(*ast.CompositeLit)
			require.True(t, ok, "NewTLSConfig first argument must be a TLSConfig literal")
			typeIdent, ok := lit.Type.(*ast.Ident)
			require.True(t, ok, "NewTLSConfig first argument must name TLSConfig")
			require.Equal(t, "TLSConfig", typeIdent.Name)

			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				require.True(t, ok, "TLSConfig literal must use keyed fields")
				key, ok := kv.Key.(*ast.Ident)
				require.True(t, ok, "TLSConfig key must be an identifier")
				value, ok := kv.Value.(*ast.SelectorExpr)
				require.True(t, ok, "TLSConfig value for %s must be a selector", key.Name)
				base, ok := value.X.(*ast.Ident)
				require.True(t, ok, "TLSConfig value for %s must read from conf", key.Name)
				fields[key.Name] = base.Name + "." + value.Sel.Name
			}
			return true
		})
	}

	require.True(t, foundGetOperator, "ElasticsearchPump.getOperator must exist")
	require.True(t, foundTLSConfigCall, "getOperator must call NewTLSConfig")
	require.True(t, sawLogger, "getOperator must pass e.log to NewTLSConfig")
	assert.Equal(t, "conf.SSLCertFile", fields["CertFile"])
	assert.Equal(t, "conf.SSLKeyFile", fields["KeyFile"])
	assert.Equal(t, "conf.SSLCAFile", fields["CAFile"])
	assert.Equal(t, "conf.SSLInsecureSkipVerify", fields["InsecureSkipVerify"])
}

// Verifies: KI:elasticsearch-api-key-auth-dropped-when-use-ssl
// Reproduces: elasticsearch-api-key-auth-dropped-when-use-ssl
func TestElasticsearchPump_ApiKeyAuthDroppedWhenUseSSL_KI(t *testing.T) {
	src, err := os.ReadFile("elasticsearch.go")
	require.NoError(t, err)
	text := string(src)
	compactText := strings.Join(strings.Fields(text), " ")

	apiKeyAssignment := `httpClient = &http.Client{Transport: &ApiKeyTransport`
	tlsAssignment := `httpClient = &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConf}}`

	apiKeyIdx := strings.Index(compactText, apiKeyAssignment)
	tlsIdx := strings.Index(compactText, tlsAssignment)
	require.NotEqual(t, -1, apiKeyIdx, "expected API key transport assignment in getOperator")
	require.NotEqual(t, -1, tlsIdx, "expected TLS transport assignment in getOperator")
	assert.Less(t, apiKeyIdx, tlsIdx, "API key transport is configured before TLS transport")
	assert.NotContains(t, compactText[tlsIdx:tlsIdx+len(tlsAssignment)], "ApiKeyTransport",
		"KI active: UseSSL replaces the API-key transport instead of wrapping/preserving it")
}

// Verifies: INT-REQ-006
// Verifies: SW-REQ-100
// MCDC INT-REQ-006: mapping_per_implementation=T, record_dispatched_to_backend=T => TRUE
// SW-REQ-100:structured_projection_preserved:nominal
// MCDC SW-REQ-100: es_alias_present=T, es_alias_projected=T => TRUE
//
//mcdc:ignore SW-REQ-100: es_alias_present=T, es_alias_projected=F => FALSE -- getMapping assigns mapping["alias"] directly from record.Alias in the same map literal; with a populated alias and unmodified code, the projection-false violation has no runtime branch to exercise, while the positive and empty-alias rows are witnessed [reviewed: human:buger] [category: defensive]
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

// Verifies: SW-REQ-100
// SW-REQ-100:structured_projection_preserved:boundary
// MCDC SW-REQ-100: es_alias_present=F, es_alias_projected=F => TRUE
func TestGetMapping_AliasProjection_EmptyAlias(t *testing.T) {
	record := analytics.AnalyticsRecord{
		APIID: "api1",
		OrgID: "org1",
	}

	mapping, _ := getMapping(record, false, false, false)

	assert.Contains(t, mapping, "alias")
	assert.Empty(t, mapping["alias"])
}

// Verifies: INT-REQ-006
// MCDC INT-REQ-006: mapping_per_implementation=T, record_dispatched_to_backend=T => TRUE
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

// Verifies: INT-REQ-006
// MCDC INT-REQ-006: mapping_per_implementation=T, record_dispatched_to_backend=T => TRUE
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

// Verifies: SW-REQ-069
func TestGetIndexName_NoRolling(t *testing.T) {
	conf := &ElasticsearchConf{
		IndexName: "tyk_analytics",
	}
	assert.Equal(t, "tyk_analytics", getIndexName(conf))
}

// Verifies: SW-REQ-069
// SW-REQ-069:nominal:nominal
func TestGetIndexName_Rolling(t *testing.T) {
	conf := &ElasticsearchConf{
		IndexName:    "tyk_analytics",
		RollingIndex: true,
	}
	today := time.Now().Format("2006.01.02")
	assert.Equal(t, "tyk_analytics-"+today, getIndexName(conf))
}

// Verifies: INT-REQ-006
// MCDC INT-REQ-006: mapping_per_implementation=T, record_dispatched_to_backend=T => TRUE
// SW-REQ-068:backend_decoded_payload_textual:nominal
// SW-REQ-068:backend_decoded_payload_textual:negative
// SW-REQ-068:backend_decoded_payload_textual:review
func TestGetMapping_ExtendedStatistics(t *testing.T) {
	record := analytics.AnalyticsRecord{
		APIID:       "api1",
		RawRequest:  "cmF3LXJlcXVlc3Q=", // base64 "raw-request"
		RawResponse: "cmF3LXJlc3BvbnNl", // base64 "raw-response"
		UserAgent:   "test-agent",
	}

	t.Run("extended stats with base64 decode", func(t *testing.T) {
		mapping, _ := getMapping(record, true, false, true)
		assert.IsType(t, "", mapping["raw_request"])
		assert.IsType(t, "", mapping["raw_response"])
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

// Verifies: KI:elasticsearch-decode-base64-errors-silent-empty
// Reproduces: elasticsearch-decode-base64-errors-silent-empty
func TestGetMapping_DecodeBase64MalformedInput_KI(t *testing.T) {
	record := analytics.AnalyticsRecord{
		RawRequest:  "not-base64-request!",
		RawResponse: "not-base64-response!",
		UserAgent:   "test-agent",
	}

	mapping, _ := getMapping(record, true, false, true)
	assert.Empty(t, mapping["raw_request"])
	assert.Empty(t, mapping["raw_response"])
	assert.Equal(t, "test-agent", mapping["user_agent"])
}
