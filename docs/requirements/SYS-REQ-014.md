# SYS-REQ-014: Consume uptime data on each purge cycle when enabled

## Intent
When uptime purging is enabled, the pump consumes uptime report data from the temporal store on each purge cycle. This satisfies parent **STK-REQ-005** (uptime / host-checker delivery) on the ingestion side: uptime reports produced by the gateway's host checker must not accumulate in Redis indefinitely.

## Motivation
The host checker writes URL-probe reports into a separate Redis list (`tyk-uptime-analytics`); operators expect them to be drained on the same cadence as analytics records when `dont_purge_uptime_data` is false. Capturing this as its own SYS req — rather than folding it into SYS-REQ-001 — recognizes that uptime has its own storage namespace, its own pump implementation (`UptimePump`) and its own enable flag. Phase 0.6 split the original SYS-REQ-014: the forwarding side moved to companion **SYS-REQ-021**.

## Formalization
```
when uptime_purging_enabled uptime shall eventually satisfy uptime_data_consumed
```
The input `uptime_purging_enabled` is true when `!SystemConfig.DontPurgeUptimeData`; the output `uptime_data_consumed` becomes true once `UptimeStorage.GetAndDeleteSet(storage.UptimeAnalytics_KEYNAME, ...)` returns. Variables: `specs/system/variables/uptime.vars.yaml`.

## Code references
- `main.go:293-301` — `if !SystemConfig.DontPurgeUptimeData { UptimeValues, err := UptimeStorage.GetAndDeleteSet(storage.UptimeAnalytics_KEYNAME, chunkSize, expire); ...; UptimePump.WriteUptimeData(UptimeValues) }`.
- `storage/store.go:19 UptimeAnalytics_KEYNAME = "tyk-uptime-analytics"`.
- `main.go:140-156` — `setupAnalyticsStore` creates a dedicated `UptimeStorage` handler with `KeyPrefix = "host-checker:"`.
- `main.go:233-235` — `initialisePumps` conditionally calls `initialiseUptimePump()` only when `!DontPurgeUptimeData`.
- `main.go:239 initialiseUptimePump` — selects SQL or Mongo backend for uptime.

## Evidence
- `analytics/uptime_data_test.go` covers the uptime data model and aggregation (`TestAggregateUptimeData`, `TestUptimeReportData_*`).
- Satisfying SW child: **SW-REQ-015** (uptime report data model + storage glue).

## Open questions
- Phase 0.6 split: the forwarding-side obligation is now **SYS-REQ-021**.
- The uptime consume call runs *once per purge tick* (not iterated over shard keys like the analytics path) — this is consistent with the host checker not sharding uptime keys, but the SYS req does not state the single-key assumption.
- Errors from `GetAndDeleteSet` are logged but the pump proceeds to call `UptimePump.WriteUptimeData(UptimeValues)` with the potentially empty slice; the SYS req does not address the error path.
