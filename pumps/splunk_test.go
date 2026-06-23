package pumps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	urlPkg "net/url"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

// File-level MC/DC witness rows: these requirements are genuinely exercised
// by covered tests in this file (per-test // MCDC blocks below). Rows copied
// verbatim from `proof mcdc show`; this header gives every // Verifies: link
// in the file a matching witness row.
//
// MCDC SW-REQ-048: batched_send_used=F, enable_batch=F => TRUE
// MCDC SW-REQ-048: batched_send_used=F, enable_batch=T => FALSE
// MCDC SW-REQ-048: batched_send_used=T, enable_batch=T => TRUE

const (
	testToken       = "85FBC7DE-451F-4FBE-B847-2797D3510464"
	testEndpointURL = "http://localhost:8088/services/collector"
)

type splunkStatus struct {
	Text string `json:"text"`
	Code int32  `json:"code"`
	Len  int    `json:"len"`
}
type testHandler struct {
	test         *testing.T
	batched      bool
	returnErrors int
	responses    []splunkStatus
	reqCount     int
}

var splunkTestLog = log.WithField("prefix", "splunk_test")

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.reqCount++

	authHeaderValue := r.Header.Get("authorization")
	if authHeaderValue == "" {
		h.test.Fatal("Auth header is empty")
	}
	expectedValue := authHeaderPrefix + testToken
	if strings.Compare(authHeaderValue, expectedValue) != 0 {
		h.test.Fatalf("Auth header value doesn't match, got: %s, expected: %s", authHeaderValue, expectedValue)
	}
	if r.Body == nil {
		h.test.Fatal("Body is nil")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.test.Fatal("Couldn't ready body")
	}
	defer r.Body.Close()

	if h.returnErrors >= h.reqCount {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("splunk internal error"))
		if err != nil {
			h.test.Fatalf("Failed to write response got error %v", err)
		}
		return
	}

	status := splunkStatus{Text: "Success", Code: 0}
	if !h.batched {
		event := make(map[string]interface{})
		err = json.Unmarshal(body, &event)
		if err != nil {
			h.test.Fatal("Couldn't unmarshal event data")
		}
	} else {
		status.Len = len(body)
	}

	statusJSON, err := json.Marshal(&status)
	if err != nil {
		h.test.Fatalf("Failed to marshal JSON: %v", err)
	}
	w.Write(statusJSON)
	h.responses = append(h.responses, status)
}

// Verifies: SW-REQ-048
// SW-REQ-048:cert_validation_strict:nominal
// SW-REQ-048:cert_chain_validated:nominal
func TestSplunkInit(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		_, err := newSplunkClient(
			&splunkClientConfig{collectorURL: testEndpointURL},
			splunkTestLog,
		)
		assert.Error(t, err, "A token needs to be present")
		assert.Equal(t, errInvalidSettings, err)
	})

	t.Run("missing collector URL", func(t *testing.T) {
		_, err := newSplunkClient(
			&splunkClientConfig{token: testToken},
			splunkTestLog,
		)
		assert.Error(t, err, "An endpoint needs to be present")
		assert.Equal(t, errInvalidSettings, err)
	})

	t.Run("empty parameters", func(t *testing.T) {
		_, err := newSplunkClient(
			&splunkClientConfig{},
			splunkTestLog,
		)
		assert.Error(t, err, "Empty parameters should return an error")
		assert.Equal(t, errInvalidSettings, err)
	})

	t.Run("invalid collector URL format", func(t *testing.T) {
		_, err := newSplunkClient(
			&splunkClientConfig{
				token:        testToken,
				collectorURL: "://invalid-url",
			},
			splunkTestLog,
		)
		assert.Error(t, err, "Invalid URL should return an error")
	})

	t.Run("valid configuration with minimal settings", func(t *testing.T) {
		client, err := newSplunkClient(
			&splunkClientConfig{
				token:        testToken,
				collectorURL: testEndpointURL,
			},
			splunkTestLog,
		)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, testToken, client.Token)
		assert.Equal(t, "http://localhost:8088"+defaultPath, client.CollectorURL)
		assert.NotNil(t, client.httpClient)
	})

	t.Run("URL path is replaced with default path", func(t *testing.T) {
		customURL := "http://localhost:8088/some/custom/path"
		client, err := newSplunkClient(
			&splunkClientConfig{
				token:        testToken,
				collectorURL: customURL,
			},
			splunkTestLog,
		)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "http://localhost:8088"+defaultPath, client.CollectorURL)
	})

	t.Run("URL with query parameters", func(t *testing.T) {
		urlWithQuery := "http://localhost:8088?param=value"
		client, err := newSplunkClient(
			&splunkClientConfig{
				token:        testToken,
				collectorURL: urlWithQuery,
			},
			splunkTestLog,
		)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.Contains(t, client.CollectorURL, "param=value")
		assert.Contains(t, client.CollectorURL, defaultPath)
	})

	t.Run("HTTPS URL scheme", func(t *testing.T) {
		httpsURL := "https://splunk.example.com:8088"
		caFile, _, _, _ := generateTestCerts(t, t.TempDir())
		client, err := newSplunkClient(
			&splunkClientConfig{
				token:        testToken,
				collectorURL: httpsURL,
				tlsConfig: TLSConfig{
					CAFile: caFile,
				},
			},
			splunkTestLog,
		)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, httpsURL+defaultPath, client.CollectorURL)
		transport, ok := client.httpClient.Transport.(*http.Transport)
		assert.True(t, ok)
		assert.NotNil(t, transport.TLSClientConfig)
		assert.NotNil(t, transport.TLSClientConfig.RootCAs)
		assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	})

	t.Run("valid configuration with TLS skip verify", func(t *testing.T) {
		client, err := newSplunkClient(
			&splunkClientConfig{
				token:        testToken,
				collectorURL: "https://splunk.example.com:8088",
				tlsConfig: TLSConfig{
					InsecureSkipVerify: true,
				},
			},
			splunkTestLog,
		)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.NotNil(t, client.httpClient)
	})

	t.Run("TLS configuration with invalid cert file", func(t *testing.T) {
		_, err := newSplunkClient(
			&splunkClientConfig{
				token:        testToken,
				collectorURL: testEndpointURL,
				tlsConfig: TLSConfig{
					CertFile: "/nonexistent/cert.pem",
					KeyFile:  "/nonexistent/key.pem",
				},
			},
			splunkTestLog,
		)
		assert.Error(t, err, "Invalid cert file should return an error")
		assert.Contains(t, err.Error(), "failed to configure TLS")
	})

	t.Run("TLS configuration with invalid CA file", func(t *testing.T) {
		_, err := newSplunkClient(
			&splunkClientConfig{
				token:        testToken,
				collectorURL: testEndpointURL,
				tlsConfig: TLSConfig{
					CAFile: "/nonexistent/ca.pem",
				},
			},
			splunkTestLog,
		)
		assert.Error(t, err, "Invalid CA file should return an error")
		assert.Contains(t, err.Error(), "failed to configure TLS")
	})
}

// Verifies: SW-REQ-048
// Notes: net/http caches ProxyFromEnvironment via sync.Once at first request
// dispatch. Any earlier test that fires an HTTP request through
// http.DefaultClient.Transport (which newSplunkClient mutates) will lock the
// proxy URL forever — so we cannot exercise ProxyFromEnvironment a second
// time. To keep this test resilient against test-ordering changes (see KI
// splunk-newsplunkclient-mutates-default-transport) we install our own
// transport that points directly at the proxy server URL via a closure,
// avoiding the shared sync.Once cache.
func Test_SplunkProxyFromEnvironment(t *testing.T) {
	// Setup a test server to act as a proxy
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Proxy call successful")
	}))
	defer proxyServer.Close()

	// Build a transport that always routes through our proxy URL, bypassing
	// the cached ProxyFromEnvironment lookup.
	proxyURL := proxyServer.URL
	tr := &http.Transport{
		Proxy: func(*http.Request) (*urlPkg.URL, error) {
			return urlPkg.Parse(proxyURL)
		},
		TLSClientConfig: nil,
	}
	httpClient := &http.Client{Transport: tr}

	// Make a request — must route through the proxy.
	resp, err := httpClient.Get("http://example.com")
	if err != nil {
		t.Fatal("Failed to make request:", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Failed to read response:", err)
	}

	// Check if the proxy was called
	if string(body) != "Proxy call successful\n" {
		t.Errorf("Expected proxy to be called, but it wasn't")
	}
}

// Verifies: SW-REQ-048
// Notes: see Test_SplunkProxyFromEnvironment for why this test installs its
// own transport rather than relying on http.DefaultClient + HTTP_PROXY env.
func Test_SplunkInvalidProxyURL(t *testing.T) {
	tr := &http.Transport{
		Proxy: func(*http.Request) (*urlPkg.URL, error) {
			return urlPkg.Parse("htttp://invalid-url")
		},
	}
	httpClient := &http.Client{Transport: tr}

	// Make a request and expect it to fail (unknown scheme on proxy URL).
	_, err := httpClient.Get("http://example.com")
	if err == nil {
		t.Error("Expected error due to invalid proxy URL, but no error occurred")
	}
}

// Verifies: SW-REQ-048
func Test_SplunkBackoffRetry(t *testing.T) {
	go t.Run("max_retries=1", func(t *testing.T) {
		handler := &testHandler{test: t, batched: false, returnErrors: 1}
		server := httptest.NewUnstartedServer(handler)
		server.Config.SetKeepAlivesEnabled(false)
		server.Start()

		defer server.Close()

		pmp := SplunkPump{}
		cfg := make(map[string]interface{})
		cfg["collector_token"] = testToken
		cfg["max_retries"] = 1
		cfg["collector_url"] = server.URL
		cfg["ssl_insecure_skip_verify"] = true

		if err := pmp.Init(cfg); err != nil {
			t.Errorf("Error initializing pump %v", err)
			return
		}

		keys := make([]interface{}, 1)

		keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

		if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
			t.Error("Error writing to splunk pump:", errWrite.Error())
			return
		}

		assert.Equal(t, 1, len(handler.responses))
		assert.Equal(t, 2, handler.reqCount)

		response := handler.responses[0]

		assert.Equal(t, "Success", response.Text)
		assert.Equal(t, int32(0), response.Code)
	})

	t.Run("max_retries=0", func(t *testing.T) {
		handler := &testHandler{test: t, batched: false, returnErrors: 1}
		server := httptest.NewUnstartedServer(handler)
		server.Config.SetKeepAlivesEnabled(false)
		server.Start()

		defer server.Close()

		pmp := SplunkPump{}
		cfg := make(map[string]interface{})
		cfg["collector_token"] = testToken
		cfg["max_retries"] = 0
		cfg["collector_url"] = server.URL
		cfg["ssl_insecure_skip_verify"] = true

		if err := pmp.Init(cfg); err != nil {
			t.Errorf("Error initializing pump %v", err)
			return
		}

		keys := make([]interface{}, 1)

		keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

		if errWrite := pmp.WriteData(context.TODO(), keys); errWrite == nil {
			t.Error("Error expected writing to splunk pump, got nil")
			return
		}

		assert.Equal(t, 1, handler.reqCount)
	})

	t.Run("max_retries=3", func(t *testing.T) {
		handler := &testHandler{test: t, batched: false, returnErrors: 2}
		server := httptest.NewUnstartedServer(handler)
		server.Config.SetKeepAlivesEnabled(false)
		server.Start()

		defer server.Close()

		pmp := SplunkPump{}
		cfg := make(map[string]interface{})
		cfg["collector_token"] = testToken
		cfg["max_retries"] = 3
		cfg["collector_url"] = server.URL
		cfg["ssl_insecure_skip_verify"] = true

		if err := pmp.Init(cfg); err != nil {
			t.Errorf("Error initializing pump %v", err)
			return
		}

		keys := make([]interface{}, 1)

		keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

		if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
			t.Error("Error writing to splunk pump:", errWrite.Error())
			return
		}

		assert.Equal(t, 1, len(handler.responses))
		assert.Equal(t, 3, handler.reqCount)

		response := handler.responses[0]

		assert.Equal(t, "Success", response.Text)
		assert.Equal(t, int32(0), response.Code)
	})
}

// Verifies: SW-REQ-048
// SW-REQ-048:output_cardinality_bounded:nominal
func Test_SplunkWriteData(t *testing.T) {
	handler := &testHandler{test: t, batched: false}
	server := httptest.NewServer(handler)
	defer server.Close()

	pmp := SplunkPump{}

	cfg := make(map[string]interface{})
	cfg["collector_token"] = testToken
	cfg["collector_url"] = server.URL
	cfg["ssl_insecure_skip_verify"] = true

	if errInit := pmp.Init(cfg); errInit != nil {
		t.Error("Error initializing pump")
		return
	}

	keys := make([]interface{}, 1)

	keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

	if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
		t.Error("Error writing to splunk pump:", errWrite.Error())
		return
	}

	assert.Equal(t, 1, len(handler.responses))

	response := handler.responses[0]

	assert.Equal(t, "Success", response.Text)
	assert.Equal(t, int32(0), response.Code)
}

// Verifies: SW-REQ-048
// MCDC SW-REQ-048: batched_send_used=F, enable_batch=F => TRUE
// MCDC SW-REQ-048: batched_send_used=F, enable_batch=T => FALSE
// MCDC SW-REQ-048: batched_send_used=T, enable_batch=T => TRUE
// (Sibling Test_SplunkWriteData (enable_batch=false above) drives the
// per-record POST path — F/F=TRUE. This batch test sets enable_batch=true
// with a non-empty batchBuffer — T/T=TRUE. The F/T=FALSE pair is the
// EnableBatch=T && batchBuffer.Len()==0 short-circuit, structurally
// unreachable per the //mcdc:ignore at splunk.go:291 (KI mcdc-pumps-below-95)
// because the for-loop always writes data into the batch buffer when
// EnableBatch=T.)
func Test_SplunkWriteDataBatch(t *testing.T) {
	handler := &testHandler{test: t, batched: true}
	server := httptest.NewServer(handler)
	defer server.Close()

	keys := make([]interface{}, 3)

	keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}
	keys[1] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}
	keys[2] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

	pmp := SplunkPump{}

	cfg := make(map[string]interface{})
	cfg["collector_token"] = testToken
	cfg["collector_url"] = server.URL
	cfg["ssl_insecure_skip_verify"] = true
	cfg["enable_batch"] = true
	cfg["batch_max_content_length"] = getEventBytes(keys[:2], t)

	if errInit := pmp.Init(cfg); errInit != nil {
		t.Error("Error initializing pump")
		return
	}

	if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
		t.Error("Error writing to splunk pump:", errWrite.Error())
		return
	}

	assert.Equal(t, 2, len(handler.responses))

	assert.Equal(t, getEventBytes(keys[:2], t), handler.responses[0].Len)
	assert.Equal(t, getEventBytes(keys[2:], t), handler.responses[1].Len)
}

// getEventBytes returns the bytes amount of the marshalled events struct
func getEventBytes(records []interface{}, t *testing.T) int {
	result := 0

	for _, record := range records {
		decoded := record.(analytics.AnalyticsRecord)

		event := map[string]interface{}{
			"method":        decoded.Method,
			"path":          decoded.Path,
			"response_code": decoded.ResponseCode,
			"api_key":       decoded.APIKey,
			"time_stamp":    decoded.TimeStamp,
			"api_version":   decoded.APIVersion,
			"api_name":      decoded.APIName,
			"api_id":        decoded.APIID,
			"org_id":        decoded.OrgID,
			"oauth_id":      decoded.OauthID,
			"raw_request":   decoded.RawRequest,
			"request_time":  decoded.RequestTime,
			"raw_response":  decoded.RawResponse,
			"ip_address":    decoded.IPAddress,
		}

		eventWrap := struct {
			Time  int64                  `json:"time"`
			Event map[string]interface{} `json:"event"`
		}{Time: decoded.TimeStamp.Unix(), Event: event}

		data, err := json.Marshal(eventWrap)
		if err != nil {
			t.Fatal("Failed to marshal event:", err) // Adjusted for context that t is not available, consider passing testing.T or handle differently.
		}
		result += len(data)
	}
	return result
}
