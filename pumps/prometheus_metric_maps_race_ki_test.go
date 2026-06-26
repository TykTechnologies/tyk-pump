//go:build race

package pumps

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verifies: SW-REQ-024
// Verifies: KI:prometheus-metric-maps-race
// Reproduces: prometheus-metric-maps-race
func TestPrometheusMetric_MapsRace_KI(t *testing.T) {
	if os.Getenv("TYK_PUMP_PROMETHEUS_MAPS_RACE_CHILD") == "1" {
		runPrometheusMetricMapsRaceChild(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestPrometheusMetric_MapsRace_KI$")
	cmd.Env = append(os.Environ(), "TYK_PUMP_PROMETHEUS_MAPS_RACE_CHILD=1")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "KI active: race-instrumented child should report PrometheusMetric map races")
	text := string(output)
	assert.Contains(t, text, "DATA RACE", "child output should contain race detector output:\n%s", text)
	assert.Contains(t, text, "prometheus.go", "race should originate in Prometheus metric map code:\n%s", text)
}

func runPrometheusMetricMapsRaceChild(t *testing.T) {
	t.Helper()

	counterMetric := &PrometheusMetric{
		MetricType: counterType,
		counterMap: make(map[string]counterStruct),
	}
	histogramMetric := &PrometheusMetric{
		MetricType:             histogramType,
		histogramMap:           make(map[string]histogramCounter),
		aggregatedObservations: true,
	}

	const workers = 8
	const iterations = 200
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				key := fmt.Sprintf("api-%d", j%4)
				_ = counterMetric.Inc(key, "200")
				_ = histogramMetric.Observe(int64(j+1), "total", key, "200")
			}
		}(i)
	}
	close(start)
	wg.Wait()
}
