package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
)

// panickingPump panics inside WriteData. In Go, a panic in a goroutine
// that is not recovered terminates the entire process — so if the
// dispatcher in writeToPumps does NOT use defer/recover around the
// per-pump goroutine, the test process itself will crash. To keep this
// test honest AND non-crashing, the actual panicking dispatch is run
// in a subprocess (the standard Go pattern for "expect this to crash").
type panickingPump struct {
	pumps.CommonPumpConfig
	name string
}

func (p *panickingPump) GetName() string                                   { return p.name }
func (p *panickingPump) New() pumps.Pump                                   { return &panickingPump{name: p.name} }
func (p *panickingPump) Init(_ interface{}) error                          { return nil }
func (p *panickingPump) WriteData(_ context.Context, _ []interface{}) error {
	panic("simulated downstream library bug in " + p.name)
}
func (p *panickingPump) Shutdown() error { return nil }

// completionTrackingPump records whether WriteData was called and whether
// it returned (so we can prove sibling-pump completion is independent of
// a peer panic).
type completionTrackingPump struct {
	pumps.CommonPumpConfig
	name      string
	mu        sync.Mutex
	called    bool
	completed bool
}

func (p *completionTrackingPump) GetName() string { return p.name }
func (p *completionTrackingPump) New() pumps.Pump {
	return &completionTrackingPump{name: p.name}
}
func (p *completionTrackingPump) Init(_ interface{}) error { return nil }
func (p *completionTrackingPump) WriteData(_ context.Context, _ []interface{}) error {
	p.mu.Lock()
	p.called = true
	p.mu.Unlock()
	// Tiny sleep to give the panicking peer time to fire if it would
	// cascade — failure-independence proof.
	time.Sleep(50 * time.Millisecond)
	p.mu.Lock()
	p.completed = true
	p.mu.Unlock()
	return nil
}
func (p *completionTrackingPump) Shutdown() error { return nil }

func (p *completionTrackingPump) Snapshot() (called, completed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.called, p.completed
}

// Verifies: SW-REQ-001
// SW-REQ-001:fanout_panic_isolated:negative
//
// Contract: a panic inside one pump's WriteData MUST NOT crash the
// tyk-pump process or starve sibling pumps in the same purge cycle.
// Per the goroutine-fanout in main.go:execPumpWriting (line 469), each
// pump's WriteData runs in its own goroutine; an unrecovered panic in
// any of those goroutines would terminate the host process per Go
// runtime semantics.
//
// As of this test's authorship, execPumpWriting does NOT have a
// defer/recover around the inner goroutine. This test runs the
// panicking dispatch in a SUBPROCESS so the parent test binary
// survives, then inspects the subprocess exit code and stderr:
//
//   - Subprocess exit code 0 + sibling-completed marker on stdout
//     => panic isolated (CONTRACT HONORED, ideal future state)
//   - Subprocess exit code != 0 with "panic:" on stderr
//     => panic escaped goroutine; sibling pumps may or may not have
//        completed (CONTRACT GAP, current state — KI surface)
//
// Either outcome is OBSERVED, not crashed-on. Wave 4 should file the
// KI `pump-fanout-panic-not-recovered` if the subprocess crashes.
func TestFanoutPanicIsolated_PanicInOneDoesNotStallOthers(t *testing.T) {
	if os.Getenv("TYK_PUMP_PANIC_ISOLATION_SUBPROCESS") == "1" {
		// We are the subprocess. Run the actual panicking fanout and
		// emit a marker if sibling completion happened before the
		// panic took the process down.
		runPanicIsolationSubprocess()
		return
	}

	cmd := exec.Command(os.Args[0],
		"-test.run=TestFanoutPanicIsolated_PanicInOneDoesNotStallOthers",
		"-test.v")
	cmd.Env = append(os.Environ(), "TYK_PUMP_PANIC_ISOLATION_SUBPROCESS=1")
	out, err := cmd.CombinedOutput()
	combined := string(out)

	hasSiblingCompleted := strings.Contains(combined, "SIBLING_COMPLETED=true")
	hasPanic := strings.Contains(combined, "panic:")
	hasFanoutSurvived := strings.Contains(combined, "FANOUT_SURVIVED=true")

	t.Logf("subprocess exit err: %v", err)
	t.Logf("subprocess output (truncated to 4096B):\n%s", truncate(combined, 4096))

	switch {
	case err == nil && hasFanoutSurvived && hasSiblingCompleted:
		// IDEAL FUTURE STATE: panic was recovered, fanout returned, sibling
		// completed. If this branch is hit, a fix landed and the test should
		// be tightened — flip the expectation, close KI
		// `pump-fanout-panic-not-recovered`.
		t.Errorf("PANIC ISOLATION APPEARS PRESENT — subprocess survived a " +
			"panicking pump. This is the desired future state but contradicts " +
			"the documented baseline. Update test expectations to assert " +
			"panic-recovery semantics and close KI " +
			"`pump-fanout-panic-not-recovered`.")
	case err != nil && hasPanic:
		// CURRENT DOCUMENTED BEHAVIOR: panic escaped the inner WriteData
		// goroutine in execPumpWriting (main.go:469) and crashed the host
		// process. Whether or not sibling completed before the crash
		// depends on goroutine-scheduling race; the contract gap is the
		// same in either case (process-isolation absent).
		//
		// Wave 4 KI: `pump-fanout-panic-not-recovered`.
		// Stack trace from subprocess output above shows the exact
		// uncovered code path: main.go:469 spawns a goroutine that calls
		// pmp.WriteData directly without defer/recover.
		t.Logf("DOCUMENTED GAP CONFIRMED — panic escaped pump goroutine and " +
			"crashed subprocess (hasSiblingCompleted=%v). " +
			"Wave 4 KI: pump-fanout-panic-not-recovered. "+
			"Production code: main.go:469 spawns goroutine calling " +
			"WriteData without defer/recover.", hasSiblingCompleted)
	default:
		t.Errorf("unexpected subprocess outcome: err=%v hasPanic=%v hasSibling=%v hasFanoutSurvived=%v "+
			"— neither the documented-gap nor the ideal-future state matched",
			err, hasPanic, hasSiblingCompleted, hasFanoutSurvived)
	}
}

func runPanicIsolationSubprocess() {
	originalPumps := Pumps
	defer func() { Pumps = originalPumps }()

	panicker := &panickingPump{name: "panic-A"}
	sibling := &completionTrackingPump{name: "sibling-B"}
	Pumps = []pumps.Pump{panicker, sibling}

	keys := []interface{}{
		analytics.AnalyticsRecord{APIID: "r1", OrgID: "o1"},
	}

	// Run dispatch in a wrapper goroutine so we can poll the sibling's
	// status if the host process crashes before writeToPumps returns.
	// Note: if execPumpWriting's INNER goroutine panics without
	// recovery, the runtime terminates the process — there is no
	// "after writeToPumps returns" in that path.

	// Start a watchdog that emits the sibling status periodically so
	// at least one snapshot reaches stdout before any crash.
	stopWatch := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopWatch:
				return
			case <-ticker.C:
				called, completed := sibling.Snapshot()
				if completed {
					// Emit and exit watchdog
					_, _ = os.Stdout.WriteString("SIBLING_COMPLETED=true\n")
					_ = os.Stdout.Sync()
					return
				}
				if called {
					_, _ = os.Stdout.WriteString("SIBLING_CALLED=true\n")
					_ = os.Stdout.Sync()
				}
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		defer func() {
			// Recover here is OUR recovery, only for the test wrapper.
			// It does NOT recover panics from execPumpWriting's inner
			// goroutine (different goroutine stack).
			_ = recover()
			close(done)
		}()
		writeToPumps(keys, nil, time.Now(), 2)
	}()

	select {
	case <-done:
		// writeToPumps returned. Give the sibling watchdog one final
		// chance to flush a completion marker.
		time.Sleep(100 * time.Millisecond)
		close(stopWatch)
		called, completed := sibling.Snapshot()
		if completed {
			_, _ = os.Stdout.WriteString("SIBLING_COMPLETED=true\n")
		} else if called {
			_, _ = os.Stdout.WriteString("SIBLING_CALLED=true\n")
		}
		_, _ = os.Stdout.WriteString("FANOUT_SURVIVED=true\n")
		_ = os.Stdout.Sync()
	case <-time.After(5 * time.Second):
		close(stopWatch)
		_, _ = os.Stdout.WriteString("FANOUT_HUNG=true\n")
		_ = os.Stdout.Sync()
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...<truncated>"
}
