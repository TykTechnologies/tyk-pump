# SYS-REQ-013: Instrumentation emits purge metrics when enabled

## Intent
When instrumentation is enabled, the pump emits purge-timing and record-count metrics to the configured metrics sink. This satisfies parent **STK-REQ-004** (observability) by giving operators a quantitative view of pump throughput and latency, independent of per-backend logging.

## Motivation
Operators need at least two numbers to size a pump deployment: how long each purge takes (so they can pick `purge_delay`) and how many records flow through (so they can pick `purge_chunk`). Capturing this as a SYS-level obligation guarantees those numbers exist whenever instrumentation is on — future refactors cannot silently drop the timing calls without violating this req. The metrics sink is currently StatsD, but the abstraction (`instrument.NewJob`, `job.Timing`, `job.Event`) leaves room for additional sinks without changing the contract.

## Formalization
```
when instrumentation_enabled observability shall eventually satisfy metrics_emitted
```
The input `instrumentation_enabled` becomes true when `TYK_INSTRUMENTATION=1` and `statsd_connection_string` is set; the output `metrics_emitted` becomes true once `job.Timing("purge_time_all", ...)` or `job.Event("record")` has flushed to the sink. Variables: `specs/system/variables/observability.vars.yaml`.

## Code references
- `instrumentation_helpers.go:16 SetupInstrumentation` — env check (`TYK_INSTRUMENTATION == "1"`), sink wiring, then `instrument.AddSink(statsdSink)`.
- `instrumentation_helpers.go:50 MonitorApplicationInstrumentation` — background GC stats reporter.
- `instrumentation_statsd_sink.go` — full StatsD sink implementation.
- `main.go:264 job := instrument.NewJob("PumpRecordsPurge")` — per-purge job context.
- `main.go:291 job.Timing("purge_time_all", time.Since(startTime).Nanoseconds())` — full-purge timing.
- `main.go:328 job.Event("record")` — per-record counter.
- `main.go:493 job.Timing("purge_time_"+pmp.GetName(), ...)` — per-pump timing.

## Evidence
- `instrumentation_test.go:11 TestNewStatsDSink_InvalidAddress` — verifies sink construction error handling.
- Satisfying SW child: **SW-REQ-005** (instrumentation sink + metrics emission).

## Open questions
- The "configured metrics sink" today is only StatsD; the SYS req is intentionally sink-agnostic but the implementation does not yet support an alternative.
- When instrumentation is disabled, `instrument.NewJob` still runs (the stream just has no sinks attached); the timing calls are no-ops. The SYS req correctly conditions emission on `instrumentation_enabled` but does not say the calls are made unconditionally — worth noting for reviewers expecting a runtime gate.
- The set of emitted metric names is implicit in the code; there is no SYS-level enumeration of which metrics must exist.
