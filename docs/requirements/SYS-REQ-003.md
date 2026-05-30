# SYS-REQ-003: Aggregation windowed over configurable time bucket

## Intent
When aggregation is enabled, the pump windows aggregated analytics over a configurable time bucket — hourly by default, optionally per-minute via `store_analytics_per_minute`. This satisfies the parent **STK-REQ-001** at the aggregation-output side: aggregated rollups must be emitted on a deterministic cadence so downstream dashboards (Mongo/SQL aggregate collections) refresh predictably.

## Motivation
Operators choosing aggregated outputs (Mongo aggregate, SQL aggregate, MCP/GraphQL SQL aggregate) need a stable bucket boundary so that joining the rollup with raw analytics is meaningful and so that storage growth is bounded by `buckets_per_hour * orgs * dimensions`. Capturing windowing at the SYS layer codifies the contract that "aggregated_emitted" eventually happens and respects the configured window — a black-box promise independent of which aggregate pump implementation is used. Phase 0.6 split the original SYS-REQ-003: the dimension and counter-content obligations spun out to **SYS-REQ-018** and **SYS-REQ-019**.

## Formalization
```
when aggregation_enabled aggregation shall always satisfy aggregates_emitted
```
The input `aggregation_enabled` is true whenever a configured backend is an aggregate variant; the output `aggregates_emitted` becomes true when `AggregateData` produces a per-window map for the current bucket. Variables: `specs/system/variables/aggregation.vars.yaml`.

## Code references
- `analytics/aggregate.go:722 AggregateData` — entry that returns `map[orgID]AnalyticsRecordAggregate` for a window.
- `analytics/aggregate.go:749 setAggregateTimestamp(..., aggregationTime)` — selects the bucket boundary; called from inside `AggregateData`.
- `pumps/mongo_aggregate.go:55 StoreAnalyticsPerMinute` config + `:468-475` aggregation-time selection.
- `pumps/sql_aggregate.go:31 StoreAnalyticsPerMinute` + `:260-268` per-minute switch.
- `pumps/graph_sql_aggregate.go:136-138` and `pumps/mcp_sql_aggregate.go:161` — same toggle for the graph / MCP aggregate variants.

## Evidence
- `analytics/aggregate_test.go:77` (used by `TestAggregate_Tags`), `:415 TestSetAggregateTimestamp` — verify bucket-boundary math.
- `pumps/mongo_aggregate_test.go:479 TestMongoAggregatePump_StoreAnalyticsPerMinute`.
- `pumps/mcp_sql_aggregate_test.go:285`, `:607` — per-minute aggregation toggle.
- Satisfying SW child: **SW-REQ-011** (aggregate counter accumulation).

## Open questions
- Phase 0.6 split-offs: dimension grouping is now **SYS-REQ-018**; per-aggregated-unit counter content (hits/successes/errors/latency) is now **SYS-REQ-019**.
- The default window value (hourly) is encoded implicitly in `setAggregateTimestamp` (zeroing minutes/seconds when `aggregationTime != 1`); there is no SYS-level statement that the default is exactly 60 minutes.
- Acceptable window granularities are limited to "hour" or "minute" by the boolean toggle, but the FRETish does not bound the set.
