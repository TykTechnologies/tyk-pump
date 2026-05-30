# SW-REQ-043: Graph SQL aggregate pump — per-(API, dimension) upsert with day sharding

## Intent
The `GraphSQLAggregatePump` shall, on each purge, day-bucket records when
`TableSharding` is enabled and route each day-slice to a
`<AggregateGraphSQLTable>_<YYYYMMDD>` table (auto-migrated when missing).
Within each table the pump shall aggregate graph analytics via
`analytics.AggregateGraphData` (windowed per minute when
`StoreAnalyticsPerMinute` is true, otherwise per hour) and upsert each
per-(API, dimension) row with `clause.OnConflict` on `id` using
`analytics.OnConflictAssignments`. Per-batch errors *shall be returned* to
the caller. Derived from SYS-REQ-003 via Phase A decomposition of
SW-REQ-019.

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
- Live-Postgres tests are excluded from the local audit MC/DC scope (known
  issue).

## Open questions
- The day-bucket algorithm is duplicated from sql.go / sql_aggregate.go /
  graph_sql.go.
- Aggregation windowing is per-minute or per-hour (same model as
  SW-REQ-058 for mongo-aggregate); a uniform windowing helper across all
  aggregate pumps would simplify reasoning.
