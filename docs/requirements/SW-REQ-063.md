# SW-REQ-063: Mongo aggregate — index ensure lifecycle

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-036 (mongo-aggregate). It carries the per-org-collection index
ensure obligation in isolation.

## Intent
On each `DoAggregatedWriting` call (per-org), the MongoAggregate pump
shall idempotently ensure baseline indexes on the target aggregate
collection: (a) a TTL index on `expireAt` with TTL=0 (skipped for
CosmosDB), (b) a `timestamp` index, (c) an `orgid` index. Index creation
shall be skipped when `OmitIndexCreation` is true or, for StandardMongo,
when the collection already exists. Background index creation is used for
StandardMongo; foreground for CosmosDB / DocumentDB. Derived from
SYS-REQ-004.

## Motivation
The aggregate documents are queried by `{orgid, timestamp}` and aged out
via the `expireAt` TTL index; without these indexes the dashboard
backend's queries degrade dramatically (table-scan per query) and the
collection grows unbounded. Background index creation avoids blocking
write throughput on StandardMongo; CosmosDB and DocumentDB do not
support background creation so the pump runs foreground there.

## Code references
- `pumps/mongo_aggregate.go:MongoAggregatePump.ensureIndexes` — the
  baseline index ensure (TTL, timestamp, orgid).
- `pumps/mongo_aggregate.go:MongoAggregatePump.collectionExists` —
  short-circuits the ensure for StandardMongo when the collection is
  pre-existing.

## Evidence
- Index ensure is exercised indirectly by the live-Mongo aggregate tests
  (which require a fresh collection per test); no isolated unit test for
  `ensureIndexes` exists today.
- Live-Mongo tests are excluded from the local audit MC/DC scope (known
  issue).

## Related requirements
`SW-REQ-099` decomposes the TT-5302 DocumentDB index compatibility rule for
this aggregate index path: DocumentDB must not use the StandardMongo
collection-exists shortcut and must attempt foreground baseline indexes unless
`omit_index_creation` is set.

## Open questions
- `MCPMongoAggregatePump` (SW-REQ-039) re-uses this same ensure helper
  via embedding; the obligation transitively applies there.
- No isolated test for `ensureIndexes` — a future unit test using a
  testcontainer would let the idempotency obligation be exercised
  without the full pipeline.
