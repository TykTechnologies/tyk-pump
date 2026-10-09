package metrics

import (
	"context"
	"strings"
	"testing"

	"github.com/TykTechnologies/tyk-pump/ci/tests/metrics/promtest"
)

// TestFamilies checks every registered metric family for exact name, type and
// required label keys. A family that is renamed, dropped, or loses a label
// fails here with a message naming it; per-family files cover behaviour.
// Families that need traffic are checked by their profile's test through
// checkFamilies instead.
func TestFamilies(t *testing.T) {
	requireProfile(t, "default")
	waitForPump(t)

	checkFamilies(t)
}

// checkFamilies runs the family check for every family registered under the
// active profile. Call it once the stack has exported them.
func checkFamilies(t *testing.T) {
	t.Helper()
	checked := 0
	for _, f := range promtest.Families {
		if !f.CheckedIn(profile()) {
			continue
		}
		checked++
		t.Run(f.Name, func(t *testing.T) {
			// Wait for the first export before judging; after that the check
			// is a snapshot.
			promtest.WaitForSeries(t, prom, pumpSelector(f.SeriesName()), pollTimeout, pollInterval)
			md := promtest.WaitForMetadata(t, prom, f.Name, pollTimeout, pollInterval)

			series, err := prom.Query(context.Background(), pumpSelector(f.SeriesName()))
			if err != nil {
				t.Fatal(err)
			}
			if problems := f.Check(md, true, series); len(problems) > 0 {
				t.Fatalf("metric family %s does not match its registration:\n  %s", f.Name, strings.Join(problems, "\n  "))
			}
			t.Logf("%s (%s) OK: type=%s, %d series, labels %v", f.Name, f.OTelName, md.Type, len(series), series[0].LabelKeys())
		})
	}
	if checked == 0 {
		t.Fatalf("no metric family is registered for the %q profile", profile())
	}
}
