# INT-REQ-007: SQL schema migratable with older pumps still writing

## Intent
The SQL pumps write into day-sharded analytics tables (`tyk_analytics`,
`tyk_aggregated`, `tyk_uptime_analytics`, `tyk_mcp_aggregated`,
`tyk_graph_aggregated`, and optionally per-day-suffixed variants like
`tyk_analytics_20260530`). This requirement asserts that the schema for
those tables can be migrated forwards while older pumps are still writing,
following an expand-then-contract pattern (add nullable columns first;
remove old columns only after all writers have rolled forward). It
satisfies SYS-REQ-003.

## Motivation
SQL is the one backend where the schema is materialised inside the
backend (Mongo and Elastic are document stores; Kafka has no schema
beyond the message body). When the gateway grows a new analytics field,
that field has to land as a new column on `tyk_analytics` for the SQL
backend to record it — and operators almost always run multiple pump
instances behind a single SQL backend for HA, with some rolling forward
ahead of others. Without an expand-then-contract migration story, a pump
running old code would fail to write because the new column it doesn't
know about is `NOT NULL`, or an old column it does write to has been
dropped under it.

This is also why GORM's `AutoMigrate` is used (additive only by design)
rather than handwritten DDL.

## Code references
- SQL table-name constants:
  - `analytics/analytics.go:43` `SQLTable = "tyk_analytics"` (the raw
    request table).
  - `analytics/aggregate.go:23-24`
    `AggregateSQLTable = "tyk_aggregated"`,
    `AggregateGraphSQLTable = "tyk_graph_aggregated"`.
  - `analytics/aggregate_mcp.go:11` `AggregateMCPSQLTable = "tyk_mcp_aggregated"`.
  - `analytics/analytics.go` uses `UptimeSQLTable` for uptime records.
- Day-sharding pattern: `pumps/sql.go:309`
  `table := analytics.SQLTable + "_" + recDate` (where `recDate =
  TimeStamp.Format("20060102")`); similarly `pumps/sql.go:383` for the
  uptime sharded table.
- Migration entry points:
  - `pumps/common.go:117` `HandleTableMigration` — the shared switch
    between three modes:
    - `!conf.TableSharding`: `db.Table(tableName).AutoMigrate(model)` on
      the unsharded main table.
    - `conf.MigrateShardedTables`: scan and migrate *all* existing
      sharded tables (via `migrateAllFunc`).
    - default sharded case: migrate only today's table
      (`tableName + "_" + time.Now().Format("20060102")`).
  - `pumps/common.go:147` `MigrateAllShardedTables` — uses
    `information_schema.tables` per dialect (sqlite/mysql/postgres) to
    find tables matching `<prefix>_YYYYMMDD`, then `AutoMigrate`s each.
- `pumps/sql.go:240-258` wires the migration for the analytics pump (one
  branch for `IsUptime`, one for the main analytics table); the
  aggregate variants do the same at `pumps/sql_aggregate.go:108-114`,
  `pumps/mcp_sql_aggregate.go:66-69`, `pumps/graph_sql.go:75-78`.
- The expand-then-contract discipline is documented operator-side at
  `pumps/sql.go:85-89` (`MigrateShardedTables` field comment).

## Evidence
- `pumps/migration_test.go` covers `MigrateAllShardedTables` including
  the date-pattern matcher, the different-models case, and the
  empty-table case.
- `pumps/sql_test.go`, `pumps/sql_mysql_test.go`,
  `pumps/sql_pgxv5_test.go` exercise migrations on real DBs.

## Open questions
- "Expand-then-contract" is asserted by the requirement text but not
  enforced anywhere in code or CI — `AutoMigrate` is additive (it does
  not drop columns), but the schema-version policy is not pinned in any
  SYS/SW req. An operator who manually `ALTER TABLE ... DROP COLUMN` to
  reclaim space would silently break older pumps.
- The "scan all sharded tables" path is opt-in
  (`MigrateShardedTables=false` by default; see `pumps/sql.go:88-89`),
  which means by default only today's table picks up new columns. A
  rolled-out gateway producing data for a yesterday-dated record could
  hit a schema mismatch on the previous day's shard. Operator-visible
  behaviour is "logged warning, write skipped".
- There is no schema-version table; migration drift across pump instances
  is undetectable except by symptom.
