# SYS-REQ-005: Per-backend write timeout enforced

## Intent
The pump enforces a configurable per-backend write timeout and abandons any write that exceeds it, without blocking writes to other backends. This boundary obligation satisfies the parent **STK-REQ-002** (resilient delivery): a slow or hung backend is bounded by its `timeout` setting rather than wedging the whole purge cycle.

## Motivation
Real-world backends include cloud logging APIs that can stall for minutes during incidents. Without an enforced upper bound per write, a stalled backend would keep its goroutine alive past the next `purge_delay` tick and pile up work indefinitely. Codifying the abort-on-timeout behavior at the SYS layer protects operators from configuration mistakes (a backend without `timeout` set still gets a context, but only the `Cancel` variant) and tells reviewers what to expect when reading the per-pump fan-out code.

## Formalization
```
when write_exceeds_timeout delivery shall immediately satisfy write_aborted
```
The input `write_exceeds_timeout` becomes true when the configured `pmp.GetTimeout()` elapses before the pump's goroutine completes; the output `write_aborted` holds when `ctx.Err() == context.DeadlineExceeded` and `execPumpWriting` returns without waiting further. Variables: `specs/system/variables/delivery.vars.yaml`.

## Code references
- `main.go:456-464` — `timeout := pmp.GetTimeout(); ctx, cancel := context.WithTimeout(...)` (or `WithCancel` when timeout is 0).
- `main.go:480-490` — `case <-ctx.Done():` then `context.DeadlineExceeded` branch logs "Timeout Writing to: ..." and returns.
- `pumps/common.go:40 SetTimeout` / `:45 GetTimeout` — the configuration knob each pump uses.
- `main.go:436-446` — `time.AfterFunc(purge_delay)` warning fires if any pump exceeds `purge_delay`; complements the strict timeout.

## Evidence
- `main_test.go:168 TestWriteDataWithFilters` exercises the fan-out write path; no dedicated timeout-expiry test was located in `main_test.go` — pump-specific timeout tests live in individual backend test suites.
- SW children: **SW-REQ-016** (`SetTimeout`/`GetTimeout` on `CommonPumpConfig`) and the per-backend SW reqs that honour the context deadline.

## Open questions
- When `pmp.GetTimeout() == 0` the pump runs with `context.WithCancel` and no deadline — the SYS req says "configurable timeout" but does not state the behaviour when timeout is unset. In practice this means "no abort" with only the `purge_delay` warning at `main.go:438`.
- The req says "immediately satisfy write_aborted", but `execPumpWriting` only stops *waiting* on the slow goroutine; the pump's own goroutine may continue running to completion in the background until it observes the cancelled context. The strength of "abort" is therefore down to each backend honoring `ctx`.
- No dedicated SYS-level test asserts the timeout boundary; assurance comes from per-pump tests and the FRETish pattern.
