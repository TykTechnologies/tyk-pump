# SW-REQ-065: SQL aggregate — dimension table ensure

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-041 (sql-aggregate). It carries the per-table CreateTable
obligation in isolation.

## Intent
The SQL Aggregate pump shall idempotently create the target aggregate
table (default `tyk_aggregated` or a sharded `tyk_aggregated_<YYYYMMDD>`)
via GORM `CreateTable` against the `SQLAnalyticsRecordAggregate` schema
whenever the table is absent. For non-sharded mode this is done once at
`Init`; for sharded mode it is done on each new date boundary in
`WriteData`. Derived from SYS-REQ-004.

## Motivation
Auto-creating the aggregate table removes a manual migration step from
operator onboarding (operators don't have to run a separate schema-init
script). The idempotency guarantee means re-runs after a transient
failure don't double-create or corrupt the schema. This obligation is
split from the index-ensure (SW-REQ-066) because they have different
failure modes — table-ensure failures are usually permission errors,
index-ensure failures are usually duplicate-key races.

## Code references
- `pumps/sql_aggregate.go:SQLAggregatePump.ensureTable` — the
  `CreateTable` wrapper.
- `pumps/sql_aggregate.go:SQLAggregatePump.Init` — invokes `ensureTable`
  once for non-sharded mode.
- `pumps/sql_aggregate.go:SQLAggregatePump.WriteData` — invokes
  `ensureTable` on each new date boundary for sharded mode.

## Evidence
- `pumps/sql_aggregate_test.go:TestSQLAggregateInit` (re-annotated
  `Verifies: SW-REQ-041` parent) — exercises table-ensure on Init.
- `pumps/sql_aggregate_test.go:TestSQLAggregateWriteData_Sharded`
  (annotated against SW-REQ-064) — exercises per-day table ensure.
- Live-Postgres tests are excluded from the local audit MC/DC scope
  (known issue).

## Open questions
- No isolated unit test for `ensureTable` — the path is exercised only
  via the broader Init / WriteData tests.
- The schema is `SQLAnalyticsRecordAggregate`; if that struct's columns
  drift the table-ensure silently creates the new shape, which can break
  on-conflict assignments mid-deployment.
