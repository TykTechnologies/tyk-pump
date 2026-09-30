package promtest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseQueryResponse(t *testing.T) {
	series, err := ParseQueryResponse(fixture(t, "query_uptime.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 {
		t.Fatalf("got %d series, want 1", len(series))
	}
	s := series[0]
	if s.Name() != "process_uptime_seconds" {
		t.Errorf("name = %q", s.Name())
	}
	if s.Value != 42.5 {
		t.Errorf("value = %v, want 42.5", s.Value)
	}
	if s.Labels["service_name"] != ServiceName {
		t.Errorf("service_name = %q", s.Labels["service_name"])
	}
	wantKeys := []string{"deployment_environment", "host_name", "instance", "job", "process_pid", "service_instance_id", "service_name", "service_version"}
	if got := s.LabelKeys(); !reflect.DeepEqual(got, wantKeys) {
		t.Errorf("LabelKeys() = %v, want %v", got, wantKeys)
	}
}

func TestParseQueryResponse_Empty(t *testing.T) {
	series, err := ParseQueryResponse(fixture(t, "query_empty.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 0 {
		t.Fatalf("got %d series, want 0", len(series))
	}
}

func TestParseQueryResponse_Error(t *testing.T) {
	_, err := ParseQueryResponse(fixture(t, "query_error.json"))
	if err == nil || !strings.Contains(err.Error(), "parse error") {
		t.Fatalf("expected the Prometheus error to surface, got %v", err)
	}
}

func TestParseMetadataResponse(t *testing.T) {
	md, err := ParseMetadataResponse(fixture(t, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if md["process_uptime_seconds"].Type != "gauge" {
		t.Errorf("type = %q, want gauge", md["process_uptime_seconds"].Type)
	}
	if _, ok := md["nope"]; ok {
		t.Error("unexpected family")
	}

	empty, err := ParseMetadataResponse(fixture(t, "metadata_empty.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("got %d families, want 0", len(empty))
	}
}

func TestSeries_MissingLabels(t *testing.T) {
	series, err := ParseQueryResponse(fixture(t, "query_missing_label.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := series[0].MissingLabels(IdentityLabels...)
	// deployment_environment is absent; service_version is present but empty.
	want := []string{"deployment_environment", "service_version"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingLabels() = %v, want %v", got, want)
	}
}

func TestFamily_Check(t *testing.T) {
	uptime := Families[0]
	good, _ := ParseQueryResponse(fixture(t, "query_uptime.json"))
	bad, _ := ParseQueryResponse(fixture(t, "query_missing_label.json"))

	t.Run("match", func(t *testing.T) {
		if problems := uptime.Check(Metadata{Type: "gauge"}, true, good); len(problems) != 0 {
			t.Errorf("unexpected problems: %v", problems)
		}
	})
	t.Run("missing family", func(t *testing.T) {
		problems := uptime.Check(Metadata{}, false, nil)
		if len(problems) != 1 || !strings.Contains(problems[0], "missing or renamed") || !strings.Contains(problems[0], "process.uptime") {
			t.Errorf("problems = %v", problems)
		}
	})
	t.Run("wrong type", func(t *testing.T) {
		problems := uptime.Check(Metadata{Type: "counter"}, true, good)
		if len(problems) != 1 || !strings.Contains(problems[0], `type is "counter", want "gauge"`) {
			t.Errorf("problems = %v", problems)
		}
	})
	t.Run("no series", func(t *testing.T) {
		problems := uptime.Check(Metadata{Type: "gauge"}, true, nil)
		if len(problems) != 1 || !strings.Contains(problems[0], "no series exported") {
			t.Errorf("problems = %v", problems)
		}
	})
	t.Run("missing labels", func(t *testing.T) {
		problems := uptime.Check(Metadata{Type: "gauge"}, true, bad)
		if len(problems) != 1 || !strings.Contains(problems[0], "[deployment_environment service_version]") {
			t.Errorf("problems = %v", problems)
		}
	})
}

func TestSelector(t *testing.T) {
	if got := Selector("up", nil); got != "up" {
		t.Errorf("Selector(no labels) = %q", got)
	}
	got := Selector("process_uptime_seconds", map[string]string{"service_name": "tyk-pump", "job": "tyk-pump"})
	want := `process_uptime_seconds{job="tyk-pump",service_name="tyk-pump"}`
	if got != want {
		t.Errorf("Selector() = %q, want %q", got, want)
	}
}

// TestClient drives the HTTP client against a fake Prometheus serving the
// fixtures, so the request paths and parameters are covered too.
func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/query" && r.URL.Query().Get("query") == "process_uptime_seconds":
			w.Write(fixture(t, "query_uptime.json"))
		case r.URL.Path == "/api/v1/query":
			w.Write(fixture(t, "query_empty.json"))
		case r.URL.Path == "/api/v1/metadata" && r.URL.Query().Get("metric") == "process_uptime_seconds":
			w.Write(fixture(t, "metadata.json"))
		case r.URL.Path == "/api/v1/metadata":
			w.Write(fixture(t, "metadata_empty.json"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	series, err := c.Query(ctx, "process_uptime_seconds")
	if err != nil || len(series) != 1 || series[0].Value != 42.5 {
		t.Fatalf("Query = %v, %v", series, err)
	}
	none, err := c.Query(ctx, "does_not_exist")
	if err != nil || len(none) != 0 {
		t.Fatalf("Query(miss) = %v, %v", none, err)
	}

	md, ok, err := c.Metadata(ctx, "process_uptime_seconds")
	if err != nil || !ok || md.Type != "gauge" {
		t.Fatalf("Metadata = %+v, %v, %v", md, ok, err)
	}
	if _, ok, err := c.Metadata(ctx, "does_not_exist"); err != nil || ok {
		t.Fatalf("Metadata(miss) ok=%v err=%v", ok, err)
	}

	got := WaitForSeries(t, c, "process_uptime_seconds", 2*time.Second, 10*time.Millisecond)
	if len(got) != 1 {
		t.Fatalf("WaitForSeries returned %d series", len(got))
	}

	// WaitForValue: the fixture reads 42.5, so that value is returned at once
	// while any other value, or no series at all, times out with the reason.
	if v := WaitForValue(t, c, "process_uptime_seconds", 42.5, 2*time.Second, 10*time.Millisecond); len(v) != 1 {
		t.Fatalf("WaitForValue returned %d series", len(v))
	}
	ft := &fakeT{}
	WaitForValue(ft, c, "process_uptime_seconds", 1, 30*time.Millisecond, 5*time.Millisecond)
	if !ft.failed || !strings.Contains(ft.msg, "want 1") {
		t.Fatalf("WaitForValue must time out while a series differs: failed=%v msg=%q", ft.failed, ft.msg)
	}
	ft = &fakeT{}
	WaitForValue(ft, c, "does_not_exist", 1, 30*time.Millisecond, 5*time.Millisecond)
	if !ft.failed || !strings.Contains(ft.msg, "no series") {
		t.Fatalf("WaitForValue must time out on no series: failed=%v msg=%q", ft.failed, ft.msg)
	}
}

func TestEventually_ReportsLastReason(t *testing.T) {
	ft := &fakeT{}
	Eventually(ft, 30*time.Millisecond, 5*time.Millisecond, func(context.Context) (bool, string) {
		return false, "still waiting for X"
	})
	if !ft.failed || !strings.Contains(ft.msg, "still waiting for X") {
		t.Fatalf("Eventually did not fail with the last reason: failed=%v msg=%q", ft.failed, ft.msg)
	}
}

// fakeT captures Fatalf so Eventually's failure path can be asserted.
type fakeT struct {
	testing.TB
	failed bool
	msg    string
}

func (f *fakeT) Helper() {}
func (f *fakeT) Fatalf(format string, args ...any) {
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
}

// TestFamilySeriesName pins how each family type is addressed in series
// queries: histograms only exist as _bucket/_sum/_count series in Prometheus.
func TestFamilySeriesName(t *testing.T) {
	gauge := Family{Name: "process_uptime_seconds", Type: "gauge"}
	if got := gauge.SeriesName(); got != "process_uptime_seconds" {
		t.Errorf("gauge SeriesName = %q, want the family name", got)
	}
	hist := Family{Name: "http_server_request_duration_seconds", Type: "histogram"}
	if got := hist.SeriesName(); got != "http_server_request_duration_seconds_count" {
		t.Errorf("histogram SeriesName = %q, want _count", got)
	}
}

// TestFamiliesRegistry keeps the registry itself honest: unique names, a known
// Prometheus type, and the identity labels on every family.
func TestFamiliesRegistry(t *testing.T) {
	if len(Families) == 0 {
		t.Fatal("the registry is empty")
	}
	seen := map[string]bool{}
	for _, f := range Families {
		if seen[f.Name] {
			t.Errorf("%s is registered twice", f.Name)
		}
		seen[f.Name] = true
		switch f.Type {
		case "gauge", "counter", "histogram":
		default:
			t.Errorf("%s has unknown type %q", f.Name, f.Type)
		}
		if f.OTelName == "" {
			t.Errorf("%s has no OTelName", f.Name)
		}
		have := map[string]bool{}
		for _, l := range f.Labels {
			have[l] = true
		}
		for _, l := range IdentityLabels {
			if !have[l] {
				t.Errorf("%s does not require identity label %s", f.Name, l)
			}
		}
	}
}
