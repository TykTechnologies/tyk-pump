package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/ci/tests/metrics/promtest"
)

// uptimeFamily is process.uptime as Prometheus names it (unit "s" → _seconds).
const uptimeFamily = "process_uptime_seconds"

// TestUptime covers the liveness gauge: it reaches the collector on every
// export cycle, is a gauge, and grows with the process' age.
func TestUptime(t *testing.T) {
	requireProfile(t, "default")
	waitForPump(t)

	selector := pumpSelector(uptimeFamily)

	// Stopwatch starts before the first read so elapsed also covers v1's age.
	start := time.Now()
	first := promtest.WaitForSeries(t, prom, selector, pollTimeout, pollInterval)
	if len(first) != 1 {
		t.Fatalf("expected exactly one %s series for one Pump process, got %d: %v", uptimeFamily, len(first), first)
	}
	if first[0].Value <= 0 {
		t.Fatalf("%s = %v, want > 0", uptimeFamily, first[0].Value)
	}

	md := promtest.WaitForMetadata(t, prom, uptimeFamily, pollTimeout, pollInterval)
	if md.Type != "gauge" {
		t.Errorf("%s type = %q, want gauge", uptimeFamily, md.Type)
	}

	// A later export cycle must have advanced the observable gauge. Poll until
	// it does; a single stale sample only triggers a re-check.
	v1 := first[0].Value
	var v2 float64
	promtest.Eventually(t, pollTimeout, pollInterval, func(ctx context.Context) (bool, string) {
		series, err := prom.Query(ctx, selector)
		if err != nil {
			return false, err.Error()
		}
		if len(series) != 1 {
			return false, fmt.Sprintf("expected 1 series, got %d", len(series))
		}
		if series[0].Value <= v1 {
			return false, fmt.Sprintf("uptime has not advanced yet (%.1f)", series[0].Value)
		}
		v2 = series[0].Value
		return true, ""
	})

	// Sanity-check the gauge really is process age in seconds: the growth must
	// be consistent with wall-clock time. Both samples can be up to
	// sampleStaleness old in opposite directions, so allow twice that.
	elapsed := time.Since(start).Seconds()
	delta := v2 - v1
	if maxDelta := elapsed + 2*sampleStaleness.Seconds(); delta > maxDelta {
		t.Errorf("%s advanced by %.1fs while only %.1fs passed (allowed up to %.1fs)", uptimeFamily, delta, elapsed, maxDelta)
	}
	t.Logf("%s advanced %.1f → %.1f over %.1fs", uptimeFamily, v1, v2, elapsed)
}
