# SW-REQ-097: Standard Mongo DocumentDB index compatibility

## Intent
When `MongoPump` is configured for AWS DocumentDB and index creation is enabled,
`ensureIndexes` shall attempt the standard collection's baseline indexes without
using the StandardMongo collection-exists shortcut. Indexes shall be foreground
indexes (`Background=false`) for DocumentDB.

## Motivation
TT-5302 fixed a DocumentDB compatibility bug. The historical implementation used
the collection-listing shortcut for all Mongo-compatible backends. On DocumentDB
that probe could leak cursors, and an existing collection could incorrectly skip
Tyk's baseline indexes.

## Formalization
```
when docdb_backend_configured & omit_index_creation_disabled pumps_mongo_standard shall always satisfy docdb_indexes_attempted
```

Variables are declared in `specs/software/variables/pumps-mongo-standard.vars.yaml`.

## Code References
- `pumps/mongo.go:MongoPump.ensureIndexes` checks `OmitIndexCreation`, then
  uses `collectionExists` only when `MongoDBType == StandardMongo`.
- The DocumentDB branch attempts `orgid`, `apiid`, and `logBrowserIndex` with
  `Background` false.

## Evidence
- `pumps/mongo_mcdc_100_test.go:TestMongoPump_EnsureIndexes_DocumentDBDoesNotUseExistsShortcut`
  uses a recording fake store to prove DocumentDB does not call `HasTable`, does
  attempt the expected indexes, and still honors `OmitIndexCreation`.
