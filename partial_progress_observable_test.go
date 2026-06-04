package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingMockPump simulates a pump whose WriteData returns an error so the
// caller can observe per-pump failure. It tracks invocation count so we can
// also assert it was actually attempted (not silently skipped).
type failingMockPump struct {
	pumps.CommonPumpConfig
	name        string
	writeErr    error
	invocations int
	mu          sync.Mutex
}

func (p *failingMockPump) GetName() string {
	return p.name
}

func (p *failingMockPump) New() pumps.Pump {
	return &failingMockPump{name: p.name, writeErr: p.writeErr}
}

func (p *failingMockPump) Init(_ interface{}) error {
	return nil
}

func (p *failingMockPump) WriteData(_ context.Context, keys []interface{}) error {
	p.mu.Lock()
	p.invocations++
	p.mu.Unlock()
	return p.writeErr
}

func (p *failingMockPump) Shutdown() error {
	return nil
}

// Verifies: SW-REQ-001
// SW-REQ-001:partial_progress_observable:scenario
//
// Contract: when the purge cycle dispatches to N pumps and M of them fail,
// the operator MUST be able to observe WHICH pump succeeded and WHICH
// failed — not just an aggregate "cycle took T ms" line.
//
// What this test exercises:
//
//   1. Construct three pumps: two healthy (MockedPump returns nil) and one
//      failing (failingMockPump returns a sentinel error).
//   2. Capture logrus output during a single writeToPumps invocation.
//   3. Assert:
//        (a) the failing pump's name appears in a Warning log line
//            referencing the specific error;
//        (b) the healthy pumps were actually invoked (CounterRequest > 0)
//            — proving "partial progress" (other pumps continued despite
//            the one failure);
//        (c) the failing pump WAS invoked once (the failure was real, not
//            a stub no-op).
//
// If a future regression aggregates errors into a single "cycle had errors"
// line without naming pumps, item (a) fails. If a future regression aborts
// the whole cycle when one pump fails, item (b) fails.
func TestPartialProgressObservable_PerPumpFailureLogged(t *testing.T) {
	originalOut := log.Out
	originalLevel := log.Level
	originalPumps := Pumps
	t.Cleanup(func() {
		log.Out = originalOut
		log.Level = originalLevel
		Pumps = originalPumps
	})

	var buf bytes.Buffer
	log.Out = &buf
	log.Level = logrus.WarnLevel

	healthy1 := &MockedPump{}
	failing := &failingMockPump{
		name:     "failing-pump-A",
		writeErr: errors.New("simulated downstream outage"),
	}
	healthy2 := &MockedPump{}

	Pumps = []pumps.Pump{healthy1, failing, healthy2}

	keys := make([]interface{}, 2)
	keys[0] = analytics.AnalyticsRecord{APIID: "rec1"}
	keys[1] = analytics.AnalyticsRecord{APIID: "rec2"}

	job := instrument.NewJob("TestPartialProgressJob")
	writeToPumps(keys, job, time.Now(), 2)

	// Drain the WaitGroup inside writeToPumps before asserting on
	// counters/logs (writeToPumps already wg.Wait()s, but the logrus
	// flush may lag fractionally on some platforms).
	logOutput := buf.String()

	t.Run("per-pump failure is named in log output", func(t *testing.T) {
		assert.Contains(t, logOutput, failing.name,
			"failing pump name MUST appear in operator-visible log "+
				"so the operator knows WHICH pump failed.\n"+
				"Full log capture:\n%s", logOutput)
		assert.Contains(t, logOutput, "simulated downstream outage",
			"the underlying error string MUST appear so the operator "+
				"can diagnose root cause.\nFull log capture:\n%s", logOutput)
	})

	t.Run("healthy pumps wrote despite peer failure (partial progress)", func(t *testing.T) {
		assert.Equalf(t, 2, healthy1.CounterRequest,
			"healthy pump 1 must process all 2 records despite the failing "+
				"peer — proves one bad pump does not abort the cycle")
		assert.Equalf(t, 2, healthy2.CounterRequest,
			"healthy pump 2 must process all 2 records despite the failing "+
				"peer — proves one bad pump does not abort the cycle")
	})

	t.Run("failing pump was actually invoked", func(t *testing.T) {
		failing.mu.Lock()
		defer failing.mu.Unlock()
		assert.Equalf(t, 1, failing.invocations,
			"failing pump must have been invoked exactly once "+
				"(invocations=%d) — otherwise the test is vacuous", failing.invocations)
	})
}

// Verifies: SW-REQ-001
// SW-REQ-001:partial_progress_observable:nominal
//
// All-pumps-succeed sanity arm: when every pump succeeds, no per-pump
// failure log lines are emitted (no false alarms). This is the
// no-trigger pair of the failure-observability contract.
func TestPartialProgressObservable_AllSuccessNoFailureLog(t *testing.T) {
	originalOut := log.Out
	originalLevel := log.Level
	originalPumps := Pumps
	t.Cleanup(func() {
		log.Out = originalOut
		log.Level = originalLevel
		Pumps = originalPumps
	})

	var buf bytes.Buffer
	log.Out = &buf
	log.Level = logrus.WarnLevel

	healthy1 := &MockedPump{}
	healthy2 := &MockedPump{}

	Pumps = []pumps.Pump{healthy1, healthy2}

	keys := []interface{}{
		analytics.AnalyticsRecord{APIID: "rec1"},
	}

	job := instrument.NewJob("TestAllSuccessJob")
	writeToPumps(keys, job, time.Now(), 2)

	logOutput := buf.String()
	assert.NotContains(t, strings.ToLower(logOutput), "error writing to",
		"no per-pump error log line should appear when all pumps succeed.\nFull log capture:\n%s", logOutput)
	require.Equal(t, 1, healthy1.CounterRequest)
	require.Equal(t, 1, healthy2.CounterRequest)
}
