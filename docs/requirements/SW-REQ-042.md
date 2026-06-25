# SW-REQ-042: Graph SQL pump — GraphQL record forwarding to SQL with optional sharding

## Intent
The `GraphSQLPump` shall, on each purge, retain only records for which
`AnalyticsRecord.IsGraphRecord()` is true, convert each to a `GraphRecord`
while preserving GraphQLStats-derived `RootFields` in the persisted graph row,
then if `TableSharding` is true split on each `YYYYMMDD` boundary and route each
day-slice to a `<TableName>_<YYYYMMDD>` table (auto-migrated when missing);
otherwise route all records to the configured `TableName` (default
`tyk_analytics_graph`). Within each table the pump shall issue
parameter-bound batched inserts of `BatchSize` records using `gorm.Create`.
Per-batch errors are logged but not propagated. Derived from SYS-REQ-004
via Phase A decomposition of SW-REQ-019.

## Motivation
The Graph SQL pump is the SQL analogue of the Graph Mongo pump
(SW-REQ-037): it keeps GraphQL-specific analytics out of the main analytics
table so the standard analytics queries do not have to skip-filter graph
records. Splitting it out clarifies that — unlike the standard SQL pump —
this writer applies a hard filter on `IsGraphRecord()` (rather than
silently inserting non-graph records) and uses a separate default table.

## Code references
- `pumps/graph_sql.go:GraphSQLPump.Init` — table-name default, dialect
  dispatch.
- `pumps/graph_sql.go:GraphSQLPump.WriteData` — day-bucket loop with
  `IsGraphRecord()` filter.
- `pumps/graph_sql.go:GraphSQLPump.getGraphRecords` — `AnalyticsRecord` →
  `GraphRecord` conversion.

## Evidence
- `pumps/graph_sql_test.go` (re-annotated `Verifies: SW-REQ-042`).
- `pumps/graph_sql_test.go:TestGraphSQLPump_WriteData` proves graph SQL
  persistence/readback includes `RootFields` from the projected `GraphRecord`.
- Live-Postgres tests are excluded from the local audit MC/DC scope (known
  issue).

## Open questions
- Per-batch errors logged but not propagated (same as standard SQL pump).
- Day-bucket algorithm duplicated from sql.go.
- Records failing the `IsGraphRecord()` filter are silently dropped — unlike
  Graph Mongo (SW-REQ-037) which still inserts them. The two pumps disagree
  on the right behaviour; honest description above documents the SQL pump's
  drop-on-mismatch policy.
