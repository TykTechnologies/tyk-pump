# SW-REQ-108: Mongo aggregate — invalid configured window defaults

## Parent
This requirement decomposes SW-REQ-058's configurable-window policy for the
unset or out-of-range `aggregation_time` branch.

## Intent
When `StoreAnalyticsPerMinute` is false and `AggregationTime` is unset, less
than 1, or greater than 60, `MongoAggregatePump.SetAggregationTime` shall
default the aggregation window to 60 minutes.

## Motivation
The default-to-60 behavior is operator-visible. Modeling it explicitly prevents
a future change from silently accepting invalid values as custom windows or
from changing the default bucket width without a requirement review.

## Code References
- `pumps/mongo_aggregate.go:MongoAggregatePump.SetAggregationTime` defaults
  unset or out-of-range values to 60 when the per-minute override is disabled.

## Evidence
- `pumps/mongo_mcdc_test.go:TestSetAggregationTime_GreaterThan60` verifies the
  high out-of-range branch defaults to 60.
- `pumps/mongo_mcdc_test.go:TestSetAggregationTime_LessThan1` verifies the low
  out-of-range branch defaults to 60.
- `pumps/mongo_mcdc_test.go:TestSetAggregationTime_ValidValuePreserved`
  verifies valid values are not defaulted.
- `pumps/mongo_aggregate_test.go:TestMongoAggregatePump_StoreAnalyticsPerMinute`
  verifies the per-minute override takes precedence over an invalid configured
  value.

## Open Questions
- Operators may expect an Init error for invalid values instead of silent
  defaulting; current product behavior is default-to-60.
