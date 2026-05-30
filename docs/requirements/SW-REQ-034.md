# SW-REQ-034: MongoDB standard pump — batched inserts with capped collection support

## Intent
The standard MongoDB pump (`MongoPump`) shall, on each purge, filter out
MCP-classified analytics records, accumulate the remaining records into
size-bounded batches (`MaxInsertBatchSizeBytes`, default 10 MiB), rewrite the
raw bodies of documents larger than `MaxDocumentSizeBytes`, and concurrently
insert each batch into the single configured collection, returning the first
per-batch insert error to the caller. When `CollectionCapEnable` is true on a
64-bit host and the target collection does not yet exist, the pump shall
create it as a capped collection of `CollectionCapMaxSizeBytes` (default
5 GiB). Derived from SYS-REQ-004 (independent per-backend delivery) via the
Phase A decomposition of the previous family req SW-REQ-018.

## Motivation
Phase 0 grouped six MongoDB pumps under one family req. The standard pump is
the historical default backend for Tyk Pump and the recommended target for the
Tyk Dashboard — it is the only variant that batches inserts to respect
Mongo's 16 MiB per-document limit while also supporting capped-collection
provisioning for fixed-footprint deployments. Splitting it out clarifies that
the capped-collection logic, the size-bounded batching, and the MCP-record
skip are unique to this variant (the selective pump shards by org; the
aggregate pump bucketises; the graph/MCP variants filter different record
classes).

## Code references
- `pumps/mongo.go:177` — `MongoPump.New`; `Init` at line 207 normalises the
  config and resolves defaults for `MaxInsertBatchSizeBytes` /
  `MaxDocumentSizeBytes`.
- `pumps/mongo.go:274` — `capCollection` (gated on a 64-bit host check; the
  upstream Mongo driver requires int64 sizes).
- `pumps/mongo.go:332` — `ensureIndexes`.
- `pumps/mongo.go:402` — `WriteData`; per-batch goroutines fan-out through an
  `errCh`, returning the first non-nil error.
- `pumps/mongo.go:464` — `AccumulateSet`; the per-batch size bookkeeping.
- `pumps/mongo.go:499` — `shouldProcessItem` (skips MCP records via
  `IsMCPRecord()`).
- `pumps/mongo.go:525` — `handleLargeDocuments` (truncates raw bodies beyond
  `MaxDocumentSizeBytes`).

## Evidence
- `pumps/mongo_test.go` (re-annotated `Verifies: SW-REQ-034`) covers
  `capCollection`, MCP-skip, large-document handling, and batching.
- `pumps/mgo_helper_test.go` provides the shared MongoDB test helper used by
  the family.
- Live-MongoDB tests are excluded from the local audit MC/DC scope (recorded
  as a known issue).

## Open questions
- `WriteData` and `WriteUptimeData` ignore the caller `ctx` and pass
  `context.Background()` to `m.store.Insert` (see lines 435 and 589). This is
  tracked by the existing known-issue `mongo-pump-ignores-caller-context`
  (extended to cover all four mongo writers in Phase A).
- Only the *first* non-nil per-batch error is returned; multi-error behaviour
  is currently lossy.
- `capCollection` requires a 64-bit host; behaviour on a 32-bit build is
  silent no-op.
