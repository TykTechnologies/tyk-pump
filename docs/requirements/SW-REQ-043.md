# SW-REQ-043: Graph SQL aggregate pump — per-(API, dimension) upsert with day sharding

## Intent
The `GraphSQLAggregatePump` shall, on each purge, day-bucket records when
`TableSharding` is enabled and route each day-slice to a
`<AggregateGraphSQLTable>_<YYYYMMDD>` table (auto-migrated when missing).
Within each table the pump shall aggregate graph analytics via
`analytics.AggregateGraphData` (windowed per minute when
`StoreAnalyticsPerMinute` is true, otherwise per hour) and upsert each
per-(API, graph dimension) row, including `types`, `fields`, `operation`, and
`rootfields`, with `clause.OnConflict` on `id` using
`analytics.OnConflictAssignments`. Per-batch errors *shall be returned* to the
caller. Derived from SYS-REQ-003 via Phase A decomposition of SW-REQ-019.

## Motivation
This is the only graph-flavoured SQL writer that propagates errors — unlike
the standard sql / graph-sql / mcp-sql pumps which swallow them. Splitting
it out makes the propagation guarantee explicit so reviewers do not assume
the family-level swallow-errors behaviour applies. The pump exists to give
operators per-API GraphQL operation-level aggregates (counts, hits,
latencies) in SQL form, suitable for Grafana / Metabase dashboards.

## Code references
- `pumps/graph_sql_aggregate.go:GraphSQLAggregatePump.WriteData` — day-bucket
  loop and table ensure.
- `pumps/graph_sql_aggregate.go:DoAggregatedWriting` — `clause.OnConflict`
  upsert via GORM; returns errors to the caller.

## Evidence
- `pumps/graph_sql_aggregate_test.go` (re-annotated `Verifies: SW-REQ-043`).
- `analytics/aggregate_test.go:TestAggregateGraphData_PartitionsSameOrgByAPIID`
  proves that same-org, same-dimension GraphQL records remain isolated by
  `APIID` before SQL upsert, including the `rootfields` dimension.
- SW-REQ-086 decomposes the sharded table-target invariant and is witnessed by
  `pumps/graph_sql_aggregate_test.go:TestGraphSQLAggregatePump_WriteData_Sharded`.
- Live-Postgres tests are excluded from the local audit MC/DC scope (known
  issue).

## Obligations
- `aggregate_partition_isolated` — aggregate keys include the declared API
  partition dimension so same-org GraphQL counters for different APIs cannot
  collapse into one row.
- `routing_target_consistent` — decomposed to SW-REQ-086 for the selected day
  shard table used by `WriteData` and `DoAggregatedWriting`.
- `atomicity`, `transaction_isolation_declared`, and `errors_propagated` —
  required for backend write correctness and tracked under KnownIssue
  `graph-sql-aggregate-atomicity-fault-injection-missing` until a database
  transaction/failure-injection harness proves serialization/deadlock and
  forced `tx.Error` behavior.

## Open questions
- The day-bucket algorithm is duplicated from sql.go / sql_aggregate.go /
  graph_sql.go.
- Aggregation windowing is per-minute or per-hour (same model as
  SW-REQ-058 for mongo-aggregate); a uniform windowing helper across all
  aggregate pumps would simplify reasoning.
