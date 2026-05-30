# SW-REQ-007: Temporal storage reconnect with bounded exponential backoff

## Intent
Realises the reconnect side of parent **SYS-REQ-006**. `TemporalStorageHandler.ensureConnection` is the gate on every public storage operation (`GetAndDeleteSet`, `SetKey`, …): it checks the package-level `connectorSingleton`; if `nil` it logs "Connection dropped, reconnecting…" and invokes `backoff.Retry` with the exponential strategy returned by `retry.GetTemporalStorageExponentialBackoff` (multiplier 2, max interval 10s, `MaxElapsedTime = 0` meaning retry forever) until `r.connect()` succeeds and the singleton is non-nil.

## Motivation
The retry loop sits behind the storage API rather than inside `connect` so that *initial* connect (called from `Init`) fails fast and surfaces config errors at startup, while *steady-state* reconnect (after the connector goes nil) is patient and never gives up. Trade-off: `MaxElapsedTime = 0` means the pump never exits the reconnect loop on its own — operationally you rely on shutdown via SIGTERM rather than self-detected total failure. The 10s `MaxInterval` cap keeps the steady-state poll rate bounded so a long Redis outage doesn't degrade into hourly retry intervals.

## Code references
- `storage/temporal_storage.go:328 ensureConnection` — singleton check, backoff strategy from `retry.GetTemporalStorageExponentialBackoff()`, `backoff.Retry(operation, backoffStrategy)`.
- `storage/temporal_storage.go:267, 314` — `ensureConnection` call sites in `GetAndDeleteSet` and `SetKey`.
- `storage/temporal_storage.go:106 connect` — the inner `operation` body; resets the singleton via `resetConnection` if `forceReconnect` or if the singleton is nil.
- `retry/storage-retry.go:10 GetTemporalStorageExponentialBackoff` — the bounded strategy (see SW-REQ-031 for that req's full intent).

## Evidence
- `storage/temporal_storage_test.go:161 TestTemporalStorageHandler_ensureConnection` — tagged `// Verifies: SW-REQ-006` and `// Verifies: SW-REQ-007`; exercises the singleton-nil path that triggers the backoff loop.
- `storage/temporal_storage_negative_test.go:13 TestTemporalStorageHandler_GetAndDeleteSet_BackendUnreachable` — tagged `// SW-REQ-007:error_handling:negative`; uses a short-circuited backoff to assert that an unreachable backend eventually surfaces an error rather than blocking forever (the test relies on the backoff `MaxElapsedTime` being overridden for the test).

## Open questions
- The `connectorSingleton` is package-level state, so two `TemporalStorageHandler` instances (e.g. the analytics store and the uptime store created in `main.setupAnalyticsStore`) share the same connector. A reconnect triggered by one handler implicitly serves the other; this is intentional but worth pinning because it means `forceReconnect=true` on one handler resets the connection for the other.
- The req says "bounded exponential backoff" but `MaxElapsedTime = 0` means *unbounded total elapsed time*. The per-attempt interval is bounded (`MaxInterval = 10s`). The req text should arguably say "bounded per-attempt interval" — slight wording mismatch.
