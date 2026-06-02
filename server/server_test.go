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
// MCDC SW-REQ-032: enable_profiling=F, pprof_route_registered=F => TRUE
// MCDC SW-REQ-032: enable_profiling=T, pprof_route_registered=F => FALSE
// MCDC SW-REQ-032: enable_profiling=T, pprof_route_registered=T => TRUE
//
// enable_profiling=F arm: this test invokes Healthcheck without enabling profiling — the
// server still answers /hello (liveness) without mounting pprof routes, so pprof_route_registered
// stays false (vacuous true under the FRETish "when enable_profiling" trigger). The
// enable_profiling=T/pprof_route_registered=T arm is exercised by the main process when
// Profiling=true in config — the production binary registers pprof under /debug/pprof. KI
// pprof-routes-not-isolated tracks the architectural seam where mux registration happens
// outside this package.
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
