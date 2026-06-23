# SW-REQ-019: SQL pump family — parameter-bound writes with day-sharded tables

## Intent
The SQL pump family shall write analytics records (and aggregates, for the
aggregate variants) to SQL backends via GORM, using parameter-bound `Create`
statements and optionally sharding into per-day tables (`<table>_YYYYMMDD`)
when `TableSharding` is enabled. Six implementations share the family:
standard, aggregate, two graph variants, and two MCP variants. Derived from
SYS-REQ-004 (independent per-backend delivery).

## Motivation
SQL backends (Postgres, MySQL, SQLite) are widely used for self-hosted
analytics warehousing and downstream BI tooling. The day-sharding strategy
addresses two concrete problems: (a) unbounded analytics tables become
expensive to drop or rotate, and (b) per-day tables let operators delete or
archive old data with a single `DROP TABLE` rather than a long-running
`DELETE`. The trade-off is per-write complexity: the sharding loop in
`SQLPump.WriteData` (`sql.go:288-333`) iterates through records, detects
date-boundary changes, and migrates a new table on the fly via `ensureTable`.

Common GORM concerns (log level translation, dialect selection, connection
open) are centralised in `pumps/common.go`'s `OpenGormDB`, and table-migration
helpers (`HandleTableMigration`, `MigrateAllShardedTables`) are shared by the
standard, aggregate, graph-standard, and MCP SQL variants. Graph SQL aggregate
still uses the older `if !TableSharding { AutoMigrate(...) }` startup shape and
does not honor `migrate_sharded_tables` for existing shards; this is tracked as
KI `graph-sql-aggregate-migrate-sharded-tables-ignored`. Parameter-bound
`Create` is what GORM produces by default and protects against SQL injection
from analytics fields containing user-supplied content.

## Code references
- `pumps/sql.go:78` — `TableSharding bool` field on `SQLConf`.
- `pumps/sql.go:269-338` — `SQLPump.WriteData` with date-boundary detection
  (`TimeStamp.Format("20060102")`) and per-day `ensureTable` migration.
- `pumps/sql.go:341-433` — `WriteUptimeData` uses the same sharding pattern for
  uptime data.
- `pumps/sql_aggregate.go:218-286` — aggregate variant, also sharded.
- `pumps/graph_sql.go:117`, `pumps/graph_sql_aggregate.go:93` — graph-record
  variants.
- `pumps/mcp_sql.go:143`, `pumps/mcp_sql_aggregate.go:195` — MCP-record
  variants.
- `pumps/common.go:117-208` — `HandleTableMigration`, `MigrateAllShardedTables`
  reused by every variant.

## Evidence
- `pumps/sql_test.go`, `pumps/sql_aggregate_test.go`, `pumps/sql_mysql_test.go`,
  `pumps/sql_pgxv5_test.go`, `pumps/graph_sql_test.go`,
  `pumps/graph_sql_aggregate_test.go`, `pumps/mcp_sql_test.go`,
  `pumps/mcp_sql_aggregate_test.go`, `pumps/migration_test.go`.
- MySQL/Postgres-backed tests require external services and are excluded from
  the local audit MC/DC scope (recorded as a known issue); SQLite-backed tests
  exercise the sharding logic locally.

## Open questions
- Same shape as SW-REQ-018: six implementations with materially different
  guarantees (graph filters to graph records, MCP filters to MCP records,
  aggregate pre-aggregates per-org buckets). Phase A should split into
  per-implementation reqs.
- `SQLPump.WriteData` mutates `c.db` in-place via `c.db = c.db.Table(...)`
  inside the sharding loop. This is correct for GORM (each call returns a
  scoped session) but is non-obvious and could be a latent concurrency bug
  if the same pump instance were ever called from multiple goroutines — the
  current core design serialises per-pump but the requirement doesn't capture
  that constraint.
- Errors from `tx := c.db.Create(...)` are logged with `c.log.Error(tx.Error)`
  but not propagated to the caller — `WriteData` returns `nil` regardless.
  The requirement text doesn't claim error propagation, but this is a real
  contrast with SW-REQ-018 and worth an explicit Phase-A obligation.
- Uptime-data path uses a different aggregation function and conflict
  resolution (`OnConflict` via `OnConflictUptimeAssignments`); not currently
  reflected in any req.
