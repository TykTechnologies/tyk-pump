package pumps

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

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
	test    *testing.T
	batched bool

	responses []splunkStatus
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.test.Fatal("Couldn't ready body")
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

	statusJSON, _ := json.Marshal(&status)
	w.Write(statusJSON)
	h.responses = append(h.responses, status)
}

func TestSplunkInit(t *testing.T) {
	_, err := NewSplunkClient("", testEndpointURL, true, "", "", "")
	if err == nil {
		t.Fatal("A token needs to be present")
	}
	_, err = NewSplunkClient(testToken, "", true, "", "", "")
	if err == nil {
		t.Fatal("An endpoint needs to be present", "", "")
	}
	_, err = NewSplunkClient("", "", true, "", "", "")
	if err == nil {
		t.Fatal("Empty parameters should return an error")
	}
}

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
func Test_SplunkWriteDataBatch(t *testing.T) {
	handler := &testHandler{test: t, batched: true}
	server := httptest.NewServer(handler)
	defer server.Close()

	keys := make([]interface{}, 3)

	keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}
	keys[1] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}
	keys[2] = analytics.AnalyticsRecord{OrgID: "1", APIID: "123", Path: "/test-path", Method: "POST", TimeStamp: time.Now()}

	fmt.Println(maxContentLength)

	pmp := SplunkPump{}

	cfg := make(map[string]interface{})
	cfg["collector_token"] = testToken
	cfg["collector_url"] = server.URL
	cfg["ssl_insecure_skip_verify"] = true
	cfg["enable_batch"] = true
	cfg["batch_max_content_length"] = getEventBytes(keys[:2])

	if errInit := pmp.Init(cfg); errInit != nil {
		t.Error("Error initializing pump")
		return
	}

	if errWrite := pmp.WriteData(context.TODO(), keys); errWrite != nil {
		t.Error("Error writing to splunk pump:", errWrite.Error())
		return
	}

	assert.Equal(t, 2, len(handler.responses))

	assert.Equal(t, getEventBytes(keys[:2]), handler.responses[0].Len)
	assert.Equal(t, getEventBytes(keys[2:]), handler.responses[1].Len)

}

// getEventBytes returns the bytes amount of the marshalled events struct
func getEventBytes(records []interface{}) int {
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

		data, _ := json.Marshal(eventWrap)
		result += len(data)
	}
	return result
}
