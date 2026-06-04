package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
)

// countingFailingPump fails every WriteData call and counts how many times
// it was invoked. Used to observe whether the dispatcher re-invokes a
// repeatedly-failing pump on each purge cycle (no circuit breaker) or
// backs off after N consecutive failures (circuit breaker present).
type countingFailingPump struct {
	pumps.CommonPumpConfig
	name string
	mu   sync.Mutex
	n    int
}

func (p *countingFailingPump) GetName() string { return p.name }
func (p *countingFailingPump) New() pumps.Pump {
	return &countingFailingPump{name: p.name}
}
func (p *countingFailingPump) Init(_ interface{}) error { return nil }
func (p *countingFailingPump) WriteData(_ context.Context, _ []interface{}) error {
	p.mu.Lock()
	p.n++
	p.mu.Unlock()
	return errors.New("simulated persistent downstream failure")
}
func (p *countingFailingPump) Shutdown() error { return nil }

func (p *countingFailingPump) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.n
}

// stallingPump blocks WriteData until the supplied channel is closed. Used
// to verify that a slow/stalled pump does not stall sibling pumps in the
// same purge cycle (cascade-isolation property).
type stallingPump struct {
	pumps.CommonPumpConfig
	name    string
	release chan struct{}
	called  chan struct{}
	once    sync.Once
}

func (p *stallingPump) GetName() string { return p.name }
func (p *stallingPump) New() pumps.Pump { return &stallingPump{name: p.name} }
func (p *stallingPump) Init(_ interface{}) error { return nil }
func (p *stallingPump) WriteData(ctx context.Context, _ []interface{}) error {
	p.once.Do(func() { close(p.called) })
	select {
	case <-p.release:
		return errors.New("released with failure")
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *stallingPump) Shutdown() error { return nil }

// Verifies: SW-REQ-001 (cascade_circuit_breaker), SYS-REQ-004 (failure_independence_proven), SW-REQ-016
// SW-REQ-001:cascade_circuit_breaker:scenario
// SYS-REQ-004:failure_independence_proven:scenario
//
// Contract: when one pump fails (or stalls), the purge dispatcher MUST
// continue dispatching to healthy sibling pumps in the same cycle. The
// failure-independence proof is that healthy pumps receive every record
// even while a peer is failing or blocked.
//
// What this test exercises:
//   - 3 pumps wired into the global Pumps slice
//   - pump A: stalls until released (simulating a wedged backend)
//   - pumps B, C: healthy MockedPump
//   - run writeToPumps with a per-pump timeout shorter than the stall
//   - assert B and C completed all records BEFORE A's stall resolves
//
// If the dispatcher serialized pumps or shared state in a way that one
// stall blocks others, B/C counters would be 0 when we sample them
// after the timeout.
func TestCascadeCircuitBreaker_FailingPumpDoesNotStallOthers(t *testing.T) {
	originalPumps := Pumps
	t.Cleanup(func() { Pumps = originalPumps })

	stall := &stallingPump{
		name:    "stalling-A",
		release: make(chan struct{}),
		called:  make(chan struct{}),
	}
	// Give the stalling pump a short timeout so writeToPumps doesn't block
	// forever — the property under test is that B/C are not affected by
	// A's slowness, not that A finishes.
	stall.SetTimeout(1)

	healthyB := &MockedPump{}
	healthyC := &MockedPump{}

	Pumps = []pumps.Pump{stall, healthyB, healthyC}

	keys := []interface{}{
		analytics.AnalyticsRecord{APIID: "r1", OrgID: "o1"},
		analytics.AnalyticsRecord{APIID: "r2", OrgID: "o1"},
		analytics.AnalyticsRecord{APIID: "r3", OrgID: "o1"},
	}

	done := make(chan struct{})
	go func() {
		writeToPumps(keys, nil, time.Now(), 2)
		close(done)
	}()

	// Wait for the stalling pump to be invoked so we know the fanout
	// started, then verify B and C made progress without waiting for A.
	select {
	case <-stall.called:
	case <-time.After(3 * time.Second):
		t.Fatal("stalling pump was never invoked — fanout dispatch may be broken")
	}

	// writeToPumps must return within the stall's timeout window (1s)
	// without us needing to release the stall. Allow generous slack.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		close(stall.release)
		<-done
		t.Fatal("writeToPumps did not complete within timeout — " +
			"a stalled pump may be blocking sibling completion (cascade not isolated)")
	}

	// At this point A's timeout fired but B and C should have completed
	// all 3 records via independent goroutines.
	if healthyB.CounterRequest != 3 {
		t.Errorf("healthy pump B got %d records; want 3 — "+
			"stalled peer A starved B (failure NOT independent)", healthyB.CounterRequest)
	}
	if healthyC.CounterRequest != 3 {
		t.Errorf("healthy pump C got %d records; want 3 — "+
			"stalled peer A starved C (failure NOT independent)", healthyC.CounterRequest)
	}

	// Defensive: unblock the stalling goroutine so the test process exits clean.
	close(stall.release)
}

// Verifies: SW-REQ-001 (per_pump_circuit_breaker)
// SW-REQ-001:per_pump_circuit_breaker:scenario
//
// Contract surface: after N consecutive failures from one pump, an ideal
// dispatcher SHOULD skip it for a cooldown window. Tyk-pump currently
// has no such breaker — every purge cycle re-invokes every configured
// pump regardless of recent failure history.
//
// This test documents the CURRENT behavior: 100 cycles == 100 invocations.
// If a per-pump circuit breaker is introduced in the future, this test
// will FAIL with invocations < 100, at which point the suite owner should:
//   1. Update the expectation to assert breaker semantics.
//   2. Close the KI `pump-no-per-pump-circuit-breaker` documenting the gap.
//
// Either outcome is valuable — this test pins down the contract as
// "always re-invoked" today, and any future regression toward (or
// away from) a breaker is observable.
func TestPerPumpCircuitBreaker_NoBackoffOnRepeatedFailure(t *testing.T) {
	originalPumps := Pumps
	t.Cleanup(func() { Pumps = originalPumps })

	failing := &countingFailingPump{name: "always-fails"}
	healthy := &MockedPump{}
	Pumps = []pumps.Pump{failing, healthy}

	const cycles = 100
	keys := []interface{}{
		analytics.AnalyticsRecord{APIID: "r", OrgID: "o"},
	}

	for i := 0; i < cycles; i++ {
		writeToPumps(keys, nil, time.Now(), 2)
	}

	got := failing.Count()

	// Document current behavior: no breaker, so invocations == cycles.
	if got != cycles {
		t.Errorf("failing pump invoked %d times across %d cycles — "+
			"a circuit breaker may have been introduced. "+
			"Update test expectation and close KI "+
			"`pump-no-per-pump-circuit-breaker`.", got, cycles)
	}

	// Independence cross-check: healthy pump still got every cycle's record.
	if healthy.CounterRequest != cycles {
		t.Errorf("healthy peer was invoked %d/%d times — "+
			"failing pump's repeated errors leaked into healthy pump's path",
			healthy.CounterRequest, cycles)
	}
}
