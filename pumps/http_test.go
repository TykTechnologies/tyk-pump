package pumps

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type httpCollectedRequest struct {
	header  http.Header
	records []map[string]interface{}
}

// httpCollector records every request it receives and answers with the next status in
// statuses, then with 200 once the list is exhausted.
type httpCollector struct {
	statuses []int
	requests []httpCollectedRequest
	delay    time.Duration
	mu       sync.Mutex
}

func (c *httpCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.delay > 0 {
		select {
		case <-time.After(c.delay):
		case <-r.Context().Done():
			return
		}
	}

	var reader io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "bad gzip", http.StatusBadRequest)
			return
		}
		reader = zr
	}
	var records []map[string]interface{}
	if err := json.NewDecoder(reader).Decode(&records); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	c.requests = append(c.requests, httpCollectedRequest{header: r.Header.Clone(), records: records})
	status := http.StatusOK
	if len(c.statuses) > 0 {
		status = c.statuses[0]
		c.statuses = c.statuses[1:]
	}
	c.mu.Unlock()

	w.WriteHeader(status)
}

func (c *httpCollector) received() []httpCollectedRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]httpCollectedRequest(nil), c.requests...)
}

func (c *httpCollector) recordCount() int {
	n := 0
	for _, r := range c.received() {
		n += len(r.records)
	}
	return n
}

func newHTTPTestPump(t *testing.T, conf map[string]interface{}) *HTTPPump {
	t.Helper()
	p, ok := (&HTTPPump{}).New().(*HTTPPump)
	require.True(t, ok)
	require.NoError(t, p.Init(conf))
	return p
}

func httpTestRecords(n int) []interface{} {
	data := make([]interface{}, n)
	for i := range data {
		data[i] = analytics.AnalyticsRecord{
			APIID:        "api1",
			OrgID:        "org1",
			ResponseCode: 200,
			TimeStamp:    time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC),
			MCPStats: analytics.MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "tools/call",
				PrimitiveType: "tool",
				PrimitiveName: "get_weather",
			},
		}
	}
	return data
}

func TestHTTPPump_Init(t *testing.T) {
	t.Run("url is required", func(t *testing.T) {
		p := &HTTPPump{}
		assert.Error(t, p.Init(map[string]interface{}{}))
	})

	t.Run("url scheme must be http or https", func(t *testing.T) {
		p := &HTTPPump{}
		assert.Error(t, p.Init(map[string]interface{}{"url": "ftp://example.com"}))
	})

	t.Run("defaults", func(t *testing.T) {
		p := newHTTPTestPump(t, map[string]interface{}{"url": "http://localhost:9999/ingest"})
		assert.Equal(t, httpDefaultBatchSize, p.conf.BatchSize)
		assert.Equal(t, httpDefaultMaxBodyBytes, p.conf.MaxBodyBytes)
		assert.Equal(t, "HTTP Pump", p.GetName())
	})

	t.Run("registered as http", func(t *testing.T) {
		pmp, err := GetPumpByName("http")
		require.NoError(t, err)
		assert.IsType(t, &HTTPPump{}, pmp)
	})
}

func TestHTTPPump_SendsFullRecordsWithHeaders(t *testing.T) {
	c := &httpCollector{}
	srv := httptest.NewServer(c)
	defer srv.Close()

	p := newHTTPTestPump(t, map[string]interface{}{
		"url":     srv.URL,
		"headers": map[string]string{"Authorization": "Bearer secret", "X-Custom": "yes"},
	})

	record := analytics.AnalyticsRecord{
		APIID:        "api1",
		OrgID:        "org1",
		APIKey:       "hashed-key",
		Alias:        "studio:app:7",
		ResponseCode: 200,
		Tags:         []string{"mcp-session-id-abc"},
		Latency:      analytics.Latency{Total: 12, Upstream: 10},
		MCPStats: analytics.MCPStats{
			IsMCP:                    true,
			JSONRPCMethod:            "tools/call",
			PrimitiveType:            "tool",
			PrimitiveName:            "get_weather",
			JSONRPCErrorCode:         -32603,
			EffectiveProtocolVersion: "2025-06-18",
		},
	}
	require.NoError(t, p.WriteData(context.Background(), []interface{}{record}))

	reqs := c.received()
	require.Len(t, reqs, 1)
	assert.Equal(t, "Bearer secret", reqs[0].header.Get("Authorization"))
	assert.Equal(t, "yes", reqs[0].header.Get("X-Custom"))
	assert.Equal(t, "application/json", reqs[0].header.Get("Content-Type"))
	assert.Empty(t, reqs[0].header.Get("Content-Encoding"))

	require.Len(t, reqs[0].records, 1)
	got := reqs[0].records[0]
	assert.Equal(t, "api1", got["api_id"])
	assert.Equal(t, "org1", got["org_id"])
	assert.Equal(t, "hashed-key", got["api_key"])
	assert.Equal(t, "studio:app:7", got["alias"])
	assert.Equal(t, float64(200), got["response_code"])
	assert.Equal(t, []interface{}{"mcp-session-id-abc"}, got["tags"])
	assert.Equal(t, map[string]interface{}{"total": float64(12), "upstream": float64(10), "gateway": float64(0)}, got["latency"])

	mcp, ok := got["mcp_stats"].(map[string]interface{})
	require.True(t, ok, "mcp_stats missing from record")
	assert.Equal(t, true, mcp["is_mcp"])
	assert.Equal(t, "tools/call", mcp["jsonrpc_method"])
	assert.Equal(t, "tool", mcp["primitive_type"])
	assert.Equal(t, "get_weather", mcp["primitive_name"])
	assert.Equal(t, float64(-32603), mcp["jsonrpc_error_code"])
	assert.Equal(t, "2025-06-18", mcp["effective_protocol_version"])
}

func TestHTTPPump_Gzip(t *testing.T) {
	c := &httpCollector{}
	srv := httptest.NewServer(c)
	defer srv.Close()

	p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "gzip": true})
	require.NoError(t, p.WriteData(context.Background(), httpTestRecords(3)))

	reqs := c.received()
	require.Len(t, reqs, 1)
	assert.Equal(t, "gzip", reqs[0].header.Get("Content-Encoding"))
	assert.Len(t, reqs[0].records, 3)
}

func TestHTTPPump_BatchSize(t *testing.T) {
	c := &httpCollector{}
	srv := httptest.NewServer(c)
	defer srv.Close()

	p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "batch_size": 2})
	require.NoError(t, p.WriteData(context.Background(), httpTestRecords(5)))

	reqs := c.received()
	require.Len(t, reqs, 3)
	assert.Len(t, reqs[0].records, 2)
	assert.Len(t, reqs[1].records, 2)
	assert.Len(t, reqs[2].records, 1)
}

func TestHTTPPump_MaxBodyBytes(t *testing.T) {
	c := &httpCollector{}
	srv := httptest.NewServer(c)
	defer srv.Close()

	records := httpTestRecords(5)
	one, err := json.Marshal(records[0])
	require.NoError(t, err)

	// room for two records ("[" + rec + "," + rec + "]") but not three
	limit := 2*len(one) + 3
	p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "max_body_bytes": limit})
	require.NoError(t, p.WriteData(context.Background(), records))

	reqs := c.received()
	require.Len(t, reqs, 3)
	assert.Len(t, reqs[0].records, 2)
	assert.Len(t, reqs[1].records, 2)
	assert.Len(t, reqs[2].records, 1)

	t.Run("record larger than the limit is dropped, the rest is sent", func(t *testing.T) {
		c := &httpCollector{}
		srv := httptest.NewServer(c)
		defer srv.Close()

		big, ok := httpTestRecords(1)[0].(analytics.AnalyticsRecord)
		require.True(t, ok)
		big.RawRequest = strings.Repeat("x", 4*len(one))

		p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "max_body_bytes": limit})
		err := p.WriteData(context.Background(), []interface{}{records[0], big, records[1]})
		assert.ErrorContains(t, err, "dropped 1 records")
		assert.Equal(t, 2, c.recordCount())
	})
}

func TestHTTPPump_StatusHandling(t *testing.T) {
	tcs := []struct {
		name         string
		statuses     []int
		maxRetries   int
		wantRequests int
		wantErr      bool
	}{
		{name: "200 is success", statuses: []int{200}, wantRequests: 1},
		{name: "202 is success", statuses: []int{202}, wantRequests: 1},
		{name: "204 is success", statuses: []int{204}, wantRequests: 1},
		{name: "400 is permanent", statuses: []int{400}, maxRetries: 3, wantRequests: 1, wantErr: true},
		{name: "401 is permanent", statuses: []int{401}, maxRetries: 3, wantRequests: 1, wantErr: true},
		{name: "413 is permanent", statuses: []int{413}, maxRetries: 3, wantRequests: 1, wantErr: true},
		{name: "503 is retried until success", statuses: []int{503, 503}, maxRetries: 3, wantRequests: 3},
		{name: "429 is retried until success", statuses: []int{429}, maxRetries: 1, wantRequests: 2},
		{name: "500 gives up after max_retries", statuses: []int{500, 500, 500}, maxRetries: 1, wantRequests: 2, wantErr: true},
		{name: "no retries by default", statuses: []int{502}, wantRequests: 1, wantErr: true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &httpCollector{statuses: append([]int(nil), tc.statuses...)}
			srv := httptest.NewServer(c)
			defer srv.Close()

			p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "max_retries": tc.maxRetries})
			err := p.WriteData(context.Background(), httpTestRecords(1))
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Len(t, c.received(), tc.wantRequests)
		})
	}
}

func TestHTTPPump_FailedBatchDoesNotStopTheRest(t *testing.T) {
	c := &httpCollector{statuses: []int{400, 200}}
	srv := httptest.NewServer(c)
	defer srv.Close()

	p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "batch_size": 1, "max_retries": 2})
	err := p.WriteData(context.Background(), httpTestRecords(2))
	assert.Error(t, err)
	assert.Len(t, c.received(), 2)
}

func TestHTTPPump_ConnectionRefusedIsRetried(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL
	srv.Close()

	p := newHTTPTestPump(t, map[string]interface{}{"url": addr, "max_retries": 1})
	start := time.Now()
	err := p.WriteData(context.Background(), httpTestRecords(1))
	assert.Error(t, err)
	// one backoff interval (~500ms) means the refused dial was retried
	assert.Greater(t, time.Since(start), 200*time.Millisecond)
}

func TestHTTPPump_TimeoutHonoured(t *testing.T) {
	c := &httpCollector{delay: 5 * time.Second}
	srv := httptest.NewServer(c)
	defer srv.Close()

	p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "max_retries": 5})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := p.WriteData(ctx, httpTestRecords(3))
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestHTTPPump_MCPOnly(t *testing.T) {
	c := &httpCollector{}
	srv := httptest.NewServer(c)
	defer srv.Close()

	data := []interface{}{
		analytics.AnalyticsRecord{APIID: "rest", ResponseCode: 200},
		httpTestRecords(1)[0],
		analytics.AnalyticsRecord{APIID: "rest", ResponseCode: 500},
	}

	t.Run("mcp_only keeps MCP records", func(t *testing.T) {
		p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "mcp_only": true})
		require.NoError(t, p.WriteData(context.Background(), data))
		reqs := c.received()
		require.Len(t, reqs, 1)
		require.Len(t, reqs[0].records, 1)
		assert.Equal(t, "api1", reqs[0].records[0]["api_id"])
	})

	t.Run("nothing is sent when no record matches", func(t *testing.T) {
		before := len(c.received())
		p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "mcp_only": true})
		require.NoError(t, p.WriteData(context.Background(), []interface{}{data[0], data[2]}))
		assert.Len(t, c.received(), before)
	})

	t.Run("without mcp_only every record is sent", func(t *testing.T) {
		before := c.recordCount()
		p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL})
		require.NoError(t, p.WriteData(context.Background(), data))
		assert.Equal(t, before+3, c.recordCount())
	})
}

func TestHTTPPump_EnvOverride(t *testing.T) {
	c := &httpCollector{}
	srv := httptest.NewServer(c)
	defer srv.Close()

	t.Setenv("TYK_PMP_PUMPS_HTTP_META_URL", srv.URL)
	t.Setenv("TYK_PMP_PUMPS_HTTP_META_HEADERS", "Authorization:Bearer from-env")
	t.Setenv("TYK_PMP_PUMPS_HTTP_META_BATCHSIZE", "2")
	t.Setenv("TYK_PMP_PUMPS_HTTP_META_MAXBODYBYTES", "100000")
	t.Setenv("TYK_PMP_PUMPS_HTTP_META_GZIP", "true")
	t.Setenv("TYK_PMP_PUMPS_HTTP_META_MAXRETRIES", "4")
	t.Setenv("TYK_PMP_PUMPS_HTTP_META_MCPONLY", "true")

	p := newHTTPTestPump(t, map[string]interface{}{"url": "http://overridden.invalid"})
	assert.Equal(t, srv.URL, p.conf.URL)
	assert.Equal(t, 2, p.conf.BatchSize)
	assert.Equal(t, 100000, p.conf.MaxBodyBytes)
	assert.True(t, p.conf.Gzip)
	assert.Equal(t, uint64(4), p.conf.MaxRetries)
	assert.True(t, p.conf.MCPOnly)

	require.NoError(t, p.WriteData(context.Background(), httpTestRecords(3)))
	reqs := c.received()
	require.Len(t, reqs, 2)
	assert.Equal(t, "Bearer from-env", reqs[0].header.Get("Authorization"))
	assert.Equal(t, "gzip", reqs[0].header.Get("Content-Encoding"))
}

func TestHTTPPump_TLSInstancesAreIndependent(t *testing.T) {
	c := &httpCollector{}
	srv := httptest.NewTLSServer(c)
	defer srv.Close()

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	require.NoError(t, os.WriteFile(caFile, caPEM, 0o600))

	defaultTransport := http.DefaultClient.Transport

	trusted := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "ssl_ca_file": caFile})
	untrusted := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL})
	insecure := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL, "ssl_insecure_skip_verify": true})

	assert.Equal(t, defaultTransport, http.DefaultClient.Transport, "http.DefaultClient must not be modified")

	// interleave the instances so a shared transport would show up as a wrong result
	assert.NoError(t, trusted.WriteData(context.Background(), httpTestRecords(1)))
	err := untrusted.WriteData(context.Background(), httpTestRecords(1))
	assert.ErrorContains(t, err, "certificate")
	assert.NoError(t, insecure.WriteData(context.Background(), httpTestRecords(1)))
	assert.NoError(t, trusted.WriteData(context.Background(), httpTestRecords(1)))
	assert.Error(t, untrusted.WriteData(context.Background(), httpTestRecords(1)))

	assert.Equal(t, 3, c.recordCount())
}

func TestHTTPPump_RedirectsAreNotFollowed(t *testing.T) {
	target := &httpCollector{}
	targetSrv := httptest.NewServer(target)
	defer targetSrv.Close()

	for _, status := range []int{http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, targetSrv.URL+"/elsewhere", status)
			}))
			defer redirector.Close()

			p := newHTTPTestPump(t, map[string]interface{}{
				"url":         redirector.URL,
				"headers":     map[string]string{"X-Api-Key": "secret"},
				"max_retries": 2,
			})
			err := p.WriteData(context.Background(), httpTestRecords(1))
			assert.ErrorContains(t, err, fmt.Sprintf("status code %d", status))
			assert.Empty(t, target.received(), "records and headers must not be replayed to the redirect target")
		})
	}
}

func TestHTTPPump_ErrorBodyIsBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(strings.Repeat("x", 1<<20))); err != nil {
			t.Log(err)
		}
	}))
	defer srv.Close()

	p := newHTTPTestPump(t, map[string]interface{}{"url": srv.URL})
	err := p.WriteData(context.Background(), httpTestRecords(1))
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 8<<10)
}

// The pump honours HTTP_PROXY as it is when the pump starts, without going
// through http.ProxyFromEnvironment, whose once-per-process cache would pin
// the proxy settings for every other client (and test) in the process.
func TestHTTPPump_ProxyFromEnvironment(t *testing.T) {
	proxy := &httpCollector{}
	proxySrv := httptest.NewServer(proxy)
	defer proxySrv.Close()

	t.Setenv("HTTP_PROXY", proxySrv.URL)
	t.Setenv("NO_PROXY", "")
	p := newHTTPTestPump(t, map[string]interface{}{"url": "http://collector.example.invalid/ingest"})
	require.NoError(t, p.WriteData(context.Background(), httpTestRecords(2)))
	assert.Equal(t, 2, proxy.recordCount(), "the request went through the proxy")

	// A pump started after the proxy is unset connects directly.
	t.Setenv("HTTP_PROXY", "")
	direct := &httpCollector{}
	directSrv := httptest.NewServer(direct)
	defer directSrv.Close()
	p = newHTTPTestPump(t, map[string]interface{}{"url": directSrv.URL})
	require.NoError(t, p.WriteData(context.Background(), httpTestRecords(1)))
	assert.Equal(t, 1, direct.recordCount())
	assert.Equal(t, 2, proxy.recordCount())
}
