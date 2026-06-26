# SW-REQ-099: MongoAggregate DocumentDB index compatibility

## Intent
When `MongoAggregatePump` is configured for AWS DocumentDB and index creation is
enabled, `ensureIndexes` shall attempt the aggregate collection's baseline
indexes without using the StandardMongo collection-exists shortcut. Indexes shall
be foreground indexes (`Background=false`) for DocumentDB.

## Motivation
TT-5302 fixed a DocumentDB compatibility bug. The historical implementation used
the collection-listing shortcut for all Mongo-compatible backends. On DocumentDB
that probe could leak cursors, and an existing aggregate collection could
incorrectly skip Tyk's baseline indexes.

## Formalization
```
when docdb_backend_configured & omit_index_creation_disabled pumps_mongo_aggregate shall always satisfy docdb_indexes_attempted
```

Variables are declared in `specs/software/variables/pumps-mongo-aggregate.vars.yaml`.

## Code References
- `pumps/mongo_aggregate.go:MongoAggregatePump.ensureIndexes` checks
  `OmitIndexCreation`, then uses `collectionExists` only when
  `MongoDBType == StandardMongo`.
- The DocumentDB branch attempts `expireAt` TTL, `timestamp`, and `orgid` with
  `Background` false.
- `expireAt` supports aggregate expiry cleanup. `timestamp` supports aggregate
  time-window lookups and average recalculation queries. `orgid` supports
  per-organisation aggregate collection access paths.

## Evidence
- `pumps/mongo_mcdc_100_test.go:TestMongoAggregatePump_EnsureIndexes_DocumentDBDoesNotUseExistsShortcut`
  uses a recording fake store to prove DocumentDB does not call `HasTable`, does
  attempt the exact `expireAt` TTL (`TTL=0`), `timestamp`, and `orgid`
  definitions with `Background=false`, non-TTL metadata for non-expiry indexes,
  and still honors `OmitIndexCreation`.
- This is fake-store/index-model evidence. It proves Pump sends the intended
  index definitions into the persistence layer; it is not a live AWS DocumentDB
  catalog acceptance test.
