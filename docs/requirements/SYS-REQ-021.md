# SYS-REQ-021: Consumed uptime data forwarded to uptime backend

## Intent
When uptime data has been consumed from the temporal store and uptime purging is enabled, the pump forwards the uptime data to the configured uptime backend (Mongo or SQL). This satisfies parent **STK-REQ-005** at the delivery side: drained uptime reports must reach their persistent home, not just be evicted from Redis.

## Motivation
Consume-without-forward would be a data-loss bug: the gateway-side host-checker reports would disappear without being persisted. Capturing this as its own SYS req (split from SYS-REQ-014 in Phase 0.6) makes the obligation atomic and lets the consume-side and the forward-side be reviewed independently — important because they are realised by different subsystems (`storage.GetAndDeleteSet` vs `UptimePump.WriteUptimeData`).

## Formalization
```
when uptime_data_consumed uptime shall eventually satisfy uptime_forwarded
```
The input `uptime_data_consumed` becomes true once `GetAndDeleteSet(UptimeAnalytics_KEYNAME, ...)` returns; the output `uptime_forwarded` becomes true once `UptimePump.WriteUptimeData(...)` has completed (subject to the same `!DontPurgeUptimeData` gate). Variables: `specs/system/variables/uptime.vars.yaml`.

## Code references
- `main.go:293-301` — the gate and the call: `if !SystemConfig.DontPurgeUptimeData { UptimeValues, err := UptimeStorage.GetAndDeleteSet(...); UptimePump.WriteUptimeData(UptimeValues) }`.
- `main.go:239 initialiseUptimePump` — backend selection (`sql` -> `pumps.SQLPump{IsUptime: true}`; default -> `pumps.MongoPump{IsUptime: true}`).
- `pumps/sql.go` / `pumps/mongo.go` — `WriteUptimeData` implementations on each pump (search by symbol; both pumps implement `pumps.UptimePump`).
- `pumps/pump.go` — `UptimePump` interface definition (`Init`, `WriteUptimeData`, `GetName`).

## Evidence
- `analytics/uptime_data_test.go:127 TestAggregateUptimeData` and `:343 TestOnConflictUptimeAssignments` — uptime aggregation / persistence shape.
- Satisfying SW child: **SW-REQ-015** (uptime report data model + storage glue).

## Open questions
- Phase 0.6 origin: spun out of SYS-REQ-014 so the consume and forward sides are atomic.
- Errors from `WriteUptimeData` are not handled at the call site (`main.go:300`) — the return value, if any, is discarded. The SYS req says "forward" without specifying error handling for the forward call.
- Empty `UptimeValues` still results in `WriteUptimeData(nil)` being called; behaviour depends on each pump. The req does not address the empty-slice case.
