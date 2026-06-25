# SW-REQ-007: Temporal storage reconnect with bounded exponential backoff

## Intent
Realises the reconnect and storage-library connection-adapter side of parent
**SYS-REQ-006**. `TemporalStorageHandler` must preserve the operator-visible
temporal-storage contract while constructing and reconnecting the shared storage
library connector: host/port, address-list, database/auth, sentinel, cluster,
pool, timeout, key-prefix, TLS, legacy `redis_*` aliases, and both
`TYK_PMP_REDIS` / `TYK_PMP_TEMPORAL_STORAGE` env overlays are part of this
requirement.

`TemporalStorageHandler.ensureConnection` is the gate on every public storage
operation (`GetAndDeleteSet`, `SetKey`, ...): it checks the package-level
`connectorSingleton`; if `nil` it logs "Connection dropped, reconnecting..." and
invokes `backoff.Retry` with the exponential strategy returned by
`retry.GetTemporalStorageExponentialBackoff` (multiplier 2, max interval 10s,
`MaxElapsedTime = 0` meaning retry forever) until `r.connect()` succeeds and the
singleton is non-nil.

## Motivation
The retry loop sits behind the storage API rather than inside `connect` so that
*initial* connect (called from `Init`) fails fast and surfaces config errors at
startup, while *steady-state* reconnect (after the connector goes nil) is
patient and never gives up.

The storage-library migration made the adapter contract more important than a
single Redis smoke test: a single-node connection can pass while cluster,
sentinel, TLS, database/auth, or deprecated alias support is silently dropped.
This req therefore names the option mapping explicitly and ties it to static
evidence. Trade-off: `MaxElapsedTime = 0` means the pump never exits the
reconnect loop on its own -- operationally you rely on shutdown via SIGTERM
rather than self-detected total failure. The 10s `MaxInterval` cap keeps the
steady-state poll rate bounded so a long Redis outage doesn't degrade into
hourly retry intervals.

## Code references
- `storage/temporal_storage.go:78-95 Init` — applies deprecated and replacement env prefixes and key-prefix alias/default behavior.
- `storage/temporal_storage.go:106 connect` — the inner reconnect operation; resets the singleton via `resetConnection` if `forceReconnect` or if the singleton is nil.
- `storage/temporal_storage.go:142 resetConnection` — maps `TemporalStorageConfig` into storage-library `model.RedisOptions` and `model.TLS`.
- `storage/temporal_storage.go:201 createConnector` — constructs the storage-library Redis connector and KV/list adapters.
- `storage/temporal_storage.go:328 ensureConnection` — singleton check, backoff strategy from `retry.GetTemporalStorageExponentialBackoff()`, `backoff.Retry(operation, backoffStrategy)`.
- `storage/temporal_storage.go:267, 314` — `ensureConnection` call sites in `GetAndDeleteSet` and `SetKey`.
- `retry/storage-retry.go:10 GetTemporalStorageExponentialBackoff` — the bounded per-attempt strategy (see SW-REQ-031 for that req's full intent).

## Evidence
- `storage/temporal_storage_test.go:161 TestTemporalStorageHandler_ensureConnection` — tagged `// Verifies: SW-REQ-006` and `// Verifies: SW-REQ-007`; exercises the singleton-nil path that triggers the backoff loop.
- `storage/temporal_storage_negative_test.go:8 TestTemporalStorageHandler_GetAndDeleteSet_BackendUnreachable` — tagged `// SW-REQ-007:error_handling:negative`; asserts that an unreachable backend surfaces an error rather than silently dropping records.
- `storage/temporal_storage_mcdc_test.go:232 TestTemporalStorageHandler_Init_RedisKeyPrefixPromoted` — review evidence for deprecated `redis_key_prefix` alias promotion.
- `storage/temporal_storage_mcdc_test.go:255 TestTemporalStorageHandler_Init_EnvPrefixOverridesKeyPrefix` — `env_override_applied` evidence for both `TYK_PMP_REDIS_*` and `TYK_PMP_TEMPORAL_STORAGE_*` prefixes.
- `storage/temporal_storage_mcdc_test.go:370 TestTemporalStorageHandler_Init_PoolTuning` — `backend_connection_timeout_propagated` evidence for non-default pool/timeout config.
- `storage/temporal_storage_mcdc_test.go:397 TestTemporalStorageHandler_ResetConnection_StorageLibraryOptionParity` — static AST evidence for `backend_connection_mode_parity` and TLS option mapping into storage-library `RedisOptions` / `TLS`.
- `storage/temporal_storage_mcdc_test.go:529 TestTemporalStorageHandler_Connect_ResetConnectionError` and `:577 TestCreateConnector_InvalidTLS` — invalid TLS config surfaces as an Init/createConnector error.

## Open questions
- The `connectorSingleton` is package-level state, so two
  `TemporalStorageHandler` instances (e.g. the analytics store and the uptime
  store created in `main.setupAnalyticsStore`) share the same connector. A
  reconnect triggered by one handler implicitly serves the other; the race risk
  is tracked by KI `storage-connector-singleton-race`.
- The storage retry strategy has bounded per-attempt interval but unbounded
  total elapsed time (`MaxElapsedTime = 0`). That is tracked by KI
  `storage-retry-maxelapsed-zero-is-unbounded` and SW-REQ-031.
- Storage operations still use package-level `context.Background()` rather than
  caller cancellation. That gap is tracked by KI
  `temporal-storage-operations-ignore-caller-cancellation`.
