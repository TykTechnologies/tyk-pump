# SW-REQ-060: Mongo aggregate — `$inc` upsert semantics

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-036 (mongo-aggregate). It carries the counter-increment upsert
obligation in isolation.

## Intent
The MongoAggregate pump shall persist aggregation counters via a two-step
MongoDB upsert keyed by `{orgid, timestamp}`: step one applies
`$inc`/`$set`/`$max`/`$min` operators built by
`AnalyticsRecordAggregate.AsChange()` (preserving monotonicity of
hits/errors/success counters under concurrent writers via Mongo's
document-level `$inc` atomicity — see SYS-REQ-029), step two recalculates
derived averages via `AsTimeUpdate()`. The first upsert error shall be
returned to the caller. Derived from SYS-REQ-018 / SYS-REQ-019 (aggregation
counters); relies on SYS-REQ-029 (Mongo `$inc` atomicity assumption).

## Motivation
The counter-increment semantics are the heart of the aggregate pump's
correctness: multiple pumps may write to the same `{orgid, timestamp}`
document concurrently (one tyk-pump per app server in a typical HA
deployment), and the monotonic counters must remain consistent.
Outsourcing the atomicity guarantee to Mongo's per-document `$inc`
operator (documented in SYS-REQ-029) is the only honest way to satisfy
the obligation; the two-step pattern (counters then averages) is the
canonical pre-aggregation idiom.

## Code references
- `pumps/mongo_aggregate.go:MongoAggregatePump.DoAggregatedWriting` —
  the two-step upsert path.
- `analytics/aggregate.go:AnalyticsRecordAggregate.AsChange` —
  `$inc`/`$set`/`$max`/`$min` builder.
- `analytics/aggregate.go:AnalyticsRecordAggregate.AsTimeUpdate` —
  derived-average recalculation.

## Evidence
- The `atomicity` obligation's `negative` evidence is deferred: a
  fault-injecting MongoDB harness is required for an honest negative test
  and is not available in local CI today.
- Live-Mongo aggregate tests exercise the full pipeline; the on-disk
  representation is verified by `TestDoAggregatedWritingWithIgnoredAggregations`
  (annotated against SW-REQ-059) and the counters by
  `TestAggregationTime` (annotated against SW-REQ-058).

## Related requirements
`SW-REQ-096` decomposes the TT5516 mixed-collection ignored-dimension retention
case from this two-step upsert requirement. It proves that applying
`ignore_aggregations` to an incoming update does not delete existing dimension
counters already stored in the shared mixed aggregate document.

## Open questions
- The atomicity guarantee is inherited from SYS-REQ-029; if operators run
  Mongo with `writeConcern: 0` they break that assumption.
- Future negative test should run against a Mongo testcontainer with
  fault-injection (kill-mid-upsert, write-concern races).
