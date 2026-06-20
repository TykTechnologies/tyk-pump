package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// STK-REQ-004:AC-001:acceptance
func TestAcceptance_HTTPHealthEndpointReportsLiveness(t *testing.T) {
	srv := httptest.NewServer(buildHealthCheckRouter("health", false))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %v, want ok", body)
	}

	missing, err := http.Get(srv.URL + "/not-health")
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode == http.StatusOK {
		t.Fatal("unrelated route unexpectedly returned 200")
	}
}
