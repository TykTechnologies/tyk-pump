# SW-REQ-044: MCP SQL pump — MCP record forwarding to SQL with optional sharding

## Intent
The `MCPSQLPump` shall, on each purge, retain only records for which
`AnalyticsRecord.IsMCPRecord()` is true, convert each to an `MCPRecord`,
then if `TableSharding` is true split on each `YYYYMMDD` boundary and route
each day-slice to a `<TableName>_<YYYYMMDD>` table (auto-migrated when
missing); otherwise route all records to the configured `TableName` (default
`tyk_analytics_mcp`). Within each table the pump shall issue parameter-bound
batched inserts of `BatchSize` records using `gorm.Create`. Per-batch errors
are logged but not propagated. Derived from SYS-REQ-004 via Phase A
decomposition of SW-REQ-019.

## Motivation
The MCP SQL pump is the SQL analogue of the MCP Mongo pump (SW-REQ-038):
it keeps MCP records out of the main analytics table so the standard
analytics queries do not have to skip-filter MCP records. This split
exists to make MCP-specific record handling explicit in SQL (separate
table, separate index lifecycle, MCP-only schema).

## Code references
- `pumps/mcp_sql.go:MCPSQLPump.WriteData` — orchestrator.
- `pumps/mcp_sql.go:getMCPRecords` — `IsMCPRecord()` filter +
  `AnalyticsRecord` → `MCPRecord` conversion.
- `pumps/mcp_sql.go:writeMCPBatch` — `gorm.Create` per batch.
- `pumps/mcp_sql.go:ensureMCPShardedTable` — auto-migration on missing
  sharded tables.

## Evidence
- `pumps/mcp_sql_test.go` (re-annotated `Verifies: SW-REQ-044`).
- Live-Postgres tests are excluded from the local audit MC/DC scope (known
  issue).

## Open questions
- Per-batch errors logged but not propagated (same as the standard SQL
  pump). Honest obligation_class is `parameterized_only_write` plus
  `connection_leak_free`, not `errors_propagated`.
- Day-bucket algorithm duplicated from sql.go.
