// Package healthcheck probes the Pump's own dependencies (the Redis temporal
// storage and the uptime pump's datastore) and exposes the results as
// otel.HealthSample values for the tyk.pump.health gauge.
//
// Every probe is a real round trip (Ping), never a configuration-presence
// check, so a dependency that is configured but broken can never report
// healthy. Results are cached for a renewal period, probes run concurrently
// under a per-probe timeout, and at most one probe round is in flight at a
// time, so a hung dependency serves the last snapshot instead of stalling the
// export cycle. It mirrors the Dashboard's internal/healthcheck package, and
// additionally bounds probes whose driver ignores ctx (see ping).
//
// The /health endpoint is deliberately not backed by this package: it is the
// liveness target in deployment manifests, and making it deep would turn a
// Redis blip into a restart loop.
package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TykTechnologies/tyk-pump/internal/otel"
)

const (
	// DefaultRenewal is how long a probe round stays valid. It bounds the
	// load on dependencies to one lightweight ping per component per period
	// regardless of the export rate.
	DefaultRenewal = 10 * time.Second
	// DefaultTimeout bounds one probe. A probe that does not answer in time
	// counts as unhealthy.
	DefaultTimeout = 2 * time.Second
)

// Probe is one dependency to check: its stable role, the engine backing it,
// and the real operation that proves it is reachable.
type Probe struct {
	Ping      func(ctx context.Context) error
	Component string
	Store     string
}

// Checker runs a fixed set of probes and caches the latest results. Apart
// from at most one driver call per probe abandoned at its timeout, no
// goroutine runs outside a probe round, so there is nothing to stop on
// shutdown.
type Checker struct {
	// lastAt, last and inFlight are guarded by mu.
	lastAt time.Time
	probes []Probe
	last   []otel.HealthSample // nil until the first round completes
	// busy[i] is set while probes[i]'s driver call runs, including after it
	// was abandoned at its timeout.
	busy     []atomic.Bool
	renewal  time.Duration
	timeout  time.Duration
	mu       sync.Mutex
	inFlight bool
}

// New returns a Checker with the default renewal period and probe timeout.
func New(probes []Probe) *Checker {
	return NewWithTimings(probes, DefaultRenewal, DefaultTimeout)
}

// NewWithTimings returns a Checker with explicit renewal period and timeout.
func NewWithTimings(probes []Probe, renewal, timeout time.Duration) *Checker {
	return &Checker{probes: probes, busy: make([]atomic.Bool, len(probes)), renewal: renewal, timeout: timeout}
}

// HealthSamples returns one sample per probe. It is the snapshot function for
// otel.MetricInstruments.RegisterHealthObserver and runs once per export
// cycle. A cached round younger than the renewal period is served as is;
// otherwise a new round runs, bounded by ctx: if the probes outlive ctx the
// last snapshot is returned (nil before the first round, so no series is
// emitted rather than a placeholder) and the round finishes in the background
// for the next export. Only one round is ever in flight.
func (c *Checker) HealthSamples(ctx context.Context) []otel.HealthSample {
	c.mu.Lock()
	if c.inFlight || (c.last != nil && time.Since(c.lastAt) < c.renewal) {
		out := slices.Clone(c.last)
		c.mu.Unlock()
		return out
	}
	c.inFlight = true
	c.mu.Unlock()

	done := make(chan []otel.HealthSample, 1)
	go func() {
		out := c.probeAll()
		c.mu.Lock()
		c.last, c.lastAt, c.inFlight = out, time.Now(), false
		c.mu.Unlock()
		done <- out
	}()

	select {
	case out := <-done:
		return slices.Clone(out)
	case <-ctx.Done():
		c.mu.Lock()
		defer c.mu.Unlock()
		return slices.Clone(c.last)
	}
}

// probeAll runs every probe concurrently, each under its own timeout, and
// returns the results in probe order.
func (c *Checker) probeAll() []otel.HealthSample {
	out := make([]otel.HealthSample, len(c.probes))
	var wg sync.WaitGroup
	for i, p := range c.probes {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
			defer cancel()
			out[i] = otel.HealthSample{Component: p.Component, Store: p.Store, Healthy: ping(ctx, p.Ping, &c.busy[i]) == nil}
		})
	}
	wg.Wait()
	return out
}

// ping runs fn bounded by ctx even when fn ignores it: the mgo driver's Ping
// only honours its own socket timeouts, which would keep a round in flight,
// and the gauge frozen at its last value, for up to a minute against a paused
// Mongo. The abandoned call finishes in the background when the driver gives
// up; until then busy stays set and later rounds read the dependency as
// unhealthy without calling it again, so calls against a hung dependency never
// pile up. A panicking probe counts as unhealthy instead of crashing the Pump.
func ping(ctx context.Context, fn func(context.Context) error, busy *atomic.Bool) error {
	if !busy.CompareAndSwap(false, true) {
		return errors.New("previous health probe still running")
	}
	errc := make(chan error, 1)
	go func() {
		defer busy.Store(false)
		defer func() {
			if r := recover(); r != nil {
				errc <- fmt.Errorf("health probe panicked: %v", r)
			}
		}()
		errc <- fn(ctx)
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
