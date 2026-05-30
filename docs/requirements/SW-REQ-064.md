# SW-REQ-064: SQL aggregate — day-bucket batching with date-boundary split

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-041 (sql-aggregate). It carries the day-bucket routing obligation
in isolation.

## Intent
When `TableSharding` is enabled, the SQL Aggregate pump shall scan the
incoming records in their existing order and, on each `YYYYMMDD`
timestamp boundary, route the preceding contiguous slice to a
`tyk_aggregated_<YYYYMMDD>` table (ensured via `ensureTable` and indexed
via `ensureIndex`); the trailing slice shall be routed using the last
record's date. When `TableSharding` is disabled, all records shall be
routed to the single `tyk_aggregated` table in one pass. Derived from
SYS-REQ-003.

## Motivation
Day-bucketed sharding is the standard mitigation for unbounded SQL
aggregate-table growth: each per-day table can be dropped wholesale for
retention rather than running expensive `DELETE WHERE timestamp < ...`
statements. The boundary-split algorithm assumes records are roughly
timestamp-sorted (which the pump loop normally produces); out-of-order
records still get routed correctly but may force extra table-ensure
calls.

## Code references
- `pumps/sql_aggregate.go:SQLAggregatePump.WriteData` — the day-bucket
  loop that scans `YYYYMMDD` boundaries.
- `pumps/sql_aggregate.go:ensureTable`, `:ensureIndex` — invoked per
  boundary.

## Evidence
- `pumps/sql_aggregate_test.go:TestSQLAggregateWriteData_Sharded`
  (re-annotated `Verifies: SW-REQ-064`) — exercises 3 distinct day
  buckets and asserts per-day row counts.
- Live-Postgres tests are excluded from the local audit MC/DC scope
  (known issue).

## Open questions
- The day-bucket algorithm is duplicated across sql.go, graph_sql.go,
  graph_sql_aggregate.go, mcp_sql.go, and mcp_sql_aggregate.go;
  abstracting would be a Phase-B follow-up.
- The boundary computation uses UTC dates; operators in non-UTC zones
  may see day-bucket boundaries that don't align with local-midnight.
