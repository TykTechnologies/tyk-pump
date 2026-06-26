# SW-REQ-067: SQL aggregate — on-conflict upsert with parameter binding

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-041 (sql-aggregate). It carries the on-conflict upsert obligation
in isolation.

## Intent
The SQL Aggregate pump shall, per (orgID, dimension) row in each
aggregated batch, issue a parameter-bound `INSERT ... ON CONFLICT (id)
DO UPDATE SET ...` using `clause.OnConflict` with
`analytics.OnConflictAssignments(table, "excluded")` so that monotonic
counter columns are incremented rather than overwritten. Insert failures
shall be returned to the caller. Records are batched in slices of
`BatchSize`. Derived from SYS-REQ-018 / SYS-REQ-019 (aggregation
counters).

The order in which aggregate rows are inserted is intentionally
unspecified. `AggregateData(...).Dimensions()` derives rows from Go map
state, so callers must not infer ordering from the generated batch.
Correctness is keyed by the deterministic aggregate row id and the
`ON CONFLICT (id)` merge semantics; replay or concurrent writes must
accumulate counters for the same id regardless of row order.

## Motivation
The on-conflict upsert is the SQL equivalent of mongo-aggregate's
two-step `$inc` upsert (SW-REQ-060): multiple pumps may write to the
same `(orgID, dimension, dimension_value, timestamp)` row concurrently
(one tyk-pump per app server), and the monotonic counters must remain
consistent. PostgreSQL's `ON CONFLICT DO UPDATE` is the canonical idiom
for that; the `OnConflictAssignments` helper builds the per-column
`SET counter = excluded.counter + table.counter` clauses.

## Code references
- `pumps/sql_aggregate.go:SQLAggregatePump.DoAggregatedWriting` — the
  `clause.OnConflict` upsert path.
- `analytics/sql_aggregate.go:OnConflictAssignments` — builds the
  per-column increment clauses.

## Evidence
- `pumps/sql_aggregate_test.go:TestSQLAggregateWriteData` (re-annotated
  `Verifies: SW-REQ-067`) — exercises four sequential writes verifying
  hits accumulate (3 → 6 → 9 in the same hour, then 3 in a new hour).
- `pumps/sql_aggregate_test.go:TestSQLAggregateWriteDataValues`
  (re-annotated `Verifies: SW-REQ-067`) — exercises the
  on-conflict update with latency / request-time recomputation across
  two writes.
- The `atomicity` obligation's `negative` evidence is deferred: a
  transaction-failure injection harness around the GORM session is required
  for an honest negative test and is not present today.
- The `ordering_guarantees_documented` obligation is documentation-only for
  this requirement: SQL aggregate row order is explicitly unspecified and is
  not a runtime guarantee.

## Open questions
- Parameter binding is provided by GORM; the obligation
  `parameterized_only_write` is satisfied transitively via the GORM
  contract.
- Future negative test should inject a unique-constraint violation
  mid-upsert (e.g. via savepoint manipulation) to exercise the error-
  return path.
