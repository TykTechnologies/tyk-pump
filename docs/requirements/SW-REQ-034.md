# SW-REQ-034: MongoDB standard pump — batched inserts with capped collection support

## Intent
The standard MongoDB pump (`MongoPump`) shall, on each purge, filter out
MCP-classified analytics records, accumulate the remaining records into
size-bounded batches (`MaxInsertBatchSizeBytes`, default 10 MiB), estimate each
standard analytics document as `len(RawRequest)+len(RawResponse)+1024` metadata
bytes, rewrite the raw bodies of documents larger than `MaxDocumentSizeBytes`
rather than skipping the whole record, and concurrently
insert each batch into the single configured collection, returning the first
per-batch insert error to the caller. When `CollectionCapEnable` is true on a
64-bit host and the target collection does not yet exist, the pump shall
create it as a capped collection of `CollectionCapMaxSizeBytes` (default
5 GiB). StandardMongo index setup is idempotent for an already-existing
collection by skipping baseline index creation; the non-StandardMongo
same-key/different-name `logBrowserIndex` conflict remains tracked under
`mongo-standard-logbrowser-compatible-index-conflict`. A separate current
KnownIssue, `mongo-standard-final-skipped-record-drops-pending-batch`, tracks
the final-skipped-input flush gap and is not claimed fixed by this requirement
hardening. Derived from SYS-REQ-004 (independent per-backend delivery) via the
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
- `main.go:209` and `pumps/mongo.go:391` — configured pump timeout is applied
  before `Init` and passed to `persistent.ClientOpts.ConnectionTimeout`.
- `pumps/mongo.go:464` — `AccumulateSet`; the per-batch size bookkeeping.
- `pumps/mongo.go:519` — `getItemSizeBytes`; exact standard Mongo size
  estimate `len(RawRequest)+len(RawResponse)+1024`.
- `pumps/mongo.go:499` — `shouldProcessItem` (skips MCP records via
  `IsMCPRecord()`).
- `pumps/mongo.go:525` — `handleLargeDocuments` (truncates raw bodies beyond
  `MaxDocumentSizeBytes`).

## Evidence
- `pumps/mongo_test.go` (re-annotated `Verifies: SW-REQ-034`) covers
  `capCollection`, MCP-skip, large-document handling, and batching.
- `pumps/mongo_test.go:TestMongoPump_GetItemSizeBytes_CountsRawRequestAndResponseOnce`
  proves RawRequest and RawResponse are each counted once with the 1024-byte
  metadata allowance.
- `pumps/mongo_test.go:TestMongoPump_AccumulateSet_RewritesOversizePayloadWithoutSkipping`
  proves a document whose estimate is exactly at the threshold is preserved,
  while a document over the threshold is still retained with raw bodies
  rewritten.
- `pumps/mongo_timeout_config_test.go` proves timeout setup order and
  `ConnectionTimeout: m.timeout` propagation without starting MongoDB.
- `pumps/mongo_mcdc_100_test.go:TestMongoPump_EnsureIndexes_CollectionExistsShortCircuit`
  covers StandardMongo idempotent existing-collection index setup.
- `pumps/mongo_mcdc_100_test.go:TestMongoPump_EnsureIndexes_LogBrowserDifferentName_KI`
  is the current non-StandardMongo same-key/different-name conflict tripwire.
- `pumps/mgo_helper_test.go` provides the shared MongoDB test helper used by
  the family.
- Live-MongoDB tests are excluded from the local audit MC/DC scope (recorded
  as a known issue).

## Related requirements
`SW-REQ-097` decomposes the TT-5302 DocumentDB index compatibility rule for
this standard Mongo index path: DocumentDB must not use the StandardMongo
collection-exists shortcut and must attempt foreground baseline indexes unless
`omit_index_creation` is set.

## Open questions
- `WriteData` and `WriteUptimeData` ignore the caller `ctx` and pass
  `context.Background()` to `m.store.Insert` (see lines 435 and 589). This is
  tracked by the existing known-issue `mongo-pump-ignores-caller-context`
  (extended to cover all four mongo writers in Phase A).
- Only the *first* non-nil per-batch error is returned; multi-error behaviour
  is currently lossy.
- `capCollection` requires a 64-bit host; behaviour on a 32-bit build is
  silent no-op.
- Non-StandardMongo `logBrowserIndex` same-key/different-name conflicts are
  returned from `ensureIndexes` and logged by `Init`; tracked by
  `mongo-standard-logbrowser-compatible-index-conflict`.
- If the last input item is skipped after at least one batch was already
  flushed, `AccumulateSet` can drop the pending valid batch; tracked by
  `mongo-standard-final-skipped-record-drops-pending-batch`.
