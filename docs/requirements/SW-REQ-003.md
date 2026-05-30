# SW-REQ-003: Pump initialisation before purge loop

## Intent
Realises the pump-bring-up portion of parent **SYS-REQ-001**. `main.initialisePumps` walks `SystemConfig.Pumps`, looks each entry up in `pumps.AvailablePumps` via `GetPumpByName`, applies the per-pump options (`Filters`, `Timeout`, `OmitDetailedRecording`, `MaxRecordSize`, `IgnoreFields`, `DecodeRawRequest`, `DecodeRawResponse`), calls `Init(meta)`, and appends the result to the global `Pumps` slice. `initialiseUptimePump` then constructs either a `SQLPump` or `MongoPump` (default) for uptime data and initialises it, unless `DontPurgeUptimeData` is set.

## Motivation
Initialising before the purge loop starts is what lets `StartPurgeLoop` be a tight `time.Tick` body that never blocks on per-tick wiring. Pump load failures are non-fatal (logged and skipped) so a misconfigured Splunk pump cannot take down the whole pipeline — but the post-loop check `if len(Pumps) == 0 { log.Fatal("No pumps configured") }` guarantees the process refuses to start with no functional sinks at all. Trade-off: per-pump option setters are individual methods rather than a single `Configure(PumpConfig)` call, so adding a new option requires touching both `PumpConfig` and the `Pump` interface.

## Code references
- `main.go:192 initialisePumps` — iterates `SystemConfig.Pumps`, calls `pumps.GetPumpByName(pumpTypeName)`, applies setters (lines 207-214), `Init(pmp.Meta)`, appends.
- `main.go:227-231` — fatal-on-empty `Pumps` check.
- `main.go:239 initialiseUptimePump` — switches on `UptimePumpConfig.UptimeType`: `"sql"` builds `&pumps.SQLPump{IsUptime: true}` initialised with `SQLConf`, default builds `&pumps.MongoPump{IsUptime: true}` initialised with `MongoConf`.
- `main.go:122 setupAnalyticsStore` — wires the redis-backed `AnalyticsStore`/`UptimeStorage` before `initialisePumps` runs.
- `main.go:511 main` — call order: `setupAnalyticsStore` → `initialisePumps` → (demo or) `StartPurgeLoop`.

## Evidence
- `main_test.go:168 TestWriteDataWithFilters` — `// Verifies: SW-REQ-003`; constructs a pump via the same code path and drives a write end-to-end.
- The pump-family per-implementation init tests (in `pumps/*_test.go`) cover individual `Init` behaviour but are scoped to SW-REQ-016+, not this req.

## Open questions
- The uptime pump default ("anything that isn't `sql` becomes Mongo") means a typo in `uptime_type` silently produces a Mongo pump pointed at the SQL conn-string fields and crashes at first `WriteUptimeData`. Not surfaced as a config-validation error.
- `initialiseUptimePump` ignores the `error` return of `UptimePump.Init` (lines 247, 251) — a misconfigured uptime pump logs the error inside `Init` but the caller still treats it as success.
- `initialisePumps` builds pumps serially; for N pumps with slow `Init` (e.g., remote dial in elasticsearch) startup time is the sum, not the max.
