# SYS-REQ-001: Pump consumes pending analytics records on each purge cycle

## Intent
On every tick of the purge loop the pump shall drain pending analytics records from the configured temporal store across all analytics keys, so that no record produced by an upstream gateway is left waiting in the store indefinitely. This is the load-bearing ingestion guarantee that satisfies the parent **STK-REQ-001** ("analytics records produced by gateways must be moved to downstream backends").

## Motivation
At the system layer this is a black-box promise that gateway-produced analytics will eventually be consumed regardless of how many analytics keys the gateway is sharding into. Operators rely on this to size their temporal store: as long as the pump is running, the store does not grow unbounded for normal traffic. Phase 0.6 narrowed the original SYS-REQ-001 — fan-out to backends (now **SYS-REQ-022**) was spun out so that this requirement focuses strictly on the ingestion-side promise.

## Formalization
```
when records_pending ingestion shall eventually satisfy records_consumed_from_store
```
The input `records_pending` becomes true when at least one of the analytics-key lists holds unconsumed entries; the output `records_consumed_from_store` becomes true once `GetAndDeleteSet` has popped them on a purge tick. Variables are declared in `specs/system/variables/ingestion.vars.yaml`.

## Code references
- `main.go:261 StartPurgeLoop` — fixed `time.Tick(secInterval * time.Second)` loop.
- `main.go:267-289` — for-loop iterating `i = -1..9` over `storage.ANALYTICS_KEYNAME` and `tyk-system-analytics_0..9` shard keys, plus both serializer suffixes.
- `main.go:278` — `AnalyticsStore.GetAndDeleteSet(analyticsKeyName, chunkSize, expire)` is the actual consume call.
- `storage/temporal_storage.go:262 GetAndDeleteSet` — backend pop + expiry refresh.
- `storage/store.go:18 ANALYTICS_KEYNAME = "tyk-system-analytics"` constant.

## Evidence
- `main_test.go:257 TestShutdown` exercises the purge-loop / shutdown wiring end-to-end.
- `storage/temporal_storage_test.go:47 TestRedisClusterStorageManager_GetAndDeleteSet` verifies the pop+expire semantic that backs this req.
- Satisfying SW children (per `traces.satisfies` inverse): **SW-REQ-001** (purge loop & main startup), **SW-REQ-003** (pump initialisation), **SW-REQ-004** (shutdown handler), **SW-REQ-017** (pump registry).

## Open questions
- Phase 0.6 split: the original SYS-REQ-001 also covered fan-out to all configured backend pumps; that obligation now lives in companion req **SYS-REQ-022** (`record_dispatched_to_all_backends`). Aggregation grouping and counter maintenance previously bundled here ended up under **SYS-REQ-018** / **SYS-REQ-019**.
- The "eventually" bound is not formalized: in practice it is one `secInterval` tick (`SystemConfig.PurgeDelay`), but the FRETish does not pin that interval. If `PurgeChunk` is set and the store holds more than `chunkSize`, full drain requires multiple ticks — not currently captured.
- The hard-coded `i < 10` shard ceiling in `main.go:267` is an undocumented upper bound on `analytics_config.enable_multiple_analytics_keys`; if the gateway ever shards beyond 10 keys, this req is silently violated.
