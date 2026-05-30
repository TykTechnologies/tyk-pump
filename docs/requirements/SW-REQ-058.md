# SW-REQ-058: Mongo aggregate — aggregation window policy

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-036 (mongo-aggregate). It carries the aggregation-window obligation
in isolation so it can be verified independently of the upsert / sharding
/ self-heal sub-behaviours.

## Intent
The MongoAggregate pump's aggregation window shall be set at Init by
`SetAggregationTime`: when `StoreAnalyticsPerMinute` is true the window
shall be 1 minute; otherwise `AggregationTime` (configurable 1-60 minutes,
defaulting to 60 when unset or out of range). The active window shall be
passed to `analytics.AggregateData(...)` on each `WriteData` call. Derived
from SYS-REQ-003 (aggregation windowing).

## Motivation
The aggregation window controls how analytics are time-bucketed: a 60-minute
window produces hourly aggregates (the default); `StoreAnalyticsPerMinute`
produces minute-grained aggregates suitable for high-resolution dashboards.
This sub-requirement isolates the window-selection logic from the rest of
the aggregate pipeline so the determinism obligation can be exercised
without involving the upsert / sharding behaviours.

## Code references
- `pumps/mongo_aggregate.go:MongoAggregatePump.SetAggregationTime` —
  resolves `AggregationTime` from the operator config, clamps to
  `[1, 60]`, and applies the `StoreAnalyticsPerMinute` override.
- `pumps/mongo_aggregate.go:MongoAggregatePump.WriteData` — passes the
  resolved window into `analytics.AggregateData(...)`.

## Evidence
- `pumps/mongo_aggregate_test.go:TestAggregationTime` (re-annotated
  `Verifies: SW-REQ-058`) — exercises the windowing across 1/3/7/15/30/60
  minute settings.
- `pumps/mongo_aggregate_test.go:TestMongoAggregatePump_StoreAnalyticsPerMinute`
  (re-annotated `Verifies: SW-REQ-058`) — verifies the per-minute override.

## Open questions
- The clamping behaviour (1-60) is implicit; a value of 0 or >60 is
  silently coerced. Operators may expect an Init error instead — worth a
  follow-up review.
