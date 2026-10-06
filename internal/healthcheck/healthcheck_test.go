package healthcheck

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TykTechnologies/tyk-pump/internal/otel"
)

var errDown = errors.New("dependency down")

func okProbe(component, store string) Probe {
	return Probe{Component: component, Store: store, Ping: func(context.Context) error { return nil }}
}

func failProbe(component, store string) Probe {
	return Probe{Component: component, Store: store, Ping: func(context.Context) error { return errDown }}
}

// countingProbe counts calls and fails while down is set.
func countingProbe(component, store string, calls *atomic.Int32, down *atomic.Bool) Probe {
	return Probe{Component: component, Store: store, Ping: func(context.Context) error {
		calls.Add(1)
		if down != nil && down.Load() {
			return errDown
		}
		return nil
	}}
}

func TestHealthSamples_MapsProbesToSamples(t *testing.T) {
	c := New([]Probe{
		failProbe("temporal_storage", "redis"),
		okProbe("uptime", "mongo"),
	})

	got := c.HealthSamples(context.Background())
	assert.Equal(t, []otel.HealthSample{
		{Component: "temporal_storage", Store: "redis", Healthy: false},
		{Component: "uptime", Store: "mongo", Healthy: true},
	}, got)

	got[1].Healthy = false
	assert.True(t, c.HealthSamples(context.Background())[1].Healthy, "callers get a copy of the cache")
}

// TestHealthSamples_RenewalBoundsLoad: reads inside the renewal period serve
// the cache (one ping per component per period regardless of export rate);
// after it, a new round runs and picks up recovery.
func TestHealthSamples_RenewalBoundsLoad(t *testing.T) {
	var calls atomic.Int32
	var down atomic.Bool
	down.Store(true)
	c := NewWithTimings([]Probe{countingProbe("temporal_storage", "redis", &calls, &down)}, 50*time.Millisecond, time.Second)

	assert.False(t, c.HealthSamples(context.Background())[0].Healthy)
	for i := 0; i < 20; i++ {
		c.HealthSamples(context.Background())
	}
	assert.EqualValues(t, 1, calls.Load(), "reads within the renewal period must not probe")

	down.Store(false)
	time.Sleep(60 * time.Millisecond)
	assert.True(t, c.HealthSamples(context.Background())[0].Healthy, "recovery shows on the next round")
	assert.EqualValues(t, 2, calls.Load())
}

// TestHealthSamples_TimeoutIsUnhealthy: a probe honouring ctx but slower than
// the timeout reads 0 and does not hold the others.
func TestHealthSamples_TimeoutIsUnhealthy(t *testing.T) {
	const timeout = 50 * time.Millisecond
	c := NewWithTimings([]Probe{
		{Component: "temporal_storage", Store: "redis", Ping: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }},
		okProbe("uptime", "mongo"),
	}, time.Minute, timeout)

	start := time.Now()
	got := c.HealthSamples(context.Background())
	assert.Less(t, time.Since(start), timeout+time.Second)
	assert.Equal(t, []otel.HealthSample{
		{Component: "temporal_storage", Store: "redis", Healthy: false},
		{Component: "uptime", Store: "mongo", Healthy: true},
	}, got)
}

// TestHealthSamples_ProbeIgnoringCtxIsUnhealthy: a driver that ignores ctx
// (mgo's Ping does) still reads 0 at the timeout instead of keeping the round
// in flight, so the gauge cannot freeze at a stale 1. While that call is still
// running, later rounds read 0 without calling the driver again, so calls
// against a hung dependency never pile up.
func TestHealthSamples_ProbeIgnoringCtxIsUnhealthy(t *testing.T) {
	const timeout = 50 * time.Millisecond
	release := make(chan struct{})
	var calls atomic.Int32
	c := NewWithTimings([]Probe{
		{Component: "uptime", Store: "mongo", Ping: func(context.Context) error { calls.Add(1); <-release; return nil }},
		okProbe("temporal_storage", "redis"),
	}, time.Millisecond, timeout)

	start := time.Now()
	got := c.HealthSamples(context.Background())
	assert.Less(t, time.Since(start), timeout+time.Second)
	assert.Equal(t, []otel.HealthSample{
		{Component: "uptime", Store: "mongo", Healthy: false},
		{Component: "temporal_storage", Store: "redis", Healthy: true},
	}, got)

	time.Sleep(2 * time.Millisecond) // past the renewal period: a new round runs
	assert.False(t, c.HealthSamples(context.Background())[0].Healthy, "still hung ⇒ still unhealthy")
	assert.EqualValues(t, 1, calls.Load(), "a hung call must not be stacked by later rounds")

	close(release)
	require.Eventually(t, func() bool { return c.HealthSamples(context.Background())[0].Healthy },
		time.Second, 5*time.Millisecond, "once the call returns the next round probes again")
	assert.EqualValues(t, 2, calls.Load())
}

// TestHealthSamples_PanickingProbeIsUnhealthy: a panic inside a driver never
// crashes the Pump; the dependency simply reads 0.
func TestHealthSamples_PanickingProbeIsUnhealthy(t *testing.T) {
	c := New([]Probe{
		{Component: "uptime", Store: "postgres", Ping: func(context.Context) error { panic("boom") }},
		okProbe("temporal_storage", "redis"),
	})

	var got []otel.HealthSample
	require.NotPanics(t, func() { got = c.HealthSamples(context.Background()) })
	assert.Equal(t, []otel.HealthSample{
		{Component: "uptime", Store: "postgres", Healthy: false},
		{Component: "temporal_storage", Store: "redis", Healthy: true},
	}, got)
}

// TestHealthSamples_HungProbeServesLastSnapshot: a round that outlives the
// collect context returns the previous snapshot (nil before the first round,
// so nothing is emitted) and never stacks a second round.
func TestHealthSamples_HungProbeServesLastSnapshot(t *testing.T) {
	release := make(chan struct{})
	var rounds atomic.Int32
	hung := Probe{Component: "temporal_storage", Store: "redis", Ping: func(context.Context) error {
		rounds.Add(1)
		<-release
		return nil
	}}
	c := NewWithTimings([]Probe{hung}, time.Millisecond, time.Minute)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Nil(t, c.HealthSamples(cancelled), "no snapshot yet ⇒ nothing emitted, never a placeholder")
	require.Eventually(t, func() bool { return rounds.Load() == 1 }, time.Second, time.Millisecond, "the round runs in the background")
	assert.Nil(t, c.HealthSamples(cancelled), "the in-flight round must not be duplicated")
	assert.EqualValues(t, 1, rounds.Load())

	close(release)
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return !c.inFlight
	}, time.Second, time.Millisecond)
	want := []otel.HealthSample{{Component: "temporal_storage", Store: "redis", Healthy: true}}
	assert.Equal(t, want, c.HealthSamples(context.Background()), "the finished round is served afterwards")

	// A later hung round serves the last good snapshot.
	release2 := make(chan struct{})
	defer close(release2)
	hung2 := Probe{Component: "temporal_storage", Store: "redis", Ping: func(context.Context) error { <-release2; return errDown }}
	c2 := NewWithTimings([]Probe{hung2}, time.Millisecond, time.Minute)
	c2.mu.Lock()
	c2.last, c2.lastAt = want, time.Now().Add(-time.Hour)
	c2.mu.Unlock()
	assert.Equal(t, want, c2.HealthSamples(cancelled), "a hung probe serves the last snapshot, not a stall")
}

// TestHealthSamples_EmptyProbes: a checker without probes emits no series.
func TestHealthSamples_EmptyProbes(t *testing.T) {
	assert.Empty(t, New(nil).HealthSamples(context.Background()), "no probes ⇒ no series")
}
