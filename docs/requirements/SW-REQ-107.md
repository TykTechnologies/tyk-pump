# SW-REQ-107: Mongo aggregate — valid configured window preserved

## Parent
This requirement decomposes SW-REQ-058's configurable-window policy for the
valid `aggregation_time` branch.

## Intent
When `StoreAnalyticsPerMinute` is false and `AggregationTime` is configured
within the inclusive 1-60 minute range, `MongoAggregatePump.SetAggregationTime`
shall preserve that configured value.

## Motivation
TT-506 made the Mongo aggregate window configurable so operators could reduce
the number of analytics grouped into a single Mongo document. This child
requirement pins the valid configured-value branch independently from the
per-minute override and invalid-value defaulting.

## Code References
- `pumps/mongo_aggregate.go:MongoAggregatePump.SetAggregationTime` preserves
  valid configured values when the per-minute override is disabled.

## Evidence
- `pumps/mongo_mcdc_test.go:TestSetAggregationTime_ValidValuePreserved`
  verifies that a valid configured value is preserved.
- `pumps/mongo_mcdc_test.go:TestSetAggregationTime_GreaterThan60` verifies the
  non-valid branch is not treated as a preserved configured value.
- `pumps/mongo_aggregate_test.go:TestMongoAggregatePump_StoreAnalyticsPerMinute`
  verifies that the per-minute override takes precedence over a valid
  configured value.

## Open Questions
- None.
