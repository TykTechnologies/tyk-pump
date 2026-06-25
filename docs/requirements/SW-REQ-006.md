# SW-REQ-006: Atomic chunked pop with TTL refresh on remainder

## Intent
Realises parent **SYS-REQ-007** in the storage layer. `TemporalStorageHandler.GetAndDeleteSet` pops up to `chunkSize` entries from the Redis-backed list (using the temporal-storage library's `list.Pop`) and, when `chunkSize != -1` (i.e. a bounded pop), re-applies the `expire` TTL to whatever remains under the same key via `kv.Expire`. When `chunkSize == 0` is passed in by the caller, it is rewritten to `-1` to ask the underlying library to delete the key entirely (preserving legacy "drain everything" semantics from the pre-library implementation).

## Motivation
The atomicity guarantee is what lets the gateway and the pump cohabit on the same Redis without races: a partial pop must not leave records visible-but-rotting, and a TTL re-application must not be skipped if the list still has tail entries. The `chunkSize == 0 → -1` rewrite is an intentional compatibility shim — the older in-tree implementation deleted the whole key when chunk was zero; the storage library treats `-1` as "all", and the bounded-pop branch handles the TTL refresh that the legacy code did inline. Trade-off: `list.Pop` + `kv.Expire` are two RTTs to Redis (not a single Lua script), so there is a small window where the list is shorter than the TTL implies. This is acceptable because the TTL is a safety net, not a primary correctness invariant.

## Code references
- `storage/temporal_storage.go:262 GetAndDeleteSet` — `ensureConnection` → `fixKey` → `list.Pop(ctx, fixedKey, chunkSize)` (line 288) → conditional `kv.Expire(ctx, fixedKey, expire)` (lines 293-298).
- `storage/temporal_storage.go:284-286` — the `chunkSize == 0 → -1` rewrite.
- `storage/temporal_storage.go:300-304` — the `[]string` → `[]interface{}` adapt for the caller (`main.PreprocessAnalyticsValues`).
- `storage/temporal_storage.go:251 fixKey` — applies `KeyPrefix` (default `analytics-` per `KeyPrefix` constant in `storage/store.go`).
- The underlying `list.Pop` is supplied by `github.com/TykTechnologies/storage/temporal/list` and is implemented in Redis via `LRANGE`+`LTRIM` or `LPOP count` depending on the backend version.

## Evidence
- `storage/temporal_storage_test.go:47 TestRedisClusterStorageManager_GetAndDeleteSet` — tagged `// Verifies: SW-REQ-006`; real-Redis integration test asserting pop + remaining-TTL behaviour.
- `storage/temporal_storage_mcdc_test.go:708 TestTemporalStorageHandler_GetAndDeleteSet_FullDrainNormalizesZeroAndSkipsExpire` — tagged `// SW-REQ-006:full_drain_semantics:nominal`; proves the storage-library adapter maps public `chunkSize=0` to backend sentinel `-1` and skips `Expire`.
- `storage/temporal_storage_test.go:108 TestNewTemporalClusterStorageHandler`, `:193 TestTemporalStorageHandler_SetKey`, `:232 TestTemporalStorageHandler_GetName`, `:259 TestTemporalStorageHandler_Init` — adjacent coverage for the surrounding handler API.
- `storage/temporal_storage_negative_test.go:13 TestTemporalStorageHandler_GetAndDeleteSet_BackendUnreachable` — tagged `// SW-REQ-006:atomicity:negative` (also covers SW-REQ-007 and SW-REQ-031); asserts that an unreachable backend produces an error instead of partial side-effects.

## Open questions
- The atomicity claim is only as strong as the underlying library's `list.Pop`. If the library implements pop as `LRANGE`+`LTRIM` (two commands) rather than `LPOP count` (single command, Redis ≥6.2), there is a tiny window between pop and trim where a concurrent pump would re-read the same records. Not pinned by this requirement.
- The `kv.Expire` call follows the pop *non-atomically* — if the pump dies between the two, the remainder list keeps its old TTL. Recovery: the next purge tick re-applies expiry on its own pop.
