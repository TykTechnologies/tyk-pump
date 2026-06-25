# SW-REQ-096: Mongo aggregate ignored-dimension retention

## Intent
When `MongoAggregatePump` writes to the shared mixed collection with
`ignore_aggregations` configured, it shall omit those dimensions from the
incoming update only. It must not delete dimension counters that already exist
in the mixed aggregate document from another writer or earlier batch.

## Motivation
TT5516 fixed a double-discard bug. The historical implementation discarded
ignored dimensions before the first counter upsert, then discarded them again in
the mixed-collection average path after Mongo had merged existing data. That
second discard could erase dimensions such as `apikeys` that had been written
by another aggregate pump without the ignore setting.

## Formalization
```
when ignore_aggregation_configured & mixed_collection_dimension_present pumps_mongo_aggregate shall always satisfy ignored_dimension_retained
```

Variables are declared in `specs/software/variables/pumps-mongo-aggregate.vars.yaml`.

## Code References
- `pumps/mongo_aggregate.go:MongoAggregatePump.DoAggregatedWriting` applies
  `DiscardAggregations` once, before building the incoming update document.
- `analytics/aggregate.go:AnalyticsRecordAggregate.DiscardAggregations` removes
  configured dimension maps from the incoming aggregate value.

## Evidence
- `pumps/mongo_aggregate_test.go:TestDoAggregatedWritingWithIgnoredAggregations`
  writes a mixed aggregate with `ignore_aggregations: ["apikeys"]`, writes a
  second aggregate that includes API-key dimensions, writes the ignoring
  aggregate again, and asserts the mixed collection still retains the non-ignored
  `apikey2` counter.
