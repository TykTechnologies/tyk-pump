# SW-REQ-040: SQL standard pump — batched parameterised inserts with optional table sharding

## Intent
The standard SQL pump (`SQLPump`) shall, on each purge, drop MCP-classified
analytics records, then if `TableSharding` is true split the remaining
records on each `YYYYMMDD` timestamp boundary and route each day-slice to a
`tyk_analytics_<YYYYMMDD>` table (auto-created and indexed via
`ensureTable`/`ensureIndex`); otherwise route all records to the default
`tyk_analytics` table. Within each table the pump shall issue
parameter-bound batched inserts of `BatchSize` records (default 1000) using
`gorm.Create` on the request context. The Postgres backend shall install a
custom `monthEncodePlan` `AfterConnect` callback to encode `time.Month` as
`int8` (TT-16980). Per-batch insert errors are logged but not propagated
upward; `WriteData` always returns `nil`. Derived from SYS-REQ-004 via Phase
A decomposition of SW-REQ-019.

## Motivation
The SQL pump is the recommended target for operators who want analytics in a
relational store (typically Postgres for high-write deployments, MySQL for
legacy installs). Day-bucketed sharding is the standard mitigation for
unbounded table growth — when `TableSharding` is enabled the pump creates one
table per UTC day, which lets operators run cheap `DROP TABLE` retention
instead of `DELETE WHERE timestamp < ...`. Splitting this requirement out of
the SQL family makes the non-aggregate path's behaviour (no upsert, no
on-conflict, pure inserts) explicit so it does not get conflated with the
aggregate variants.

## Code references
- `pumps/sql.go:SQLPump.Init` — config normalisation, Dialect dispatch,
  optional Postgres `AfterConnect` install.
- `pumps/sql.go:Dialect` / `monthEncodePlan` — Postgres-only `time.Month`
  encoding fix.
- `pumps/sql.go:SQLPump.WriteData` — day-bucket loop and per-table
  `gorm.Create` batching.
- `pumps/sql.go:327-328` — per-batch error logging without propagation.
- `pumps/sql.go:ensureTable`, `:ensureIndex`, `:createIndex` —
  table/index lifecycle (`CONCURRENTLY` on Postgres index creation; MySQL
  skips the option).
- `pumps/sql.go:WriteUptimeData` — sibling write path used by the legacy
  uptime stream.

## Evidence
- `pumps/sql_test.go`, `pumps/sql_mysql_test.go`, `pumps/sql_pgxv5_test.go`
  (all re-annotated `Verifies: SW-REQ-040`).
- Live-Postgres/MySQL tests are excluded from the local audit MC/DC scope
  (known issue).

## Open questions
- `WriteData` always returns `nil` even on per-batch errors (line 327-328
  just logs); upstream retry / backpressure cannot react to per-batch
  failures. Honest obligation_class is `parameterized_only_write` plus
  `connection_leak_free`, not `errors_propagated`.
- The day-bucket algorithm is duplicated in sql_aggregate, graph_sql, and
  mcp_sql — abstracting is out-of-scope for Phase A but worth a follow-up.
- The Postgres-only `CONCURRENTLY` flag means MySQL index creation locks
  the table for the duration of the create.
