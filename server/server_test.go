package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Verifies: SW-REQ-032
// SW-REQ-032:nominal:example
// Verifies: SYS-REQ-012
// Verifies: STK-REQ-004
func TestHealthcheck_ReportsLiveness(t *testing.T) {
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)

	Healthcheck(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rw.Code)
	}
	if ct := rw.Header().Get("Content-type"); ct != "application/json" {
		t.Fatalf("expected json content-type, got %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}
