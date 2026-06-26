# SW-REQ-098: MongoSelective DocumentDB index compatibility

## Intent
When `MongoSelectivePump` is configured for AWS DocumentDB and index creation is
enabled, `ensureIndexes` shall attempt the selective collection's baseline
indexes without using the StandardMongo collection-exists shortcut. Indexes shall
be foreground indexes (`Background=false`) for DocumentDB.

## Motivation
TT-5302 fixed a DocumentDB compatibility bug. The historical implementation used
the collection-listing shortcut for all Mongo-compatible backends. On DocumentDB
that probe could leak cursors, and an existing per-org collection could
incorrectly skip Tyk's baseline indexes.

## Formalization
```
when docdb_backend_configured & omit_index_creation_disabled pumps_mongo_selective shall always satisfy docdb_indexes_attempted
```

Variables are declared in `specs/software/variables/pumps-mongo-selective.vars.yaml`.

## Code References
- `pumps/mongo_selective.go:MongoSelectivePump.ensureIndexes` checks
  `OmitIndexCreation`, then uses `collectionExists` only when
  `MongoDBType == StandardMongo`.
- The DocumentDB branch attempts `apiid`, `expireAt` TTL, and
  `logBrowserIndex` with `Background` false.
- `apiid` supports per-API selective analytics lookup. `expireAt` supports
  expiry cleanup for selective records. The `logBrowserIndex` key order starts
  with descending `timestamp`, then `apiid`, `apikey`, and `responsecode`,
  matching selective log-browser time-window and dimension filter access paths.

## Evidence
- `pumps/mongo_mcdc_100_test.go:TestMongoSelectivePump_EnsureIndexes_DocumentDBDoesNotUseExistsShortcut`
  uses a recording fake store to prove DocumentDB does not call `HasTable`, does
  attempt the exact `apiid`, `expireAt` TTL (`TTL=0`), and `logBrowserIndex`
  definitions in the expected key order with `Background=false`, non-TTL
  metadata for non-expiry indexes, and still honors `OmitIndexCreation`.
- This is fake-store/index-model evidence. It proves Pump sends the intended
  index definitions into the persistence layer; it is not a live AWS DocumentDB
  catalog acceptance test.
