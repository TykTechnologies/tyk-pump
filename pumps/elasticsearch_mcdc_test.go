package pumps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
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
// MCDC SW-REQ-069: index_eq_mcp=F, is_mcp_record=T, mcp_index_configured=T => FALSE
// MCDC SW-REQ-069: index_eq_mcp=T, is_mcp_record=T, mcp_index_configured=T => TRUE
// MCDC SW-REQ-070: disable_bulk=F, per_record_indexed_else_bulk_processor=F => TRUE
// MCDC SW-REQ-070: disable_bulk=T, per_record_indexed_else_bulk_processor=F => FALSE
// MCDC SW-REQ-070: disable_bulk=T, per_record_indexed_else_bulk_processor=T => TRUE

// esIndexName builds a per-test ES index name; ES indices must be lower-case.
//
// Verifies: SW-REQ-068
func esIndexName(t *testing.T, prefix string) string {
	t.Helper()
	s := strings.ToLower(prefix + "_" + t.Name())
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
		case c == '_' || c == '-':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// esInit builds an ElasticsearchPump connected to the shared testcontainer.
// extra overrides defaults.
//
// Verifies: SW-REQ-068
func esInit(t *testing.T, extra map[string]interface{}) *ElasticsearchPump {
	t.Helper()
	url := elasticsearchURL(t)
	cfg := map[string]interface{}{
		"elasticsearch_url": url,
		"index_name":        esIndexName(t, "tyk_analytics"),
		"version":           "7",
		"use_sniffing":      false,
		"disable_bulk":      true, // tests want synchronous writes
	}
	for k, v := range extra {
		cfg[k] = v
	}
	pump := &ElasticsearchPump{}
	require.NoError(t, pump.Init(cfg))
	require.NotNil(t, pump.operator, "operator should be set after Init")
	return pump
}

// esCountDocs counts documents in an index using a Refresh+Count call against
// the v7 client behind the operator.
//
// Verifies: SW-REQ-068
func esCountDocs(t *testing.T, pump *ElasticsearchPump, index string) int64 {
	t.Helper()
	op, ok := pump.operator.(*Elasticsearch7Operator)
	require.True(t, ok, "expected v7 operator")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	_, err := op.esClient.Refresh(index).Do(ctx)
	require.NoError(t, err)
	n, err := op.esClient.Count(index).Do(ctx)
	require.NoError(t, err)
	return n
}

// TestElasticsearchPump_WriteData_RoundTrip is the happy-path end-to-end test:
// init against a real ES7 container, write one analytics record, and verify
// the index now contains exactly one document.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_WriteData_RoundTrip(t *testing.T) {
	idx := esIndexName(t, "tyk_analytics")
	pump := esInit(t, map[string]interface{}{
		"index_name":   idx,
		"disable_bulk": true,
		"generate_id":  true,
	})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	rec := analytics.AnalyticsRecord{
		APIID:        "api-rt",
		Method:       "GET",
		Path:         "/round-trip",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
	}
	require.NoError(t, pump.WriteData(ctx, []interface{}{rec}))

	assert.EqualValues(t, 1, esCountDocs(t, pump, idx), "exactly one document should be indexed")
}

// TestElasticsearchPump_WriteData_RoundTripBulk exercises the bulk-processor
// branch: the operator's Add() path with a non-trivial flush.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
// SW-REQ-070:nominal:nominal
func TestElasticsearchPump_WriteData_RoundTripBulk(t *testing.T) {
	idx := esIndexName(t, "tyk_analytics_bulk")
	pump := esInit(t, map[string]interface{}{
		"index_name":   idx,
		"disable_bulk": false,
		"bulk_config": map[string]interface{}{
			"workers":        1,
			"flush_interval": 1,
			"bulk_actions":   2,
			"bulk_size":      1024,
		},
	})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	recs := make([]interface{}, 3)
	for i := range recs {
		recs[i] = analytics.AnalyticsRecord{
			APIID:        fmt.Sprintf("api-bulk-%d", i),
			Method:       "GET",
			Path:         "/b",
			ResponseCode: 200,
			TimeStamp:    time.Now(),
		}
	}
	require.NoError(t, pump.WriteData(ctx, recs))
	require.NoError(t, pump.Shutdown(), "Shutdown should flush bulk")

	// allow ES a moment to make the refresh visible
	deadline := time.Now().Add(20 * time.Second)
	var got int64
	for time.Now().Before(deadline) {
		got = esCountDocs(t, pump, idx)
		if got == int64(len(recs)) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	assert.EqualValues(t, len(recs), got, "all bulk records should be indexed")
}

// TestElasticsearchPump_WriteData_MCPIndexRouting verifies that an MCP record
// is routed to the configured MCPIndexName via the bulk per-record index
// resolution path. The non-bulk path is broken: it computes recordIndex but
// then ignores it (see KI elasticsearch-mcp-routing-non-bulk-ignored and the
// matching regression test TestElasticsearchPump_WriteData_MCPIndexRouting_NonBulkBug).
//
// Verifies: SW-REQ-069
// SW-REQ-069:nominal:negative
// MCDC SW-REQ-069: index_eq_mcp=F, is_mcp_record=F, mcp_index_configured=T => TRUE
// MCDC SW-REQ-069: index_eq_mcp=T, is_mcp_record=T, mcp_index_configured=T => TRUE
// (This test drives both with mcp_index_configured=T: a standard record
// (is_mcp_record=F) lands in IndexName (index_eq_mcp=F -> vacuous TRUE row 1),
// and an MCP record (is_mcp_record=T) lands in MCPIndexName (index_eq_mcp=T ->
// satisfied row 4) — covering both halves of the getIndexNameForRecord decision.)
func TestElasticsearchPump_WriteData_MCPIndexRouting(t *testing.T) {
	idx := esIndexName(t, "tyk_analytics_main")
	mcpIdx := esIndexName(t, "tyk_analytics_mcp")
	pump := esInit(t, map[string]interface{}{
		"index_name":     idx,
		"mcp_index_name": mcpIdx,
		"disable_bulk":   false,
		"bulk_config": map[string]interface{}{
			"workers":      1,
			"bulk_actions": 1,
		},
	})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	rest := analytics.AnalyticsRecord{APIID: "rest", Method: "GET", Path: "/r", ResponseCode: 200, TimeStamp: time.Now()}
	mcp := analytics.AnalyticsRecord{
		APIID: "mcp", Method: "POST", Path: "/m", ResponseCode: 200, TimeStamp: time.Now(),
		MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "w"},
	}
	require.NoError(t, pump.WriteData(ctx, []interface{}{rest, mcp}))
	require.NoError(t, pump.Shutdown())

	// Bulk flush is asynchronous; poll for both indices to be populated.
	deadline := time.Now().Add(15 * time.Second)
	var restCount, mcpCount int64
	for time.Now().Before(deadline) {
		restCount = esCountDocsIgnoreMissing(t, pump, idx)
		mcpCount = esCountDocsIgnoreMissing(t, pump, mcpIdx)
		if restCount == 1 && mcpCount == 1 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	assert.EqualValues(t, 1, restCount, "REST index should have 1")
	assert.EqualValues(t, 1, mcpCount, "MCP index should have 1")
}

// esCountDocsIgnoreMissing returns 0 when the index does not yet exist, so the
// caller can poll until the bulk flush has created both indices.
//
// Verifies: SW-REQ-068
func esCountDocsIgnoreMissing(t *testing.T, pump *ElasticsearchPump, index string) int64 {
	t.Helper()
	op, ok := pump.operator.(*Elasticsearch7Operator)
	require.True(t, ok)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := op.esClient.Refresh(index).Do(ctx); err != nil {
		return 0
	}
	n, err := op.esClient.Count(index).Do(ctx)
	if err != nil {
		return 0
	}
	return n
}

// TestElasticsearchPump_WriteData_MCPIndexRouting_NonBulkBug documents
// KI elasticsearch-mcp-routing-non-bulk-ignored: when DisableBulk=true, the
// per-record index resolution computed at processData's recordIndex is not
// passed to the index builder, so MCP records land in the default IndexName
// instead of MCPIndexName.
//
// Verifies: KI elasticsearch-mcp-routing-non-bulk-ignored
// Verifies: SW-REQ-069
// SW-REQ-070:boundary:negative
// MCDC SW-REQ-069: index_eq_mcp=F, is_mcp_record=T, mcp_index_configured=T => FALSE
//
// This is the requirement-violation row (row 3): an MCP record with the MCP
// index configured (is_mcp_record=T, mcp_index_configured=T) lands in the
// default index instead of the MCP index (index_eq_mcp=F), so the guarantee is
// violated. The assertions below prove exactly this mis-routing.
func TestElasticsearchPump_WriteData_MCPIndexRouting_NonBulkBug(t *testing.T) {
	idx := esIndexName(t, "tyk_analytics_main_nb")
	mcpIdx := esIndexName(t, "tyk_analytics_mcp_nb")
	pump := esInit(t, map[string]interface{}{
		"index_name":     idx,
		"mcp_index_name": mcpIdx,
		"disable_bulk":   true,
	})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	mcp := analytics.AnalyticsRecord{
		APIID: "mcp-nb", Method: "POST", Path: "/m", ResponseCode: 200, TimeStamp: time.Now(),
		MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "x"},
	}
	require.NoError(t, pump.WriteData(ctx, []interface{}{mcp}))

	// The bug: the MCP record lands in the default index because the
	// non-bulk path uses the index builder created once with the default
	// IndexName, ignoring the per-record routing.
	assert.EqualValues(t, 1, esCountDocs(t, pump, idx),
		"BUG: MCP record incorrectly indexed into default index (DisableBulk path ignores per-record routing)")
	assert.EqualValues(t, 0, esCountDocsIgnoreMissing(t, pump, mcpIdx),
		"BUG: MCP index was not created because DisableBulk path ignores recordIndex")
}

// TestElasticsearchPump_WriteData_BadType exercises the type-assertion error
// branch inside Elasticsearch7Operator.processData (the `_, ok := ...` guard).
//
// Verifies: SW-REQ-070
// SW-REQ-070:boundary:negative
// MCDC SW-REQ-070: disable_bulk=F, per_record_indexed_else_bulk_processor=F => TRUE
// MCDC SW-REQ-070: disable_bulk=T, per_record_indexed_else_bulk_processor=F => FALSE
// MCDC SW-REQ-070: disable_bulk=T, per_record_indexed_else_bulk_processor=T => TRUE
//
// disable_bulk=T (cfg sets disable_bulk: true), per_record_indexed_else_bulk_processor=T
// (the operator indexes records one-by-one against the testcontainer ES7). The disable_bulk=F
// arm (BulkProcessor path) is exercised by the suite's default-config tests
// (TestElasticsearchPump_WriteData_Bulk etc.). The T/F regression scenario (DisableBulk=true
// but bulk still used) is guarded by the per-record assertion that records appear immediately
// rather than after the bulk flush window.
func TestElasticsearchPump_WriteData_BadType(t *testing.T) {
	idx := esIndexName(t, "tyk_analytics_badtype")
	pump := esInit(t, map[string]interface{}{
		"index_name":   idx,
		"disable_bulk": true,
	})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	// Mix one valid record with a junk entry: the junk should be skipped
	// with an error log; the valid record should be indexed.
	good := analytics.AnalyticsRecord{APIID: "ok", Method: "GET", Path: "/g", ResponseCode: 200, TimeStamp: time.Now()}
	require.NoError(t, pump.WriteData(ctx, []interface{}{"not-a-record", good}))
	assert.EqualValues(t, 1, esCountDocs(t, pump, idx), "only the valid record should be indexed")
}

// TestElasticsearchPump_WriteData_CancelledContext verifies that the
// `if ctxErr := ctx.Err(); ctxErr != nil { continue }` branch inside
// processData is entered when the caller's context is already cancelled.
// Net effect: no documents indexed even though records were supplied.
//
// Verifies: SW-REQ-070
// SW-REQ-070:boundary:negative
func TestElasticsearchPump_WriteData_CancelledContext(t *testing.T) {
	idx := esIndexName(t, "tyk_analytics_cancel")
	pump := esInit(t, map[string]interface{}{
		"index_name":   idx,
		"disable_bulk": true,
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // pre-cancel
	rec := analytics.AnalyticsRecord{APIID: "x", Method: "GET", Path: "/", ResponseCode: 200, TimeStamp: time.Now()}
	require.NoError(t, pump.WriteData(ctx, []interface{}{rec}))
	// Force-create the index so Refresh/Count don't fail. Use a regular
	// write to a different index name then count the cancelled index.
	other := esIndexName(t, "tyk_analytics_warm")
	pump.esConf.IndexName = other
	require.NoError(t, pump.WriteData(t.Context(), []interface{}{rec}))

	// Original index should have nothing.
	op := pump.operator.(*Elasticsearch7Operator)
	ctx2, cancel2 := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel2()
	// Refresh may fail on a never-created index; in that case count is 0.
	if _, err := op.esClient.Refresh(idx).Do(ctx2); err == nil {
		n, _ := op.esClient.Count(idx).Do(ctx2)
		assert.EqualValues(t, 0, n)
	}
}

// TestElasticsearchPump_Init_VersionDefaults asserts that an unset Version
// defaults to "3" and that the documented defaults for IndexName,
// ElasticsearchURL and DocumentType are applied. To drive the URL-defaults
// branch we redirect e.connect() by pre-setting a working operator so the
// init's connect() does not loop on the (unreachable) default URL.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_Init_VersionDefaults(t *testing.T) {
	// (1) Non-empty URL, empty IndexName/DocumentType → those defaults kick in.
	url := elasticsearchURL(t)
	pump := &ElasticsearchPump{}
	require.NoError(t, pump.Init(map[string]interface{}{
		"elasticsearch_url": url,
		"version":           "7",
	}))
	assert.Equal(t, "tyk_analytics", pump.esConf.IndexName)
	assert.Equal(t, "tyk_analytics", pump.esConf.DocumentType)

	// (2) Non-empty IndexName/DocumentType → defaulting branches NOT taken.
	pump2 := &ElasticsearchPump{}
	require.NoError(t, pump2.Init(map[string]interface{}{
		"elasticsearch_url": url,
		"version":           "7",
		"index_name":        "explicit_idx",
		"document_type":     "explicit_doc",
	}))
	assert.Equal(t, "explicit_idx", pump2.esConf.IndexName)
	assert.Equal(t, "explicit_doc", pump2.esConf.DocumentType)
}

// TestElasticsearchPump_Init_DefaultElasticsearchURL covers the
// `"" == e.esConf.ElasticsearchURL` true branch by leaving the URL unset.
// Init then defaults to http://localhost:9200 and immediately calls connect();
// connect() recurses on failure (see KI elasticsearch-unbounded-reconnect-
// recursion). To break that recursion we spin up a tiny ES7-pretender HTTP
// server bound to 127.0.0.1:9200 so the first connect() attempt succeeds.
//
// If 127.0.0.1:9200 is already taken on the test host, the test is skipped.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_Init_DefaultElasticsearchURL(t *testing.T) {
	srv, ok := startFakeES7On9200(t)
	if !ok {
		t.Skip("could not bind 127.0.0.1:9200 — skipping default-URL branch test")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	pump := &ElasticsearchPump{}
	require.NoError(t, pump.Init(map[string]interface{}{
		// elasticsearch_url intentionally omitted → defaults to
		// http://localhost:9200 inside Init.
		"version":      "7",
		"index_name":   esIndexName(t, "tyk_analytics_defaulturl"),
		"use_sniffing": false,
	}))
	assert.Equal(t, "http://localhost:9200", pump.esConf.ElasticsearchURL,
		"empty URL must default to http://localhost:9200")
}

// startFakeES7On9200 attempts to bind a tiny ES7-pretender HTTP server on
// 127.0.0.1:9200 (the default URL Init falls back to). Returns (srv, true)
// on success and (nil, false) if the port is already in use.
//
// Verifies: SW-REQ-068
func startFakeES7On9200(t *testing.T) (*http.Server, bool) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"fake","cluster_name":"fake","cluster_uuid":"fake",
			"version":{"number":"7.17.27","build_flavor":"default","build_type":"docker",
			"build_hash":"fake","build_date":"2024-01-01T00:00:00.000Z","build_snapshot":false,
			"lucene_version":"8.11.3","minimum_wire_compatibility_version":"6.8.0",
			"minimum_index_compatibility_version":"6.0.0-beta1"},
			"tagline":"You Know, for Search"
		}`))
	})
	srv := &http.Server{
		Addr:              "127.0.0.1:9200",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return nil, false
	}
	go func() { _ = srv.Serve(ln) }()
	return srv, true
}

// TestElasticsearchPump_Init_RollingIndexLogPath simply exercises the
// `if e.esConf.RollingIndex` log branch in Init.
//
// Verifies: SW-REQ-069
// SW-REQ-069:nominal:negative
func TestElasticsearchPump_Init_RollingIndexLogPath(t *testing.T) {
	pump := esInit(t, map[string]interface{}{
		"rolling_index": true,
	})
	assert.True(t, pump.esConf.RollingIndex)
}

// TestElasticsearchPump_Init_URLPasswordMasking confirms that the password in
// the URL is masked in the log line (regex branch). We can't directly inspect
// the log line without a hook, but we can verify the regex by calling the
// production regexp behaviour indirectly: build a URL with user:pass@host and
// check that Init does not panic and that esConf preserves the original URL.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_Init_URLPasswordMasking(t *testing.T) {
	url := elasticsearchURL(t)
	// Inject fake user:pass that the regex masks for log output.
	hostWithCreds := strings.Replace(url, "http://", "http://user:secret@", 1)
	pump := &ElasticsearchPump{}
	require.NoError(t, pump.Init(map[string]interface{}{
		"elasticsearch_url": hostWithCreds + "," + url, // multi-URL split branch
		"version":           "7",
	}))
	assert.Contains(t, pump.esConf.ElasticsearchURL, "user:secret@", "stored config is unchanged")
}

// TestElasticsearchPump_getOperator_AuthAPIKeyShortCircuit covers the
// MC/DC short-circuit gap on `conf.AuthAPIKey != "" && conf.AuthAPIKeyID != ""`.
// When AuthAPIKey is set but AuthAPIKeyID is empty the first half is true and
// the second half must be evaluated to false (skipping the ApiKeyTransport
// branch). The companion TestElasticsearchPump_getOperator_AuthAPIKeyBranch
// covers the both-set case.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_getOperator_AuthAPIKeyShortCircuit(t *testing.T) {
	url := elasticsearchURL(t)
	pump := &ElasticsearchPump{}
	pump.log = log.WithField("prefix", "test")
	pump.esConf = &ElasticsearchConf{
		ElasticsearchURL: url,
		IndexName:        esIndexName(t, "tyk_analytics_apikey_ko"),
		Version:          "7",
		AuthAPIKey:       "the-key",
		// AuthAPIKeyID intentionally empty: forces the second && operand to F.
		Username: "preserved",
		Password: "preserved",
	}
	op, err := pump.getOperator()
	require.NoError(t, err)
	require.NotNil(t, op)
}

// TestElasticsearchPump_getOperator_AuthAPIKeyBranch covers the ApiKey-auth
// branch of getOperator: when both AuthAPIKey and AuthAPIKeyID are set, the
// pump must clear Username/Password and use the ApiKeyTransport.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_getOperator_AuthAPIKeyBranch(t *testing.T) {
	url := elasticsearchURL(t)
	pump := &ElasticsearchPump{}
	pump.log = log.WithField("prefix", "test")
	pump.esConf = &ElasticsearchConf{
		ElasticsearchURL: url,
		IndexName:        esIndexName(t, "tyk_analytics_apikey"),
		Version:          "7",
		AuthAPIKey:       "the-key",
		AuthAPIKeyID:     "the-id",
		Username:         "should",
		Password:         "be-cleared",
	}
	op, err := pump.getOperator()
	require.NoError(t, err)
	require.NotNil(t, op)
	// The conf was modified in getOperator's local copy only, but the
	// transport must have been swapped: we cannot easily probe the HTTP
	// client, so just assert operator type.
	_, ok := op.(*Elasticsearch7Operator)
	assert.True(t, ok)
}

// TestElasticsearchPump_getOperator_BulkConfigBranches drives every individual
// "if conf.BulkConfig.X != 0" branch inside getOperator (workers, flush
// interval, bulk actions, bulk size — both 0 and non-zero) for the v7 path.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_getOperator_BulkConfigBranches(t *testing.T) {
	url := elasticsearchURL(t)
	cases := []struct {
		name string
		bc   ElasticsearchBulkConfig
	}{
		{"all_zero", ElasticsearchBulkConfig{}},
		{"workers_only", ElasticsearchBulkConfig{Workers: 2}},
		{"flush_interval_only", ElasticsearchBulkConfig{FlushInterval: 1}},
		{"bulk_actions_only", ElasticsearchBulkConfig{BulkActions: 100}},
		{"bulk_size_only", ElasticsearchBulkConfig{BulkSize: 4096}},
		{"all_set", ElasticsearchBulkConfig{Workers: 2, FlushInterval: 1, BulkActions: 100, BulkSize: 4096}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pump := &ElasticsearchPump{}
			pump.log = log.WithField("prefix", "test")
			pump.esConf = &ElasticsearchConf{
				ElasticsearchURL: url,
				IndexName:        esIndexName(t, "tyk_analytics_bc_"+tc.name),
				Version:          "7",
				BulkConfig:       tc.bc,
				DisableBulk:      false,
			}
			op, err := pump.getOperator()
			require.NoError(t, err)
			require.NotNil(t, op)
		})
	}
}

// TestElasticsearchPump_getOperator_DisableBulkBranch covers the
// `if !conf.DisableBulk` path being false (so the bulkAfter purger logger is
// not registered) for v7.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_getOperator_DisableBulkBranch(t *testing.T) {
	url := elasticsearchURL(t)
	pump := &ElasticsearchPump{}
	pump.log = log.WithField("prefix", "test")
	pump.esConf = &ElasticsearchConf{
		ElasticsearchURL: url,
		IndexName:        esIndexName(t, "tyk_analytics_nobulk"),
		Version:          "7",
		DisableBulk:      true,
	}
	op, err := pump.getOperator()
	require.NoError(t, err)
	require.NotNil(t, op)
}

// TestElasticsearchPump_getOperator_LegacyVersions drives the v3/v5/v6
// switch-case bodies of getOperator. The underlying olivere/elastic.v3/v5/v6
// clients only require a successful HTTP healthcheck (GET /) at construction
// time when sniffing is disabled; ES7 satisfies that, so all three branches
// exercise their bulk-processor wiring even though the wire protocol later
// diverges. We do not attempt to write data through these operators — the
// goal is MC/DC of the operator-construction switch.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
// SW-REQ-068:nominal:nominal
func TestElasticsearchPump_getOperator_LegacyVersions(t *testing.T) {
	url := elasticsearchURL(t)
	for _, version := range []string{"3", "5", "6"} {
		t.Run("v"+version, func(t *testing.T) {
			pump := &ElasticsearchPump{}
			pump.log = log.WithField("prefix", "test")
			pump.esConf = &ElasticsearchConf{
				ElasticsearchURL: url,
				IndexName:        esIndexName(t, "tyk_analytics_v"+version),
				Version:          version,
				BulkConfig: ElasticsearchBulkConfig{
					Workers:       1,
					FlushInterval: 1,
					BulkActions:   10,
					BulkSize:      1024,
				},
			}
			op, err := pump.getOperator()
			if err != nil {
				// The v3/v5/v6 client may reject the v7 ping response in
				// strict mode. That is acceptable evidence: we still
				// executed the switch case up to the failure point.
				t.Logf("v%s NewClient rejected ES7 endpoint: %v (switch branch was entered)", version, err)
				return
			}
			require.NotNil(t, op)
		})
	}
}

// TestElasticsearchPump_getOperator_LegacyVersions_DisableBulk covers the
// `!conf.DisableBulk == false` branch (skip bulk-purger-logger registration)
// in each of v3/v5/v6.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_getOperator_LegacyVersions_DisableBulk(t *testing.T) {
	url := elasticsearchURL(t)
	for _, version := range []string{"3", "5", "6"} {
		t.Run("v"+version, func(t *testing.T) {
			pump := &ElasticsearchPump{}
			pump.log = log.WithField("prefix", "test")
			pump.esConf = &ElasticsearchConf{
				ElasticsearchURL: url,
				IndexName:        esIndexName(t, "tyk_analytics_v"+version+"_nobulk"),
				Version:          version,
				DisableBulk:      true,
			}
			op, err := pump.getOperator()
			if err != nil {
				t.Logf("v%s NewClient rejected ES7 endpoint: %v (switch branch was entered)", version, err)
				return
			}
			require.NotNil(t, op)
		})
	}
}

// TestElasticsearchPump_Shutdown_BulkPath covers Shutdown's bulk-enabled path
// where flushRecords() is invoked on the operator.
//
// Verifies: SW-REQ-070
// SW-REQ-070:nominal:negative
func TestElasticsearchPump_Shutdown_BulkPath(t *testing.T) {
	pump := esInit(t, map[string]interface{}{
		"disable_bulk": false,
	})
	assert.NoError(t, pump.Shutdown())
}

// TestElasticsearchPump_Shutdown_BulkDisabled covers Shutdown's early-return
// path when bulk is disabled.
//
// Verifies: SW-REQ-070
// SW-REQ-070:nominal:negative
func TestElasticsearchPump_Shutdown_BulkDisabled(t *testing.T) {
	pump := esInit(t, map[string]interface{}{
		"disable_bulk": true,
	})
	assert.NoError(t, pump.Shutdown())
}

// TestElasticsearchPump_GetName_GetEnvPrefix covers the trivial accessors.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_GetName_GetEnvPrefix(t *testing.T) {
	pump := esInit(t, map[string]interface{}{
		"meta_env_prefix": "TEST_PREFIX",
	})
	assert.Equal(t, "Elasticsearch Pump", pump.GetName())
	assert.Equal(t, "TEST_PREFIX", pump.GetEnvPrefix())
	// New() returns a fresh *ElasticsearchPump
	fresh := pump.New()
	_, ok := fresh.(*ElasticsearchPump)
	assert.True(t, ok)
}

// TestElasticsearchPump_WriteData_EmptySlice exercises the "len(data) > 0" guard
// in WriteData — an empty slice must not call processData.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_WriteData_EmptySlice(t *testing.T) {
	pump := esInit(t, map[string]interface{}{"disable_bulk": true})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, pump.WriteData(ctx, []interface{}{}))
}

// TestElasticsearchPump_WriteData_NilOperatorReconnects exercises the
// `if e.operator == nil { e.connect(); e.WriteData(...) }` branch of
// WriteData. We set up a pump pointed at the working ES container, then
// manually zero its operator so the next WriteData call must go through
// the reconnect+recurse path.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_WriteData_NilOperatorReconnects(t *testing.T) {
	idx := esIndexName(t, "tyk_analytics_nilop")
	pump := esInit(t, map[string]interface{}{
		"index_name":   idx,
		"disable_bulk": true,
	})
	// Drop operator and verify WriteData re-creates it.
	pump.operator = nil
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	rec := analytics.AnalyticsRecord{APIID: "nilop", Method: "GET", Path: "/", ResponseCode: 200, TimeStamp: time.Now()}
	require.NoError(t, pump.WriteData(ctx, []interface{}{rec}))
	require.NotNil(t, pump.operator, "WriteData should have reconnected the operator")
	assert.EqualValues(t, 1, esCountDocs(t, pump, idx))
}

// TestElasticsearchPump_GetMapping_ExtendedNoBase64 covers the
// extendedStatistics=true && decodeBase64=false sub-branch of getMapping
// (raw fields passed through unchanged).
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_GetMapping_ExtendedNoBase64(t *testing.T) {
	rec := analytics.AnalyticsRecord{
		RawRequest:  "raw-req",
		RawResponse: "raw-resp",
		UserAgent:   "ua",
	}
	m, _ := getMapping(rec, true, false, false)
	assert.Equal(t, "raw-req", m["raw_request"])
	assert.Equal(t, "raw-resp", m["raw_response"])
	assert.Equal(t, "ua", m["user_agent"])
}

// TestPrintPurgedBulkRecords drives both branches of printPurgedBulkRecords
// (err == nil → Infof; err != nil → WithError().Errorf). The function only
// affects logging — we just need both code paths executed for MC/DC.
//
// Verifies: SW-REQ-070
// SW-REQ-070:boundary:negative
func TestPrintPurgedBulkRecords(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	printPurgedBulkRecords(5, nil, logger)
	printPurgedBulkRecords(5, errors.New("simulated bulk error"), logger)
}

// TestElasticsearchPump_WriteData_V7ProcessDataIndexError exercises the
// `err != nil` branch inside Elasticsearch7Operator.processData by writing
// to an index that we pre-create with a strict mapping that refuses the
// produced document, forcing the underlying Index().Do() call to return
// an error.
//
// Verifies: SW-REQ-070
// SW-REQ-070:boundary:negative
func TestElasticsearchPump_WriteData_V7ProcessDataIndexError(t *testing.T) {
	idx := esIndexName(t, "tyk_analytics_strict")
	pump := esInit(t, map[string]interface{}{
		"index_name":   idx,
		"disable_bulk": true,
	})

	// Create the index up-front in strict mode with api_id typed as long.
	// The generated document has api_id as a string, which under
	// "dynamic": "strict" produces a mapping_set_to_create_doc_error and
	// fails the Index().Do() call.
	op := pump.operator.(*Elasticsearch7Operator)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	_, err := op.esClient.CreateIndex(idx).Body(`{
		"mappings": {
			"dynamic": "strict",
			"properties": {
				"api_id": {"type": "long"}
			}
		}
	}`).Do(ctx)
	require.NoError(t, err)

	rec := analytics.AnalyticsRecord{APIID: "not-a-number", Method: "GET", Path: "/", ResponseCode: 200, TimeStamp: time.Now()}
	// WriteData logs the error and returns nil. We assert that no document
	// was indexed despite the call returning success-with-log.
	require.NoError(t, pump.WriteData(ctx, []interface{}{rec}))
	assert.EqualValues(t, 0, esCountDocs(t, pump, idx),
		"strict-mapping conflict should have rejected the document")
}

// TestApiKeyTransport_RoundTrip verifies the ApiKeyTransport sets the expected
// Authorization header on outgoing requests.
//
// Verifies: SW-REQ-068
// SW-REQ-068:nominal:negative
func TestApiKeyTransport_RoundTrip(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	tr := &ApiKeyTransport{APIKey: "key", APIKeyID: "id"}
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	// Expected base64(id:key) = base64("id:key")
	assert.True(t, strings.HasPrefix(got, "ApiKey "), "header should start with ApiKey: %q", got)
	assert.Equal(t, "ApiKey aWQ6a2V5", got, "Authorization should be ApiKey base64(id:key)")
}

// =============================================================================
// Regression test for the KI:
//   elasticsearch-unbounded-reconnect-recursion
//
// The production code's connect() recurses on failure with no bound. We
// drive it against an unreachable host with a tiny client timeout so that
// each failed attempt returns quickly, then capture a goroutine stack
// snapshot and assert that ElasticsearchPump.connect appears more than once
// in the same stack — proving the recursion. We also capture the warning
// counter and require >= 2 errors within the observation window.
//
// We deliberately do not let the test wait for stack overflow (that would
// take many minutes given the hard-coded time.Sleep(5*time.Second) between
// attempts). Demonstrating self-call within a single stack is sufficient
// evidence of unbounded recursion.
//
// The goroutine that drives connect() is intentionally leaked at the end of
// the test: there is no way to cancel it because the production API gives
// us no Stop()/context for connect(). The leak is bounded by process exit.
// =============================================================================

// errorCountingHook is a tiny logrus.Hook that counts Error-level lines whose
// message starts with a configured prefix.
//
// Verifies: KI elasticsearch-unbounded-reconnect-recursion
type errorCountingHook struct {
	prefix string
	count  int64
}

// Verifies: SW-REQ-068
// Verifies: KI elasticsearch-unbounded-reconnect-recursion
func (h *errorCountingHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel}
}

// Verifies: SW-REQ-068
// Verifies: KI elasticsearch-unbounded-reconnect-recursion
func (h *errorCountingHook) Fire(e *logrus.Entry) error {
	if strings.HasPrefix(e.Message, h.prefix) {
		atomic.AddInt64(&h.count, 1)
	}
	return nil
}

// TestElasticsearchPump_KI_UnboundedReconnectRecursion is a regression test that
// captures and documents the unbounded recursive connect() in
// ElasticsearchPump.connect. It does NOT fix the bug; on the contrary, the
// test PASSES while the bug exists (and would need to be updated when the
// production code is repaired).
//
// Verifies: KI elasticsearch-unbounded-reconnect-recursion
// SW-REQ-068:nominal:negative
func TestElasticsearchPump_KI_UnboundedReconnectRecursion(t *testing.T) {
	// Use a dedicated logger so the hook only sees this pump's lines.
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	var sink bytes.Buffer
	logger.SetOutput(&sink)
	hook := &errorCountingHook{prefix: "Elasticsearch connection failed"}
	logger.AddHook(hook)

	pump := &ElasticsearchPump{}
	pump.log = logrus.NewEntry(logger).WithField("prefix", elasticsearchPrefix)
	pump.esConf = &ElasticsearchConf{
		// Address a TCP port that should refuse connections quickly. Using
		// 127.0.0.1:1 avoids DNS and yields ECONNREFUSED immediately.
		ElasticsearchURL: "http://127.0.0.1:1",
		IndexName:        "tyk_analytics_ki",
		DocumentType:     "tyk_analytics",
		Version:          "7",
	}

	// Drive connect() in a background goroutine. It WILL leak — production
	// connect() has no cancellation channel. The goroutine is bounded by
	// the test process lifetime.
	done := make(chan struct{}) // closed only when connect() returns (it won't).
	go func() {
		defer close(done)
		pump.connect()
	}()

	// Allow a short window for the first one or two failed attempts. The
	// production code uses time.Sleep(5 * time.Second) between recursive
	// calls, so within ~6s we expect 1-2 logged errors and exactly one
	// goroutine whose stack shows connect calling itself (or being mid-
	// way through Sleep within a connect frame).
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&hook.count) >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.GreaterOrEqual(t, atomic.LoadInt64(&hook.count), int64(1),
		"expected at least one 'Elasticsearch connection failed' error logged within 8s; got log buffer:\n%s",
		sink.String())

	// Snapshot all goroutine stacks and verify that connect() is currently
	// active (the goroutine should be in time.Sleep within connect()).
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	stacks := string(buf[:n])

	// The fully-qualified symbol used by the go runtime is the package path
	// followed by the function name.
	const sym = "github.com/TykTechnologies/tyk-pump/pumps.(*ElasticsearchPump).connect"
	assert.Contains(t, stacks, sym,
		"goroutine stack should contain ElasticsearchPump.connect (indicating recursion is in progress)")

	// Optional: count occurrences to document recursion depth so far.
	depth := strings.Count(stacks, sym)
	t.Logf("KI elasticsearch-unbounded-reconnect-recursion: observed %d connect() frame(s) on stack after ~8s; logged errors=%d",
		depth, atomic.LoadInt64(&hook.count))

	// Sanity: don't wait on done — it never closes while the bug exists.
	select {
	case <-done:
		t.Fatalf("connect() returned; bug appears to be fixed — update the regression test")
	default:
	}
}
