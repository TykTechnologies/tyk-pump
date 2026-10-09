package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/ci/tests/metrics/promtest"
)

// probeRenewal mirrors healthcheck.DefaultRenewal: how long one probe round
// is served before the next one runs.
const probeRenewal = 10 * time.Second

// healthSelector selects the Pump's tyk_pump_health series, optionally
// narrowed to one component.
func healthSelector(component string) string {
	labels := map[string]string{"service_name": promtest.ServiceName}
	if component != "" {
		labels["component"] = component
	}
	return promtest.Selector(promtest.HealthFamily.Name, labels)
}

// waitForHealth waits until the component's health series reads want. Within
// one probe interval: 10s renewal + 2s probe timeout + export + scrape, well
// inside pollTimeout.
func waitForHealth(t *testing.T, component string, want float64) []promtest.Series {
	t.Helper()
	return promtest.WaitForValue(t, prom, healthSelector(component), want, pollTimeout, pollInterval)
}

// waitForHealthy waits until the Pump exports exactly the given
// component → store series, all reading 1, and returns them.
func waitForHealthy(t *testing.T, want map[string]string) []promtest.Series {
	t.Helper()
	var out []promtest.Series
	promtest.Eventually(t, pollTimeout, pollInterval, func(ctx context.Context) (bool, string) {
		series, err := prom.Query(ctx, healthSelector(""))
		if err != nil {
			return false, err.Error()
		}
		got := map[string]string{}
		for _, s := range series {
			if s.Value != 1 {
				return false, fmt.Sprintf("%s = %v, want 1", s.LabelsString(), s.Value)
			}
			got[s.Labels["component"]] = s.Labels["store"]
		}
		if !reflect.DeepEqual(got, want) || len(series) != len(want) {
			return false, fmt.Sprintf("health series %v, want %v", got, want)
		}
		out = series
		return true, ""
	})
	return out
}

// undoAction maps each breakDependency action to the docker command that
// reverts it.
var undoAction = map[string]string{"stop": "start", "pause": "unpause"}

// breakDependency runs `docker <action>` (stop or pause) on the service's
// container and returns restore, which undoes it once. restore is also a
// cleanup, so a failing assertion never leaves the stack broken for the next
// test. It goes through the container rather than `docker compose stop` so
// dependents (the Pump) are never touched.
func breakDependency(t *testing.T, action, service string) (restore func()) {
	t.Helper()
	id := strings.TrimSpace(compose(t, "ps", "-q", service))
	if id == "" {
		t.Fatalf("no running container for service %s", service)
	}
	run := func(action string) {
		if out, err := exec.Command("docker", action, id).CombinedOutput(); err != nil {
			t.Fatalf("docker %s %s: %v\n%s", action, service, err, out)
		}
	}
	run(action)
	var once sync.Once
	restore = func() { once.Do(func() { run(undoAction[action]) }) }
	t.Cleanup(restore)
	return restore
}

// assertLivenessUnchanged asserts /health still answers 200 {"status": "ok"}:
// the self-health gauge never makes the liveness endpoint deep.
func assertLivenessUnchanged(t *testing.T) {
	t.Helper()
	resp, err := httpClient.Get(pumpHealthURL)
	if err != nil {
		t.Fatalf("GET %s: %v", pumpHealthURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /health: %v", err)
	}
	var got map[string]string
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &got) != nil || got["status"] != "ok" {
		t.Errorf("/health = %d %s, want 200 {\"status\": \"ok\"}", resp.StatusCode, body)
	}
}

// assertExportsContinue asserts process_uptime_seconds keeps advancing: the
// export is not stalled by a broken dependency.
func assertExportsContinue(t *testing.T) {
	t.Helper()
	selector := pumpSelector(uptimeFamily)
	first := promtest.WaitForSeries(t, prom, selector, pollTimeout, pollInterval)
	promtest.Eventually(t, pollTimeout, pollInterval, func(ctx context.Context) (bool, string) {
		series, err := prom.Query(ctx, selector)
		if err != nil {
			return false, err.Error()
		}
		if len(series) != 1 || series[0].Value <= first[0].Value {
			return false, fmt.Sprintf("%s has not advanced past %.1f", uptimeFamily, first[0].Value)
		}
		return true, ""
	})
}

// TestHealth_Healthy: with Redis and the default Mongo uptime pump up, the
// Pump exports exactly two series, both 1, carrying only component and store
// on top of the labels every Pump series has.
func TestHealth_Healthy(t *testing.T) {
	requireProfile(t, "default")
	waitForPump(t)

	series := waitForHealthy(t, map[string]string{"temporal_storage": "redis", "uptime": "mongo"})

	// Same resource as process_uptime_seconds, so any extra key is a data
	// point attribute.
	base := promtest.WaitForSeries(t, prom, pumpSelector(uptimeFamily), pollTimeout, pollInterval)[0].LabelKeys()
	for _, s := range series {
		extra := difference(s.LabelKeys(), base)
		if fmt.Sprint(extra) != "[component store]" {
			t.Errorf("%s: labels beyond the resource are %v, want [component store]", s.LabelsString(), extra)
		}
		if missing := difference(base, s.LabelKeys()); len(missing) > 0 {
			t.Errorf("%s: missing resource labels %v", s.LabelsString(), missing)
		}
	}
	assertLivenessUnchanged(t)
}

// TestHealth_TemporalStorageDown: a stopped Redis reads 0 while the uptime
// store stays 1, the Pump keeps exporting and /health keeps answering ok;
// starting Redis brings it back to 1.
func TestHealth_TemporalStorageDown(t *testing.T) {
	requireProfile(t, "default")
	waitForPump(t)
	waitForHealthy(t, map[string]string{"temporal_storage": "redis", "uptime": "mongo"})

	restore := breakDependency(t, "stop", "redis")
	waitForHealth(t, "temporal_storage", 0)
	waitForHealth(t, "uptime", 1)
	assertExportsContinue(t)
	assertLivenessUnchanged(t)

	restore()
	waitForHealth(t, "temporal_storage", 1)
}

// TestHealth_TemporalStorageHung: a paused Redis (connections accepted, never
// answered) must not freeze the gauge or the export: the probe times out and
// reads 0 while process_uptime_seconds keeps arriving on schedule.
func TestHealth_TemporalStorageHung(t *testing.T) {
	requireProfile(t, "default")
	waitForPump(t)
	waitForHealthy(t, map[string]string{"temporal_storage": "redis", "uptime": "mongo"})

	restore := breakDependency(t, "pause", "redis")
	waitForHealth(t, "temporal_storage", 0)
	assertExportsContinue(t)
	assertLivenessUnchanged(t)

	restore()
	waitForHealth(t, "temporal_storage", 1)
}

// TestHealth_UptimeStoreDown: a stopped Mongo makes uptime read 0 while
// temporal_storage stays 1.
func TestHealth_UptimeStoreDown(t *testing.T) {
	requireProfile(t, "default")
	waitForPump(t)
	waitForHealthy(t, map[string]string{"temporal_storage": "redis", "uptime": "mongo"})

	restore := breakDependency(t, "stop", "mongo")
	waitForHealth(t, "uptime", 0)
	waitForHealth(t, "temporal_storage", 1)
	assertLivenessUnchanged(t)

	restore()
	waitForHealth(t, "uptime", 1)
}

// TestHealth_UptimeDisabled: with dont_purge_uptime_data=true there is no
// uptime pump, so its series is absent, never 0.
func TestHealth_UptimeDisabled(t *testing.T) {
	requireProfile(t, "uptime-disabled")
	waitForPump(t)

	waitForHealthy(t, map[string]string{"temporal_storage": "redis"})
	// A later probe round must not add an uptime series either.
	time.Sleep(probeRenewal + sampleStaleness)
	waitForHealthy(t, map[string]string{"temporal_storage": "redis"})
}

// TestHealth_SQLUptimeStore: the SQL uptime pump's series carries the live
// engine, and follows that engine's outage.
func TestHealth_SQLUptimeStore(t *testing.T) {
	requireProfile(t, "postgres")
	waitForPump(t)

	waitForHealthy(t, map[string]string{"temporal_storage": "redis", "uptime": "postgres"})

	restore := breakDependency(t, "stop", "postgres")
	waitForHealth(t, "uptime", 0)
	waitForHealth(t, "temporal_storage", 1)

	restore()
	waitForHealth(t, "uptime", 1)
}

// TestHealth_FamilyDisabled: health_metrics=false exports no tyk_pump_health
// while process_uptime_seconds keeps flowing.
func TestHealth_FamilyDisabled(t *testing.T) {
	requireProfile(t, "health-disabled")
	waitForPump(t)
	promtest.WaitForSeries(t, prom, pumpSelector(uptimeFamily), pollTimeout, pollInterval)

	// Long enough for a probe round to have been due and exported.
	time.Sleep(probeRenewal + sampleStaleness)

	ctx := context.Background()
	if series, err := prom.Query(ctx, healthSelector("")); err != nil {
		t.Fatal(err)
	} else if len(series) != 0 {
		t.Errorf("health_metrics=false, but %d %s series were exported, e.g. %s",
			len(series), promtest.HealthFamily.Name, series[0].LabelsString())
	}
	if _, ok, err := prom.Metadata(ctx, promtest.HealthFamily.Name); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Errorf("health_metrics=false, but Prometheus has metadata for %s", promtest.HealthFamily.Name)
	}
	assertLivenessUnchanged(t)
}

// difference returns the sorted keys in a that are not in b.
func difference(a, b []string) []string {
	in := map[string]bool{}
	for _, k := range b {
		in[k] = true
	}
	var out []string
	for _, k := range a {
		if !in[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
