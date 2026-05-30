package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Verifies: SW-REQ-032
// SW-REQ-032:nominal:negative
func TestResolveHealthCheckParams_AllBranches(t *testing.T) {
	cases := []struct {
		name     string
		inEnd    string
		inPort   int
		wantEnd  string
		wantPort int
	}{
		{"both defaults", "", 0, defaultHealthEndpoint, defaultHealthPort},
		{"endpoint configured", "ping", 0, "ping", defaultHealthPort},
		{"port configured", "", 9000, defaultHealthEndpoint, 9000},
		{"both configured", "alive", 9001, "alive", 9001},
	}
	for _, c := range cases {
		gotE, gotP := resolveHealthCheckParams(c.inEnd, c.inPort)
		if gotE != c.wantEnd || gotP != c.wantPort {
			t.Fatalf("%s: got (%q,%d) want (%q,%d)", c.name, gotE, gotP, c.wantEnd, c.wantPort)
		}
	}
}

// Verifies: SW-REQ-032
func TestBuildHealthCheckRouter_RegistersExpectedRoutes(t *testing.T) {
	// health-only (no profiling): /<endpoint> registered, /debug/pprof/* not
	r := buildHealthCheckRouter("alive", false)
	if r == nil {
		t.Fatal("router is nil")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alive", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/alive returned %d, want 200", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("/debug/pprof/ without profiling returned %d, want 404", rec.Code)
	}
}

// Verifies: SW-REQ-032
func TestBuildHealthCheckRouter_RegistersPprofWhenEnabled(t *testing.T) {
	// With profiling enabled, /debug/pprof/heap should route through the pprof handler
	// (which serves the heap profile inline). A non-404 response confirms registration.
	r := buildHealthCheckRouter("health", true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap?debug=1", nil)
	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("/debug/pprof/heap returned 404 with profiling enabled; pprof catchall not registered")
	}
}
