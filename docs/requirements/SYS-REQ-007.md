# SYS-REQ-007: Consumed records removed atomically (at-most-once per run)

## Intent
Records consumed from the temporal store are removed in the same operation as their retrieval, so each record is forwarded at most once per pump run even if the pump crashes mid-cycle. This atomicity guarantee satisfies the parent **STK-REQ-002** by avoiding the alternative (separate `LRANGE` + `DEL`) which would produce duplicates if the pump restarted between the two commands.

## Motivation
Analytics duplicates are silently destructive: downstream dashboards over-count requests, billing pipelines over-charge, and rate-limit decisions drift. The at-most-once promise also makes the pump idempotent across restarts from the perspective of the gateway: once the records have left the store, the pump owns them. The "atomic retrieve+delete" pattern is implemented via the storage library's `Pop` (Redis `LPOP count` under the hood) plus a follow-up `EXPIRE` to guard against stuck readers.

## Formalization
```
when records_consumed ingestion shall always satisfy records_removed_once
```
The input `records_consumed` becomes true when `GetAndDeleteSet` returns a non-empty slice; the output `records_removed_once` holds when those exact records are no longer accessible via subsequent `Pop`. Variables: `specs/system/variables/ingestion.vars.yaml`.

## Code references
- `storage/temporal_storage.go:262 GetAndDeleteSet` — uses `r.list.Pop(ctx, fixedKey, chunkSize)` for the atomic retrieve+delete.
- `storage/temporal_storage.go:288 result, err := r.list.Pop(...)` — single backend call, no separate `DEL`.
- `storage/temporal_storage.go:293-298` — `r.kv.Expire(ctx, fixedKey, expire)` re-applies expiry to the residual list when `chunkSize != -1`, so stuck data still ages out.
- `main.go:278 AnalyticsValues, err := AnalyticsStore.GetAndDeleteSet(...)` — the sole call site in the purge loop.

## Evidence
- `storage/temporal_storage_test.go:47 TestRedisClusterStorageManager_GetAndDeleteSet` — verifies that records are popped and not re-readable.
- Satisfying SW child: **SW-REQ-006** (Temporal storage GetAndDeleteSet semantics).

## Open questions
- "At most once per pump run" is a per-run guarantee, not a system-wide exactly-once: if forwarding fails *after* `Pop` (e.g. all backends down), the records are lost (no DLQ exists). The SYS req describes the storage-side atomicity correctly, but operators sometimes misread it as end-to-end exactly-once.
- The `Pop` operation is atomic per the storage library's contract; this req inherits that assumption rather than reproving it.
- When `chunkSize == 0`, the code rewrites it to `-1` (`storage/temporal_storage.go:284-286`), meaning "drain all"; the expiry-reset branch is then skipped — atomicity still holds, but the request semantic is different from the operator-visible config value.
