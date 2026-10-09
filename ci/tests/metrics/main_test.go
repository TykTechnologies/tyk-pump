// Package metrics is the Pump's end-to-end OpenTelemetry metrics suite. It
// runs against the docker-compose stack in this directory (see Taskfile.yml)
// and asserts, through Prometheus, what the Pump actually exports over OTLP.
//
// One file per metric family / concern: add a new *_test.go for each signal
// story instead of growing a monolith, plus a promtest.Families entry.
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/ci/tests/metrics/promtest"
)

const (
	// exportInterval mirrors TYK_PMP_OPENTELEMETRY_METRICS_EXPORTINTERVAL in
	// the profiles; polls and "quiet period" waits are derived from it.
	exportInterval = 2 * time.Second
	// scrapeInterval mirrors global.scrape_interval in configs/prometheus.
	scrapeInterval = 2 * time.Second
	// sampleStaleness is how old a value read from Prometheus can be: one
	// export cycle to reach the collector plus one scrape to reach Prometheus.
	sampleStaleness = exportInterval + scrapeInterval
	pollTimeout     = 60 * time.Second
	pollInterval    = time.Second
)

var (
	// Both URLs default to the ports docker-compose publishes, so `go test`
	// works without the Taskfile.
	pumpHealthURL = envOr("PUMP_HEALTH_URL", "http://localhost:"+envOr("PUMP_HEALTH_PORT", "8083")+"/health")
	prometheusURL = envOr("PROMETHEUS_URL", "http://localhost:"+envOr("PROMETHEUS_PORT", "9090"))
	prom          = promtest.NewClient(prometheusURL)

	httpClient = &http.Client{Timeout: 10 * time.Second}
)

func TestMain(m *testing.M) {
	// Guard: only run when the stack is up and explicitly requested, so a
	// plain `go test ./...` in this module stays a unit-test run.
	if os.Getenv("E2E_METRICS") == "" {
		fmt.Println("skipping e2e metrics tests (set E2E_METRICS=1 and bring the stack up with `task setup`)")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// profile returns the active PUMP_PROFILE, defaulting to "default".
func profile() string {
	return envOr("PUMP_PROFILE", "default")
}

// requireProfile skips the test unless the stack runs the given profile.
func requireProfile(t *testing.T, want string) {
	t.Helper()
	if p := profile(); p != want {
		t.Skipf("only runs under the %q profile (current: %q)", want, p)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// waitForPump blocks until the Pump answers /health with 200.
func waitForPump(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(pollTimeout)
	var last string
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(pumpHealthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			last = resp.Status
		} else {
			last = err.Error()
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("pump at %s not ready: %s", pumpHealthURL, last)
}

// pumpSelector is the PromQL selector for the family restricted to series
// exported by the Pump.
func pumpSelector(family string) string {
	return promtest.Selector(family, map[string]string{"service_name": promtest.ServiceName})
}

// targetInfoSelector selects the Pump's target_info series. The collector's
// Prometheus exporter maps service.name → job and service.instance.id →
// instance, and prometheus.yml sets honor_labels so those OTel identities are
// kept as-is.
func targetInfoSelector() string {
	return promtest.Selector("target_info", map[string]string{"job": promtest.ServiceName})
}

// compose runs a docker compose command against the profile's stack and
// returns its combined output.
func compose(t *testing.T, args ...string) string {
	t.Helper()
	project := envOr("PUMP_COMPOSE_PROJECT", "pump-metrics-"+profile())
	cmd := exec.Command("docker", append([]string{"compose", "-p", project}, args...)...) //nolint:gosec // args come from the suite, not from input
	cmd.Env = append(os.Environ(), "PUMP_PROFILE="+profile(), "COMPOSE_PROFILES="+profile())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// pumpLogs returns the Pump container's log so far.
func pumpLogs(t *testing.T) string {
	t.Helper()
	return compose(t, "logs", "--no-color", "--no-log-prefix", "tyk-pump")
}

// waitForLog polls the Pump log until re matches and returns the match.
func waitForLog(t *testing.T, re *regexp.Regexp) []string {
	t.Helper()
	var match []string
	promtest.Eventually(t, pollTimeout, pollInterval, func(context.Context) (bool, string) {
		match = re.FindStringSubmatch(pumpLogs(t))
		if match == nil {
			return false, "no log line matches " + re.String()
		}
		return true, ""
	})
	return match
}

// assertNoPumpSeries asserts that nothing the Pump could have exported reached
// Prometheus. Call it after the pipeline is proven alive (the collector is
// scraped) and a few export intervals have passed, otherwise absence proves
// nothing.
func assertNoPumpSeries(t *testing.T, why string) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		targetInfoSelector(),
		`{service_name="` + promtest.ServiceName + `"}`,
		`{job="` + promtest.ServiceName + `"}`,
	} {
		series, err := prom.Query(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(series) != 0 {
			t.Errorf("%s, but %s returned %d series, e.g. %s", why, q, len(series), series[0].LabelsString())
		}
	}
	for _, f := range promtest.Families {
		if _, ok, err := prom.Metadata(ctx, f.Name); err != nil {
			t.Fatal(err)
		} else if ok {
			t.Errorf("%s, but Prometheus has metadata for %s", why, f.Name)
		}
	}
}

// waitForCollectorScrape blocks until Prometheus scrapes the collector, so a
// later "no series" check means something.
func waitForCollectorScrape(t *testing.T) {
	t.Helper()
	promtest.WaitForValue(t, prom, `up{job="otel-collector"}`, 1, pollTimeout, pollInterval)
}

// linesContaining returns the lines of log that contain substr.
func linesContaining(log, substr string) []string {
	var lines []string
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, substr) {
			lines = append(lines, line)
		}
	}
	return lines
}
