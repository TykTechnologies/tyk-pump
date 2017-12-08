package pumps

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	testToken       = "85FBC7DE-451F-4FBE-B847-2797D3510464"
	testEndpointURL = "http://localhost:8088/services/collector"
)

type splunkStatus struct {
	Text string `json:"text"`
	Code int32  `json:"code"`
}
type testHandler struct {
	test *testing.T
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
	event := make(map[string]interface{})
	err = json.Unmarshal(body, &event)
	if err != nil {
		h.test.Fatal("Couldn't unmarshal event data")
	}
	status := splunkStatus{Text: "Success", Code: 0}
	statusJSON, _ := json.Marshal(&status)
	w.Write(statusJSON)
}

func TestSplunkInit(t *testing.T) {
	_, err := NewSplunkClient("", testEndpointURL, true)
	if err == nil {
		t.Fatal("A token needs to be present")
	}
	_, err = NewSplunkClient(testToken, "", true)
	if err == nil {
		t.Fatal("An endpoint needs to be present")
	}
	_, err = NewSplunkClient("", "", true)
	if err == nil {
		t.Fatal("Empty parameters should return an error")
	}
}

func TestSplunkSend(t *testing.T) {
	handler := &testHandler{t}
	server := httptest.NewServer(handler)
	defer server.Close()
	client, _ := NewSplunkClient(testToken, server.URL, true)
	e := map[string]interface{}{
		"method": "POST",
		"api_id": "123",
		"path":   "/test-path",
	}
	res, err := client.Send(e, time.Now())
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	status := new(splunkStatus)
	err = json.Unmarshal(body, &status)
	if err != nil {
		t.Fatal(err)
	}
	if status.Code != 0 || status.Text != "Success" {
		t.Fatalf("Bad status")
	}
}
