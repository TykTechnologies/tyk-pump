# SW-REQ-084: Mongo aggregate mixed average update target

Documents: SW-REQ-084

The Mongo aggregate pump writes each aggregate document with a first upsert for
counters and a second upsert for derived averages from `AsTimeUpdate()`.

## Contract

When the aggregate write is the mixed-collection variant, both upserts must use
the mixed collection identity. The second average-update object must preserve
`Mixed: true`; otherwise `TableName()` falls back to the per-org aggregate
collection and the shared mixed collection is left with stale derived averages.

## Evidence

- `pumps/mongo_aggregate_test.go:TestDoAggregatedWritingWithIgnoredAggregations`
  runs the live Mongo aggregate path with `use_mixed_collection: true`, writes
  nonzero request times, and reads back both the per-org and mixed collections.
  Both collections must contain the recalculated request-time average.

## Known Issues

This requirement does not claim aggregate replay idempotency or full concurrent
atomicity. Those remain tracked separately under Mongo aggregate KnownIssues.
