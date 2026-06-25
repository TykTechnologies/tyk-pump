# SW-REQ-066: SQL aggregate — composite index ensure with optional `CONCURRENTLY`

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-041 (sql-aggregate). It carries the index lifecycle obligation in
isolation.

## Intent
The SQL Aggregate pump shall, unless `OmitIndexCreation` is true, ensure
a composite index named `<table>_idx_dimension` on `(dimension,
timestamp, org_id, dimension_value)` via
`CREATE INDEX [CONCURRENTLY] IF NOT EXISTS`. For PostgreSQL the
`CONCURRENTLY` option shall be used and, in non-sharded mode, the index
creation shall run on a background goroutine signalling
`backgroundIndexCreated` on completion. For MySQL the index creation
shall run synchronously without `CONCURRENTLY`. Index creation is
skipped when the index already exists. Derived from SYS-REQ-004.

## Motivation
The composite index drives the dashboard's per-dimension query path:
operators query "all aggregate rows for `dimension=apiid AND org_id=X
AND dimension_value=Y AND timestamp BETWEEN ...`", and without the index
this becomes a table scan per query. `CONCURRENTLY` avoids blocking
writes on Postgres (which holds an exclusive lock during normal
`CREATE INDEX`); MySQL doesn't support that option so a sync create is
used. The background-goroutine pattern in non-sharded mode lets Init
return without waiting for the index to finish; tests that exercise
`ensureIndex` directly must pre-allocate the `backgroundIndexCreated`
channel to avoid hangs.

## Code references
- `pumps/sql_aggregate.go:SQLAggregatePump.ensureIndex` — the
  `CREATE INDEX` wrapper with the `CONCURRENTLY` / sync dispatch.
- `pumps/sql_aggregate.go:SQLAggregatePump.backgroundIndexCreated` — the
  signalling channel.

## Evidence
- `pumps/sql_aggregate_test.go:TestEnsureIndexSQLAggregate` (re-annotated
  `Verifies: SW-REQ-066`) — exercises the five index-creation scenarios
  including the negative "index created on non-existing table" case and
  the `OmitIndexCreation: true` short-circuit.
- `pumps/sql_aggregate_test.go:assertSQLAggregateIndexDefinition` reads the
  Postgres catalog and asserts the created index key order is exactly
  `(dimension, timestamp, org_id, dimension_value)`.
- Live-Postgres tests are excluded from the local audit MC/DC scope
  (known issue).

## Obligations
- `index_definition_matches_query` — the physical index definition must match
  the documented dashboard query path, not merely exist under the expected
  name.
- `backend_ddl_valid` is deferred to
  `sql-aggregate-mysql-create-index-if-not-exists-unsupported` for current
  MySQL DDL incompatibility.
- `concurrent` is deferred to
  `sql-aggregate-background-index-concurrency-unbounded` until the background
  index lifecycle has bounded failure/concurrency evidence.

## Open questions
- The background-goroutine pattern means Init can return before the
  index is ready; queries arriving in that window may table-scan. Worth
  documenting for operator runbooks.
- `MCPSQLAggregatePump` (SW-REQ-045) re-uses the same pattern; the
  obligation applies there too.
