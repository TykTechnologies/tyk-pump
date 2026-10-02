package metrics

import (
	"testing"
	"time"
)

// TestDisabled_NoExport runs under the "disabled" profile, where a collector
// endpoint is configured but opentelemetry.metrics.enabled=false. The Pump
// must serve normally and nothing from it may reach Prometheus.
func TestDisabled_NoExport(t *testing.T) {
	requireProfile(t, "disabled")
	waitForPump(t)

	// Prometheus must be scraping the collector, otherwise "no series" would
	// prove nothing.
	waitForCollectorScrape(t)

	// Give the Pump several export intervals to misbehave.
	time.Sleep(3 * exportInterval)

	assertNoPumpSeries(t, "metrics disabled")

	if n := len(linesContaining(pumpLogs(t), "OpenTelemetry metrics enabled")); n != 0 {
		t.Errorf("metrics disabled, but the Pump logged that export is enabled %d times", n)
	}
}
