# SW-REQ-041: SQL aggregate pump — parent requirement for per-dimension upserts

## Intent
The `SQLAggregatePump` shall aggregate analytics records per organisation
and per dimension and upsert them into a SQL table (sharded per day when
`TableSharding` is enabled). This parent anchors the four
per-significant-behaviour sub-requirements that carry the substantive
obligations: SW-REQ-064 (day-bucket batching with date-boundary split),
SW-REQ-065 (dimension-table ensure), SW-REQ-066 (composite index ensure
with optional `CONCURRENTLY`), and SW-REQ-067 (on-conflict upsert with
parameter binding). Derived from SYS-REQ-003 via Phase A decomposition of
SW-REQ-019.

## Motivation
The SQL aggregate pump is the SQL counterpart of the mongo-aggregate
pump: instead of per-org collection upserts, it emits per-(org, dimension)
rows whose monotonic counter columns are incremented via
`INSERT ... ON CONFLICT (id) DO UPDATE SET ...` using
`analytics.OnConflictAssignments`. The four sub-behaviours each have
distinct correctness criteria (date-boundary splitting, table ensure,
background index creation, on-conflict assignment list) and the
decomposition lets each carry its own obligation. This parent carries
`nominal` because the sub-reqs carry the real obligations.

## Code references
- `pumps/sql_aggregate.go:SQLAggregatePump.WriteData` — orchestrator with
  the day-bucket loop.
- `pumps/sql_aggregate.go:DoAggregatedWriting` — per-(org, dimension)
  on-conflict upsert call.
- `pumps/sql_aggregate.go:ensureTable`, `:ensureIndex` — lifecycle hooks
  used by the WriteData loop and by Init.

## Evidence
- `pumps/sql_aggregate_test.go` (re-annotated to point at the relevant
  sub-req per test function):
  - `TestSQLAggregateInit` /
    `TestDecodeRequestAndDecodeResponseSQLAggregate` → SW-REQ-041 parent.
  - `TestSQLAggregateWriteData_Sharded` → SW-REQ-064.
  - `TestEnsureIndexSQLAggregate` → SW-REQ-066.
  - `TestSQLAggregateWriteData` /
    `TestSQLAggregateWriteDataValues` → SW-REQ-067.
- Live-Postgres tests are excluded from the local audit MC/DC scope
  (known issue).

## Open questions
- Day-bucket algorithm is duplicated across sql.go, sql_aggregate.go,
  graph_sql.go, and mcp_sql.go.
- Background index creation uses an unbuffered channel
  (`backgroundIndexCreated`); tests that call `ensureIndex` directly must
  pre-allocate the channel to avoid hangs.
- The previous family req SW-REQ-019 is retained as a `[SUPERSEDED by Phase A
  decomposition: ...]` anchor.
