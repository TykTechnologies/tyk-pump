package metrics

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// initFailureWarning is the one line the Pump logs when metrics are enabled
// but the provider cannot be built.
const initFailureWarning = "OpenTelemetry metrics disabled: provider initialization failed"

// purgeLoopLogRe matches the line logged once the Pump has initialised its
// pumps and entered the purge loop.
var purgeLoopLogRe = regexp.MustCompile(`Starting purge loop @\d+`)

// TestMisconfigured_BootsWithoutExport runs under the "misconfigured"
// profile, where metrics are enabled with an unknown exporter. A broken
// metrics config must never block boot: the Pump serves /health, initialises
// its pumps and purges, logs exactly one warning naming the problem, and
// exports nothing.
func TestMisconfigured_BootsWithoutExport(t *testing.T) {
	requireProfile(t, "misconfigured")
	waitForPump(t)

	resp, err := httpClient.Get(pumpHealthURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health = %s, want 200", resp.Status)
	}

	waitForLog(t, purgeLoopLogRe)

	logs := pumpLogs(t)
	if !strings.Contains(logs, "Dummy Initialized") {
		t.Error("the dummy pump was not initialised")
	}
	warnings := linesContaining(logs, initFailureWarning)
	if len(warnings) != 1 {
		t.Fatalf("want exactly one %q line, got %d:\n%s", initFailureWarning, len(warnings), strings.Join(warnings, "\n"))
	}
	if !strings.Contains(warnings[0], "bogus") {
		t.Errorf("the warning must name the problem (the bogus exporter): %s", warnings[0])
	}
	if n := len(linesContaining(logs, "OpenTelemetry metrics enabled")); n != 0 {
		t.Errorf("metrics failed to initialise, but the Pump logged that export is enabled %d times", n)
	}

	waitForCollectorScrape(t)
	time.Sleep(3 * exportInterval)
	assertNoPumpSeries(t, "metrics misconfigured")
}
