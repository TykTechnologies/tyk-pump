# STK-REQ-001: Analytics visibility for Tyk operators

## Intent
Tyk operators (platform/SRE personas running a Tyk deployment) deploy the
pump primarily to gain visibility into the traffic the gateway is handling.
This stakeholder requirement captures the top-level "why pump exists" property:
faithful forwarding of gateway analytics into operator-chosen observability
backends so that usage can be monitored, failures debugged, and consumption
reported across organisations and APIs.

## Motivation
Without faithful, low-loss forwarding the rest of the value proposition
collapses. The gateway produces records but does not persist them itself; it
hands them off to Redis-shaped temporal storage and relies on the pump to
relay them onward. Any drop in fields, silent transformation, or "convenient"
filtering breaks dashboards and billing that downstream teams have built on
top of those records — and these breakages are typically discovered weeks
later, after the records are gone from Redis.

The trade-off this requirement embodies is also explicit: operators get to
choose where the data ends up (Mongo, SQL, Elastic, Splunk, etc.), but the
pump is responsible for not silently sacrificing fidelity in service of that
choice. Aggregation, filtering, and trimming are opt-in (see STK-REQ-003);
nominal behaviour is "every field, every record, to every configured sink".

## Code references
Decomposes into the following SYS reqs via its acceptance criteria:
- AC-001 (faithful forwarding): `SYS-REQ-001` (consume on each purge cycle),
  `SYS-REQ-002` (preserve fields when forwarding), `SYS-REQ-022` (dispatch each
  record to every configured backend).
- AC-002 (reporting dimensions): `SYS-REQ-003` (aggregation windowing),
  `SYS-REQ-018` (group counters per org/API), `SYS-REQ-019` (hits/successes/
  errors/latency counters per aggregated unit).

The operator-visible entry point is the purge loop in
`main.go:261` `StartPurgeLoop`, which reads from
`storage.ANALYTICS_KEYNAME` (`storage/store.go:18`), deserializes via
`serializer.AnalyticsSerializer` (`serializer/serializer.go:10`), and fans
out to `pumps.Pump.WriteData` (`pumps/pump.go:17`).

## Evidence
Rolled up from the SYS chain:
- Field-preservation is exercised by `serializer/serializer_test.go` round-trip
  tests against both msgpack and protobuf paths.
- Forwarding fan-out is exercised by `main_test.go` purge tests and per-pump
  `pumps/*_test.go` suites (e.g. `pumps/mongo_test.go`, `pumps/sql_test.go`).
- Aggregation/grouping is exercised by `analytics/aggregate_test.go`.

## Open questions
- The acceptance text says "without loss of the fields operators rely on" but
  the SYS chain does not pin which fields are load-bearing — protobuf path in
  `serializer/protobuf.go` silently drops `City.Names` on the round-trip
  (`protobuf.go:169` sets `Names: nil`). Whether that is considered "loss" by
  this requirement is not formalized.
- No assumption requirement documents that the gateway is the sole producer
  on the analytics key; if a second producer existed, "at most once per pump
  run" semantics in SYS-REQ-007 would not give the operator a duplicate-free
  view of all gateway records.
