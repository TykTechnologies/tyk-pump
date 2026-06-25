# SW-REQ-062: Mongo aggregate — max-doc-size self-healing

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-036 (mongo-aggregate). It carries the document-size self-healing
obligation in isolation.

## Intent
When `EnableAggregateSelfHealing` is true and an upsert fails with a
max-document-size error from MongoDB (`'Size must be between 0 and'`),
CosmosDB (`'Request size is too large'`), or DocumentDB (`'Resulting
document after update is larger than'`), the MongoAggregate pump shall:
(a) halve `AggregationTime` (unless already 1, in which case it skips
self-healing and returns the error to the caller), (b) reset the
last-document-timestamp tracker so subsequent writes form a new document,
and (c) recursively re-invoke `WriteData` once with the same input data.
Self-healing only triggers on the listed size errors, not on arbitrary
write failures. Derived from SYS-REQ-004 (recoverable error handling).

## Motivation
The aggregate pump's per-window documents grow with the number of unique
dimensions written into them. Under heavy load with high-cardinality
dimensions, a window's document can exceed Mongo's 16 MiB cap mid-window.
Self-healing exists to bisect the window time-bucket transparently rather
than dropping data: halving `AggregationTime` produces shorter time
windows, which produce smaller documents. The recursion is bounded by the
`AggregationTime > 1` guard.

## Code references
- `pumps/mongo_aggregate.go:MongoAggregatePump.ShouldSelfHeal` — matches
  the three size-error strings.
- `pumps/mongo_aggregate.go:MongoAggregatePump.divideAggregationTime` —
  halve-or-floor-at-1.
- `pumps/mongo_aggregate.go:MongoAggregatePump.WriteData` — the recursive
  call after `divideAggregationTime`.

## Evidence
- `pumps/mongo_aggregate_test.go:TestMongoAggregatePump_ShouldSelfHeal`
  (re-annotated `Verifies: SW-REQ-062`) — exercises the predicate matrix
  (random error, cosmos error, standard mongo error, doc-db error,
  disabled flag, aggregation-time-already-1).
- `pumps/mongo_aggregate_test.go:TestMongoAggregatePump_ShouldSelfHealResetsTimestampTracker`
  — verifies the non-container timestamp-reset side effect: a size error
  clears the last-document tracker so the next aggregate opens a new bucket,
  while a non-size write error keeps the current bucket.
- `pumps/mongo_aggregate_test.go:TestMongoAggregatePump_WriteDataSelfHealRetryWiring`
  — verifies the `WriteData` self-heal branch is wired to call
  `ShouldSelfHeal(err)` and retry the same `ctx`/`data` batch without running
  the historical 16 MiB stress path.
- `pumps/mongo_aggregate_test.go:TestMongoAggregatePump_divideAggregationTime`
  (re-annotated `Verifies: SW-REQ-062`) — exercises the halve / floor-at-1
  helper.

## Open questions
- Recursion depth is bounded by `AggregationTime` halving from 60 to 1
  (≤6 levels) — no risk of stack overflow.
- The three size-error strings are matched by substring; if Mongo /
  Cosmos / DocDB change their messages this matcher silently degrades.
  Could be a follow-up to use error-code matching where available.
