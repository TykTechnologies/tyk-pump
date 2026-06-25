# SW-REQ-109: Mongo aggregate — resolved window handoff

## Parent
This requirement decomposes SW-REQ-058's runtime handoff from window resolution
to aggregation execution.

## Intent
When `MongoAggregatePump.WriteData` has at least one non-MCP analytics record
after filtering, it shall pass the currently resolved `AggregationTime` value
to `analytics.AggregateData` for that batch.

## Motivation
Correctly normalizing `aggregation_time` at Init is not enough if the runtime
data path later ignores that value. This requirement pins the call-site
contract that makes configurable windows affect persisted aggregate buckets.

## Code References
- `pumps/mongo_aggregate.go:MongoAggregatePump.WriteData` filters MCP records,
  returns early for empty input, and passes `m.dbConf.AggregationTime` to
  `analytics.AggregateData`.

## Evidence
- `pumps/mongo_aggregate_test.go:TestMongoAggregatePump_WriteDataPassesResolvedAggregationTime`
  verifies the `WriteData` call site passes `m.dbConf.AggregationTime` into
  `analytics.AggregateData`.
- `pumps/mongo_mcdc_test.go:TestMongoAggregatePump_WriteData_EmptyData`
  verifies the empty-input vacuous branch.

## Open Questions
- None.
