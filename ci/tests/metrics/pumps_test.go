package metrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/ci/tests/metrics/promtest"
)

// The pump write metrics families, as Prometheus names them.
//
//nolint:gosec // G101 false positive: metric names, not credentials.
const (
	pumpInitializedFamily  = "tyk_pump_initialized"
	pumpWritesFamily       = "tyk_pump_writes_total"
	pumpWriteRecordsFamily = "tyk_pump_write_records_total"
	purgeRecordsFamily     = "tyk_pump_purge_records_total"
)

// pumpFamilies is every family registered for the pumps profile, so a family
// added to promtest.Families is covered by the label and disabled checks too.
var pumpFamilies = func() []string {
	var names []string
	for _, f := range promtest.Families {
		if f.Profile == promtest.PumpsProfile {
			names = append(names, f.Name)
		}
	}
	return names
}()

// writingPumps are the pumps the pumps profile configures that initialise,
// and so are handed every purged batch. "dummy" comes from the base config.
var writingPumps = []string{"dummy", "filtered", "splunkdown", "mongoslow"}

// pumpSnapshot is one consistent reading of the write metrics.
type pumpSnapshot struct {
	purge   map[string]float64 // by result
	writes  map[string]float64 // by pump/outcome
	records map[string]float64 // by pump/outcome
}

// waitForReconciled waits until the purge loop has read exactly purged
// records and every writing pump's records, summed over outcomes, equal that
// total: the reconciliation every pump must satisfy once traffic stops. It
// returns that reading.
func waitForReconciled(t *testing.T, purged float64, pumps []string) pumpSnapshot {
	t.Helper()
	var snap pumpSnapshot
	promtest.Eventually(t, pollTimeout, pollInterval, func(ctx context.Context) (bool, string) {
		purge, err := points(ctx, purgeRecordsFamily, "result")
		if err != nil {
			return false, err.Error()
		}
		records, err := points(ctx, pumpWriteRecordsFamily, "pump", "outcome")
		if err != nil {
			return false, err.Error()
		}
		writes, err := points(ctx, pumpWritesFamily, "pump", "outcome")
		if err != nil {
			return false, err.Error()
		}
		total := 0.0
		for _, v := range purge {
			total += v
		}
		if total != purged {
			return false, fmt.Sprintf("the purge loop read %v records, want %v (%v)", total, purged, purge)
		}
		perPump := map[string]float64{}
		for k, v := range records {
			perPump[strings.SplitN(k, "/", 2)[0]] += v
		}
		for _, p := range pumps {
			if perPump[p] != total {
				return false, fmt.Sprintf("pump %s accounts for %v records across outcomes, the purge loop read %v (%v)", p, perPump[p], total, records)
			}
		}
		snap = pumpSnapshot{purge: purge, writes: writes, records: records}
		return true, ""
	})
	return snap
}

// writesOf returns the total writes recorded for pump, over every outcome.
func (s pumpSnapshot) writesOf(pump string) float64 {
	total := 0.0
	for k, v := range s.writes {
		if strings.HasPrefix(k, pump+"/") {
			total += v
		}
	}
	return total
}

// expectValue fails when m[key] != want.
func expectValue(t *testing.T, what string, m map[string]float64, key string, want float64) {
	t.Helper()
	if got := m[key]; got != want {
		t.Errorf("%s{%s} = %v, want %v (all: %v)", what, key, got, want, m)
	}
}

// TestPumpWriteMetrics runs under the "pumps" profile. The Gateway's real
// analytics feed one pump per outcome, and every step checks the outcome
// series, the reconciliation with the purge loop, and the label bounds.
func TestPumpWriteMetrics(t *testing.T) {
	requireProfile(t, promtest.PumpsProfile)
	waitForPump(t)
	waitForGateway(t)

	t.Run("init state", func(t *testing.T) {
		waitForPoints(t, pumpInitializedFamily, map[string]float64{
			"dummy/dummy":           1,
			"filtered/dummy":        1,
			"splunkdown/splunk":     1,
			"mongoslow/mongo":       1,
			"splunknoinit/splunk":   0,
			"notregistered/unknown": 0,
		}, "pump", "pump_type")
	})

	// Batch 1: only api1, which the filtered pump skips. Mongo is healthy.
	// Batch 2 is sent while Mongo is paused.
	const batch1, batch2 = 20, 10
	sendTraffic(t, api1Path, "first", batch1)
	snap := waitForReconciled(t, batch1, writingPumps)
	t.Logf("after batch 1: writes %v, records %v", snap.writes, snap.records)

	t.Run("decoded", func(t *testing.T) {
		expectValue(t, purgeRecordsFamily, snap.purge, "decoded", batch1)
		expectValue(t, purgeRecordsFamily, snap.purge, "decode_failed", 0)
	})

	t.Run("success", func(t *testing.T) {
		expectValue(t, pumpWriteRecordsFamily, snap.records, "dummy/success", batch1)
		expectValue(t, pumpWriteRecordsFamily, snap.records, "mongoslow/success", batch1)
		if snap.writes["dummy/success"] < 1 || snap.writes["dummy/success"] != snap.writesOf("dummy") {
			t.Errorf("the dummy pump must only record successful writes: %v", snap.writes)
		}
	})

	t.Run("error", func(t *testing.T) {
		// Every purge with records is an error for splunkdown, while the
		// healthy pumps in the same Pump keep succeeding.
		expectValue(t, pumpWriteRecordsFamily, snap.records, "splunkdown/error", batch1)
		expectValue(t, pumpWritesFamily, snap.writes, "splunkdown/error", snap.writesOf("dummy"))
		expectValue(t, pumpWritesFamily, snap.writes, "splunkdown/success", 0)
	})

	t.Run("filtered", func(t *testing.T) {
		expectValue(t, pumpWriteRecordsFamily, snap.records, "filtered/filtered", batch1)
		expectValue(t, pumpWriteRecordsFamily, snap.records, "filtered/success", 0)
		// The write still happens, with nothing left to write.
		expectValue(t, pumpWritesFamily, snap.writes, "filtered/success", snap.writesOf("dummy"))
	})

	t.Run("families", checkFamilies)

	t.Run("timeout", func(t *testing.T) {
		compose(t, "pause", "mongo")
		paused := true
		t.Cleanup(func() {
			if paused {
				compose(t, "unpause", "mongo")
			}
		})

		sendTraffic(t, api1Path, "paused", batch2)
		during := waitForReconciled(t, batch1+batch2, writingPumps)
		expectValue(t, pumpWriteRecordsFamily, during.records, "mongoslow/timeout", batch2)
		expectValue(t, pumpWriteRecordsFamily, during.records, "mongoslow/success", batch1)
		if during.writes["mongoslow/timeout"] < 1 {
			t.Errorf("no timed-out write recorded for mongoslow: %v", during.writes)
		}
		// The slow pump does not change the others' outcomes.
		expectValue(t, pumpWriteRecordsFamily, during.records, "dummy/success", batch1+batch2)

		// Resume Mongo: the abandoned writes now return, and nothing more may
		// be recorded for them.
		compose(t, "unpause", "mongo")
		paused = false
		time.Sleep(2 * sampleStaleness)
		after := waitForReconciled(t, batch1+batch2, writingPumps)
		for _, outcome := range []string{"success", "error", "timeout"} {
			key := "mongoslow/" + outcome
			if after.writes[key] != during.writes[key] || after.records[key] != during.records[key] {
				t.Errorf("mongoslow %s changed after the backend resumed: writes %v → %v, records %v → %v",
					outcome, during.writes[key], after.writes[key], during.records[key], after.records[key])
			}
		}
	})

	// Wait until the mongo pump writes successfully again, so the label check
	// below sees steady outcomes. Each probe is also api2 traffic, which the
	// filtered pump passes through.
	purged := waitForMongoRecovery(t, batch1+batch2)

	t.Run("bounded labels", func(t *testing.T) {
		before := pumpSeries(t)

		// Different APIs, orgs, paths and keys on every request.
		sendTraffic(t, api1Path, "replay-a", 15)
		sendTraffic(t, api2Path, "replay-b", 15)
		purged += 30
		waitForReconciled(t, purged, writingPumps)

		after := pumpSeries(t)
		if len(after) != len(before) {
			t.Errorf("the series count changed after replaying traffic: %d → %d\nbefore: %v\nafter:  %v", len(before), len(after), before, after)
		}
		if len(after) >= 100 {
			t.Errorf("%d tyk_pump_* series, want < 100", len(after))
		}
		checkLabelValues(t)
	})
}

// waitForMongoRecovery sends small probe batches to api2 until the mongoslow
// pump records a successful write again, and returns the new purge total.
func waitForMongoRecovery(t *testing.T, purged float64) float64 {
	t.Helper()
	before := mustPoints(t, pumpWriteRecordsFamily, "pump", "outcome")["mongoslow/success"]
	for i := 0; i < 10; i++ {
		sendTraffic(t, api2Path, fmt.Sprintf("probe-%d", i), 5)
		purged += 5
		snap := waitForReconciled(t, purged, writingPumps)
		if snap.records["mongoslow/success"] > before {
			return purged
		}
	}
	t.Fatal("the mongo pump never wrote successfully after Mongo resumed")
	return purged
}

// pumpSeries returns every tyk_pump_* series the Pump exports, rendered.
func pumpSeries(t *testing.T) []string {
	t.Helper()
	series, err := prom.Query(context.Background(), `{__name__=~"tyk_pump_.*",service_name="`+promtest.ServiceName+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(series))
	for _, s := range series {
		out = append(out, s.LabelsString())
	}
	sort.Strings(out)
	return out
}

// checkLabelValues asserts every pump label holds only configured pump names,
// registry keys, "unknown" and the fixed outcome/result sets: never GetName()
// strings or anything taken from records.
func checkLabelValues(t *testing.T) {
	t.Helper()
	allowed := map[string]map[string]bool{
		"pump":      set("dummy", "filtered", "splunkdown", "mongoslow", "splunknoinit", "notregistered"),
		"pump_type": set("dummy", "splunk", "mongo", "unknown"),
		"outcome":   set("success", "error", "timeout", "filtered"),
		"result":    set("decoded", "decode_failed"),
	}
	for _, family := range pumpFamilies {
		series, err := prom.Query(context.Background(), pumpSelector(family))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range series {
			for label, values := range allowed {
				if v, ok := s.Labels[label]; ok && !values[v] {
					t.Errorf("%s has %s=%q, which is not a configured or fixed value", family, label, v)
				}
			}
		}
	}
}

func set(values ...string) map[string]bool {
	m := make(map[string]bool, len(values))
	for _, v := range values {
		m[v] = true
	}
	return m
}
