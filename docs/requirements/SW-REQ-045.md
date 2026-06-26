# SW-REQ-045: MCP SQL aggregate pump — per-(API, dimension) upsert with day sharding

## Intent
The `MCPSQLAggregatePump` shall, on each purge, day-bucket records when
`TableSharding` is enabled and route each day-slice to a
`<AggregateMCPSQLTable>_<YYYYMMDD>` table (auto-created via `CreateTable`
when missing). Within each table the pump shall aggregate MCP records via
`analytics.AggregateMCPData` (windowed per minute when
`StoreAnalyticsPerMinute` is true, otherwise per hour) and upsert each
per-(API, dimension) row with `clause.OnConflict` on `id` using
`analytics.OnConflictAssignments`. Per-batch errors *shall be returned* to
the caller. On initialisation, a composite index on `(dimension, timestamp,
org_id, dimension_value)` shall be created (Postgres uses `CONCURRENTLY` and
creates it on a background goroutine). Derived from SYS-REQ-003 via Phase A
decomposition of SW-REQ-019.

## Motivation
This pump is the MCP analogue of the SQL aggregate pump (SW-REQ-041) and
propagates errors (unlike the non-aggregate MCP SQL pump in SW-REQ-044).
Splitting it out makes the propagation guarantee explicit, and the
composite-index strategy mirrors the one used by SW-REQ-066 for the
non-MCP aggregate variant.

## Code references
- `pumps/mcp_sql_aggregate.go:MCPSQLAggregatePump.WriteData` — orchestrator.
- `pumps/mcp_sql_aggregate.go:DoAggregatedWriting` — `clause.OnConflict`
  upsert.
- `pumps/mcp_sql_aggregate.go:ensureTable`, `:ensureIndex`,
  `:ensureMCPAggregateShardedTable` — lifecycle hooks.
- `pumps/mcp_sql_aggregate.go:aggregationTimeMinutes` — window selector.
- `pumps/mcp_sql_aggregate.go:writeAggregatedSlice` — per-slice upsert
  emitter.

## Evidence
- `pumps/mcp_sql_aggregate_test.go` (re-annotated `Verifies: SW-REQ-045`).
- Live-Postgres tests are excluded from the local audit MC/DC scope (known
  issue).

## Open questions
- Background index creation channel must be pre-allocated by tests that
  call `ensureIndex` directly (same caveat as SW-REQ-066).
- The `invariant_preservation` obligation from the create-table-without-sync
  signal is deferred to KnownIssue
  `mcp-sql-aggregate-background-index-concurrency-unbounded`; this is accepted
  KI debt, not covered behavior.
- Aggregation window helper is duplicated from SW-REQ-041 / SW-REQ-043.
