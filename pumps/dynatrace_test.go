package pumps

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

const (
	testDynatraceToken       = "dt0c01.test.token.abcdef123456"
	testDynatraceEndpointURL = "http://localhost:9999"
)

type dynatraceTestHandler struct {
	test         *testing.T
	batched      bool
	returnErrors int
	reqCount     int
	responses    []dynatraceTestResponse
}

type dynatraceTestResponse struct {
	StatusCode  int
	BodyLength  int
	EventsCount int
	Body        []byte
}

func (h *dynatraceTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.reqCount++

	// Check authorization header
	authHeaderValue := r.Header.Get(dynatraceAuthHeaderName)
	if authHeaderValue == "" {
		h.test.Fatal("Auth header is empty")
	}
	expectedValue := dynatraceAuthHeaderPrefix + testDynatraceToken
	if strings.Compare(authHeaderValue, expectedValue) != 0 {
		h.test.Fatalf("Auth header value doesn't match, got: %s, expected: %s", authHeaderValue, expectedValue)
	}

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		h.test.Fatalf("Content-Type header should contain application/json, got: %s", contentType)
	}

	if r.Body == nil {
		h.test.Fatal("Body is nil")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.test.Fatal("Couldn't read body")
	}
	defer r.Body.Close()

	// Return error if configured
	if h.returnErrors >= h.reqCount {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("dynatrace internal error"))
		if err != nil {
			h.test.Fatalf("Failed to write response got error %v", err)
		}
		return
	}

	response := dynatraceTestResponse{
		StatusCode: http.StatusNoContent,
		BodyLength: len(body),
		Body:       body,
	}

	if h.batched {
		// For batch mode, body should be a JSON array
		var events []interface{}
		err = json.Unmarshal(body, &events)
		if err != nil {
			h.test.Fatalf("Couldn't unmarshal batch event data: %v", err)
		}
		response.EventsCount = len(events)
	} else {
		// For non-batch mode, body should be a single JSON object
		event := make(map[string]interface{})
		err = json.Unmarshal(body, &event)
		if err != nil {
			h.test.Fatalf("Couldn't unmarshal event data: %v", err)
		}
		response.EventsCount = 1
	}

	w.WriteHeader(http.StatusNoContent)
	h.responses = append(h.responses, response)
}

func TestDynatraceInit(t *testing.T) {
	_, err := NewDynatraceClient("", testDynatraceEndpointURL, true, "", "", "")
	if err == nil {
		t.Fatal("A token needs to be present")
	}
	_, err = NewDynatraceClient(testDynatraceToken, "", true, "", "", "")
	if err == nil {
		t.Fatal("An endpoint needs to be present")
	}
	_, err = NewDynatraceClient("", "", true, "", "", "")
	if err == nil {
		t.Fatal("Empty parameters should return an error")
	}
}

func TestDynatraceBackoffRetry(t *testing.T) {
	t.Run("max_retries=1", func(t *testing.T) {
		handler := &dynatraceTestHandler{test: t, batched: false, returnErrors: 1}
		server := httptest.NewUnstartedServer(handler)
		server.Config.SetKeepAlivesEnabled(false)
		server.Start()
		defer server.Close()

		pmp := DynatracePump{}
		cfg := make(map[string]interface{})
		cfg["api_token"] = testDynatraceToken
		cfg["max_retries"] = 1
		cfg["endpoint_url"] = server.URL
		cfg["ssl_insecure_skip_verify"] = true

		if err := pmp.Init(cfg); err != nil {
			t.Errorf("Error initializing pump %v", err)
			return
		}

		keys := make([]interface{}, 1)
		keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

		if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
			t.Error("Error writing to dynatrace pump:", errWrite.Error())
			return
		}

		assert.Equal(t, 1, len(handler.responses))
		assert.Equal(t, 2, handler.reqCount)

		response := handler.responses[0]
		assert.Equal(t, http.StatusNoContent, response.StatusCode)
	})

	t.Run("max_retries=0", func(t *testing.T) {
		handler := &dynatraceTestHandler{test: t, batched: false, returnErrors: 1}
		server := httptest.NewUnstartedServer(handler)
		server.Config.SetKeepAlivesEnabled(false)
		server.Start()
		defer server.Close()

		pmp := DynatracePump{}
		cfg := make(map[string]interface{})
		cfg["api_token"] = testDynatraceToken
		cfg["max_retries"] = 0
		cfg["endpoint_url"] = server.URL
		cfg["ssl_insecure_skip_verify"] = true

		if err := pmp.Init(cfg); err != nil {
			t.Errorf("Error initializing pump %v", err)
			return
		}

		keys := make([]interface{}, 1)
		keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

		if errWrite := pmp.WriteData(context.TODO(), keys); errWrite == nil {
			t.Error("Error expected writing to dynatrace pump, got nil")
			return
		}

		assert.Equal(t, 1, handler.reqCount)
	})

	t.Run("max_retries=3", func(t *testing.T) {
		handler := &dynatraceTestHandler{test: t, batched: false, returnErrors: 2}
		server := httptest.NewUnstartedServer(handler)
		server.Config.SetKeepAlivesEnabled(false)
		server.Start()
		defer server.Close()

		pmp := DynatracePump{}
		cfg := make(map[string]interface{})
		cfg["api_token"] = testDynatraceToken
		cfg["max_retries"] = 3
		cfg["endpoint_url"] = server.URL
		cfg["ssl_insecure_skip_verify"] = true

		if err := pmp.Init(cfg); err != nil {
			t.Errorf("Error initializing pump %v", err)
			return
		}

		keys := make([]interface{}, 1)
		keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

		if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
			t.Error("Error writing to dynatrace pump:", errWrite.Error())
			return
		}

		assert.Equal(t, 1, len(handler.responses))
		assert.Equal(t, 3, handler.reqCount)

		response := handler.responses[0]
		assert.Equal(t, http.StatusNoContent, response.StatusCode)
	})
}

func TestDynatraceWriteData(t *testing.T) {
	handler := &dynatraceTestHandler{test: t, batched: false}
	server := httptest.NewServer(handler)
	defer server.Close()

	pmp := DynatracePump{}

	cfg := make(map[string]interface{})
	cfg["api_token"] = testDynatraceToken
	cfg["endpoint_url"] = server.URL
	cfg["ssl_insecure_skip_verify"] = true

	if errInit := pmp.Init(cfg); errInit != nil {
		t.Error("Error initializing pump")
		return
	}

	keys := make([]interface{}, 1)
	keys[0] = analytics.AnalyticsRecord{
		OrgID:        "1",
		APIID:        "123",
		Path:         "/test-path",
		RawPath:      "/test-path",
		Method:       "POST",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
		IPAddress:    "127.0.0.1",
	}

	if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
		t.Error("Error writing to dynatrace pump:", errWrite.Error())
		return
	}

	assert.Equal(t, 1, len(handler.responses))

	response := handler.responses[0]
	assert.Equal(t, http.StatusNoContent, response.StatusCode)
	assert.Equal(t, 1, response.EventsCount)
}

// getDynatraceSingleEventBytes returns the bytes amount of a single marshalled event
func getDynatraceSingleEventBytes(record interface{}, t *testing.T) int {
	decoded := record.(analytics.AnalyticsRecord)

	event := map[string]interface{}{
		"http.method":      decoded.Method,
		"http.url":         decoded.RawPath,
		"http.status_code": decoded.ResponseCode,
		"http.client_ip":   decoded.IPAddress,
		"api_key":          decoded.APIKey,
		"api_version":      decoded.APIVersion,
		"api_name":         decoded.APIName,
		"api_id":           decoded.APIID,
		"org_id":           decoded.OrgID,
		"oauth_id":         decoded.OauthID,
		"raw_request":      decoded.RawRequest,
		"request_time":     decoded.RequestTime,
		"raw_response":     decoded.RawResponse,
		"timestamp":        decoded.TimeStamp.UnixMilli(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal("Failed to marshal event:", err)
	}
	return len(data)
}

func TestDynatraceWriteDataBatch(t *testing.T) {
	handler := &dynatraceTestHandler{test: t, batched: true}
	server := httptest.NewServer(handler)
	defer server.Close()

	keys := make([]interface{}, 3)
	timestamp := time.Now()

	keys[0] = analytics.AnalyticsRecord{
		OrgID:        "1",
		APIID:        "123",
		Path:         "/test-path",
		RawPath:      "/test-path",
		Method:       "POST",
		ResponseCode: 200,
		TimeStamp:    timestamp,
		IPAddress:    "127.0.0.1",
	}
	keys[1] = analytics.AnalyticsRecord{
		OrgID:        "1",
		APIID:        "124",
		Path:         "/test-path-2",
		RawPath:      "/test-path-2",
		Method:       "GET",
		ResponseCode: 200,
		TimeStamp:    timestamp,
		IPAddress:    "127.0.0.2",
	}
	keys[2] = analytics.AnalyticsRecord{
		OrgID:        "1",
		APIID:        "125",
		Path:         "/test-path-3",
		RawPath:      "/test-path-3",
		Method:       "PUT",
		ResponseCode: 201,
		TimeStamp:    timestamp,
		IPAddress:    "127.0.0.3",
	}

	pmp := DynatracePump{}

	cfg := make(map[string]interface{})
	cfg["api_token"] = testDynatraceToken
	cfg["endpoint_url"] = server.URL
	cfg["ssl_insecure_skip_verify"] = true
	cfg["enable_batch"] = true
	// Calculate batch size: opening bracket (1) + first event + comma (1) + second event + closing bracket (1)
	// We want the max content to fit exactly 2 events, so the third will trigger a new batch
	firstEventSize := getDynatraceSingleEventBytes(keys[0], t)
	secondEventSize := getDynatraceSingleEventBytes(keys[1], t)
	cfg["batch_max_content_length"] = 1 + firstEventSize + 1 + secondEventSize + 1

	if errInit := pmp.Init(cfg); errInit != nil {
		t.Error("Error initializing pump:", errInit)
		return
	}

	if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
		t.Error("Error writing to dynatrace pump:", errWrite.Error())
		return
	}

	// Should result in 2 batches: first with 2 events, second with 1 event
	assert.Equal(t, 2, len(handler.responses))
	assert.Equal(t, 2, handler.responses[0].EventsCount)
	assert.Equal(t, 1, handler.responses[1].EventsCount)
}

func TestDynatraceFilterTags(t *testing.T) {
	pmp := DynatracePump{}
	cfg := make(map[string]interface{})
	cfg["api_token"] = testDynatraceToken
	cfg["endpoint_url"] = testDynatraceEndpointURL
	cfg["ssl_insecure_skip_verify"] = true
	cfg["ignore_tag_prefix_list"] = []string{"key-", "test-"}

	if err := pmp.Init(cfg); err != nil {
		t.Fatal("Error initializing pump:", err)
	}

	tags := []string{"key-123", "test-456", "valid-tag", "another-valid"}
	filtered := pmp.FilterTags(tags)

	// Should only contain tags that don't start with "key-" or "test-"
	assert.Equal(t, 2, len(filtered))
	assert.Contains(t, filtered, "valid-tag")
	assert.Contains(t, filtered, "another-valid")
}

func TestDynatraceObfuscateAPIKeys(t *testing.T) {
	handler := &dynatraceTestHandler{test: t, batched: false}
	server := httptest.NewServer(handler)
	defer server.Close()

	pmp := DynatracePump{}

	cfg := make(map[string]interface{})
	cfg["api_token"] = testDynatraceToken
	cfg["endpoint_url"] = server.URL
	cfg["ssl_insecure_skip_verify"] = true
	cfg["obfuscate_api_keys"] = true
	cfg["obfuscate_api_keys_length"] = 4

	if errInit := pmp.Init(cfg); errInit != nil {
		t.Error("Error initializing pump")
		return
	}

	keys := make([]interface{}, 1)
	keys[0] = analytics.AnalyticsRecord{
		OrgID:        "1",
		APIID:        "123",
		Path:         "/test-path",
		RawPath:      "/test-path",
		Method:       "POST",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
		IPAddress:    "127.0.0.1",
		APIKey:       "1234567890",
	}

	if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
		t.Error("Error writing to dynatrace pump:", errWrite.Error())
		return
	}

	assert.Equal(t, 1, len(handler.responses))
}

func TestDynatraceCustomFields(t *testing.T) {
	handler := &dynatraceTestHandler{test: t, batched: false}
	server := httptest.NewServer(handler)
	defer server.Close()

	pmp := DynatracePump{}

	cfg := make(map[string]interface{})
	cfg["api_token"] = testDynatraceToken
	cfg["endpoint_url"] = server.URL
	cfg["ssl_insecure_skip_verify"] = true
	cfg["fields"] = []string{"http.method", "http.url", "api_id"}

	if errInit := pmp.Init(cfg); errInit != nil {
		t.Error("Error initializing pump")
		return
	}

	keys := make([]interface{}, 1)
	keys[0] = analytics.AnalyticsRecord{
		OrgID:        "1",
		APIID:        "123",
		Path:         "/test-path",
		RawPath:      "/test-path",
		Method:       "POST",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
		IPAddress:    "127.0.0.1",
	}

	if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
		t.Error("Error writing to dynatrace pump:", errWrite.Error())
		return
	}

	assert.Equal(t, 1, len(handler.responses))

	// Verify only the configured fields are present in the event
	var event map[string]interface{}
	err := json.Unmarshal(handler.responses[0].Body, &event)
	if err != nil {
		t.Fatal("Failed to unmarshal event body:", err)
	}

	// Check that only configured fields are present
	assert.Equal(t, 4, len(event)) // 3 custom fields + timestamp
	assert.Contains(t, event, "http.method")
	assert.Contains(t, event, "http.url")
	assert.Contains(t, event, "api_id")

	// Verify the values are correct
	assert.Equal(t, "POST", event["http.method"])
	assert.Equal(t, "/test-path", event["http.url"])
	assert.Equal(t, "123", event["api_id"])
}

func TestDynatraceCustomProperties(t *testing.T) {
	handler := &dynatraceTestHandler{test: t, batched: false}
	server := httptest.NewServer(handler)
	defer server.Close()

	pmp := DynatracePump{}

	cfg := make(map[string]interface{})
	cfg["api_token"] = testDynatraceToken
	cfg["endpoint_url"] = server.URL
	cfg["ssl_insecure_skip_verify"] = true
	cfg["properties"] = map[string]string{
		"environment": "production",
		"region":      "us-east-1",
	}

	if errInit := pmp.Init(cfg); errInit != nil {
		t.Error("Error initializing pump")
		return
	}

	keys := make([]interface{}, 1)
	keys[0] = analytics.AnalyticsRecord{
		OrgID:        "1",
		APIID:        "123",
		Path:         "/test-path",
		RawPath:      "/test-path",
		Method:       "POST",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
		IPAddress:    "127.0.0.1",
	}

	if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
		t.Error("Error writing to dynatrace pump:", errWrite.Error())
		return
	}

	assert.Equal(t, 1, len(handler.responses))
}

func TestDynatraceGetName(t *testing.T) {
	pmp := DynatracePump{}
	assert.Equal(t, dynatracePumpName, pmp.GetName())
}

func TestDynatraceNew(t *testing.T) {
	pmp := DynatracePump{}
	newPump := pmp.New()
	assert.IsType(t, &DynatracePump{}, newPump)
}
