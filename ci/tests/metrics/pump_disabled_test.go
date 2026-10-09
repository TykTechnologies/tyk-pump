package metrics

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/ci/tests/metrics/promtest"
)

// dummyWriteLogRe matches the dummy pump's log line for a non-empty write.
var dummyWriteLogRe = regexp.MustCompile(`Writing [1-9]\d* records`)

// TestPumpMetricsDisabled runs under the "pump-disabled" profile: metrics are
// on and pump_metrics is false. While the Pump moves real traffic,
// process.uptime and tyk.pump.health keep flowing and none of the pump write
// families is exported.
func TestPumpMetricsDisabled(t *testing.T) {
	requireProfile(t, "pump-disabled")
	waitForPump(t)
	waitForGateway(t)

	sendTraffic(t, api1Path, "disabled", 10)
	// Proves the purge loop read and wrote the records.
	waitForLog(t, dummyWriteLogRe)

	promtest.WaitForSeries(t, prom, pumpSelector(uptimeFamily), pollTimeout, pollInterval)
	waitForHealthy(t, map[string]string{"temporal_storage": "redis"})
	// Give the Pump several export intervals to misbehave.
	time.Sleep(3 * exportInterval)

	ctx := context.Background()
	for _, family := range pumpFamilies {
		series, err := prom.Query(ctx, pumpSelector(family))
		if err != nil {
			t.Fatal(err)
		}
		if len(series) != 0 {
			t.Errorf("pump_metrics is false, but %s has %d series, e.g. %s", family, len(series), series[0].LabelsString())
		}
		if _, ok, err := prom.Metadata(ctx, family); err != nil {
			t.Fatal(err)
		} else if ok {
			t.Errorf("pump_metrics is false, but Prometheus has metadata for %s", family)
		}
	}
}
