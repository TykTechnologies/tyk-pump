# SW-REQ-005: StatsD instrumentation sink with non-fatal errors

## Intent
Realises the instrumentation-emission obligation of parent **SYS-REQ-013**. `SetupInstrumentation` (gated on `TYK_INSTRUMENTATION=1`) constructs a `StatsDSink` pointed at `SystemConfig.StatsdConnectionString` with prefix `SystemConfig.StatsdPrefix`, adds it to the global `gocraft/health` stream, and spawns `MonitorApplicationInstrumentation` to emit GC pause quantiles every 5 seconds. Per-purge timing (`purge_time_all` and `purge_time_<pump-name>`) and per-record event counters are emitted inline from `StartPurgeLoop` / `execPumpWriting` through the same stream. The sink itself drops oversized UDP packets and silently swallows UDP write errors so that a broken statsd cannot fail the purge loop.

## Motivation
Instrumentation is a side-channel: the contract is that *enabling* it is optional and *failing* it never blocks the data path. The StatsD sink is implemented over a UDP socket (fire-and-forget by definition), buffers up to `maxUdpBytes = 1440` (Ethernet MTU minus UDP header), and uses a per-`{job,event,suffix}` prefix-buffer cache to keep per-emit allocation cost low. Trade-off: silent drop on overflow (`writeStatsDMetric` returns when `lenb > maxUdpBytes`) means a custom job that emits a giant tag set will never see those metrics at the receiver, with no observable error.

## Code references
- `instrumentation_helpers.go:16 SetupInstrumentation` — env gate (`TYK_INSTRUMENTATION == "1"`), missing-connstring early return at line 29-32, `NewStatsDSink` + `instrument.AddSink`.
- `instrumentation_helpers.go:50 MonitorApplicationInstrumentation` — GC pause quantile emitter loop.
- `instrumentation_statsd_sink.go:93 NewStatsDSink` — UDP socket + goroutine `loop`.
- `instrumentation_statsd_sink.go:166 (*StatsDSink).loop` — drain/flush state machine.
- `instrumentation_statsd_sink.go:305 flush`, `:314 writeStatsDMetric` — error-swallowing UDP write paths.
- `main.go:291 job.Timing("purge_time_all", ...)` and `main.go:493 job.Timing("purge_time_"+pmp.GetName(), ...)` — the purge-timing emit points.
- `main.go:328 job.Event("record")` — per-record event counter.

## Evidence
- `instrumentation_test.go:11 TestNewStatsDSink_InvalidAddress` — tagged `// SW-REQ-005:error_handling:negative`; asserts `NewStatsDSink` surfaces the resolver error rather than panicking.

## Open questions
- The `*StatsDSink.flush` (`instrumentation_statsd_sink.go:307`) ignores the `WriteToUDP` error entirely; a chronic network failure produces no log output. The "without failing the purge loop on sink errors" obligation is met by the absence of error propagation, but observability of *whether* metrics are flowing is non-existent.
- `MonitorApplicationInstrumentation` runs an infinite goroutine with no shutdown hook — it survives `checkShutdown` and only dies when the process exits.
- There is no test for the StatsD sink wire-format (no asserted "purge_time_all:42|ms" check). The sink is exercised indirectly only.
