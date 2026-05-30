# SYS-REQ-017: Metrics sink failure does not interrupt the purge loop

## Intent
When the metrics sink fails to accept an emission, the pump continues the purge loop and does not propagate the sink error to records being forwarded. This error-handling obligation satisfies parent **STK-REQ-004** (observability) with the right priority: telemetry is best-effort and must never become a cause of data loss.

## Motivation
A flapping StatsD endpoint, a DNS hiccup, or a sink overload should never translate into dropped analytics records. Capturing this at the SYS layer prevents future changes from synchronously propagating sink errors back up `StartPurgeLoop` — a class of bug that has caused outages in similar systems. The reverse direction (records failing should not block the sink) is implicitly out-of-scope here.

## Formalization
```
when metrics_sink_failure observability shall always satisfy purge_loop_continues
```
The input `metrics_sink_failure` becomes true when an underlying sink call (`job.Timing`, `job.Event`, GaugeKv, etc.) cannot deliver; the output `purge_loop_continues` holds when the next purge tick proceeds normally and no record write is affected. Variables: `specs/system/variables/observability.vars.yaml`.

## Code references
- `instrumentation_helpers.go:12 var instrument = health.NewStream()` — the stream owns the sinks.
- `instrumentation_helpers.go:40-44` — `if err != nil { log.Fatal("Failed to start StatsD check: ", err); return }` only at *setup* time; after setup, sink errors are absorbed by the `health.Stream` API which is fire-and-forget.
- `main.go:264 job := instrument.NewJob("PumpRecordsPurge")` and subsequent `job.Timing` / `job.Event` calls — return no error and cannot affect the surrounding control flow.
- `main.go:291`, `:328`, `:493` — emission sites, each return `void` from the perspective of the purge loop.

## Evidence
- `instrumentation_test.go:11 TestNewStatsDSink_InvalidAddress` — verifies that an invalid sink address is detected at setup; the surrounding purge code is structured so post-setup emission cannot affect record writes.
- Satisfying SW child: none with a dedicated "ignore sink errors" SW req; the obligation is realized by the absence of error-propagation paths from instrumentation back to the purge loop.

## Open questions
- The claim relies on the `gocraft/health` library being fire-and-forget; if a future version of that library returned errors and the codebase began to propagate them, this req would be silently violated. Worth pinning with a unit test that injects a failing sink and asserts the purge loop still completes.
- `instrumentation_helpers.go:38 log.Fatal` at sink construction is itself a *setup-time* contradiction of this req — if StatsD is misconfigured at startup, the pump exits. The req as written only governs steady-state emission failures, not startup, but this is not explicit.
