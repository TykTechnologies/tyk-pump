# SW-REQ-035: MongoDB selective pump — per-organisation collection routing

## Intent
The `MongoSelectivePump` shall, on each purge, partition incoming non-MCP HTTP
analytics records by `OrgID` and route each org's records into a
per-organisation collection named `z_tyk_analyticz_<orgid>`. Records with an
empty `OrgID` shall be dropped, and TCP/error records represented by
`ResponseCode == -1` shall be excluded from selective Mongo writes. For each
org collection the pump shall ensure
baseline indexes (`apiid`, a TTL index on `expireAt` which is skipped for
CosmosDB, and a composite `logBrowserIndex`) and then insert records in
size-bounded batches up to `MaxInsertBatchSizeBytes` (default 10 MiB),
skipping individual documents larger than `MaxDocumentSizeBytes`. If an
equivalent `logBrowserIndex` already exists under a different name, the
selective pump treats that backend conflict as idempotent success while still
returning unrelated index creation errors. Derived from SYS-REQ-004 via Phase
A decomposition of SW-REQ-018.

The exact `MaxDocumentSizeBytes` arithmetic is split into **SW-REQ-092**:
`len(RawRequest)+len(RawResponse)+1024`, with each raw payload counted once.

## Motivation
This pump exists to give multi-tenant Tyk Dashboard deployments per-tenant
data isolation at the storage layer: each customer organisation gets its own
Mongo collection, simplifying retention policy, GDPR data-removal requests,
and per-tenant index tuning. Lumping this pump under the family-level
SW-REQ-018 hid the fundamentally different routing model (per-org sharding
vs the standard pump's single-collection model). Critically — and unlike the
family-level claim — this pump's `WriteData` *does not* propagate per-batch
insert errors: errors are logged and discarded (see `mongo_selective.go:244`).
The honest req description above reflects this gap.

## Code references
- `pumps/mongo_selective.go:203` — `MongoSelectivePump.WriteData` partitions
  by `OrgID` into the `analyticsPerOrg` map.
- `pumps/mongo_selective.go:GetCollectionName` — returns
  `z_tyk_analyticz_<orgid>` or an error for empty `OrgID`.
- `pumps/mongo_selective.go:ensureIndexes` — per-org index ensure on
  `apiid`, the TTL `expireAt`, and the composite `logBrowserIndex`.
- `pumps/mongo_selective.go:processItem`, `:accumulate`,
  `:AccumulateSet` — selective HTTP-record filtering and size-bounded batching.
- `pumps/mongo_selective.go:getItemSizeBytes` — exact document-size estimate
  for SW-REQ-092.
- `main.go:209` and `pumps/mongo_selective.go:134` — configured pump timeout
  is applied before `Init` and passed to
  `persistent.ClientOpts.ConnectionTimeout`.

## Evidence
- `pumps/mongo_selective_test.go` (re-annotated `Verifies: SW-REQ-035`).
- `pumps/mongo_selective_test.go:TestMongoSelectivePump_AccumulateSet_DropsTCPErrorRecords`
  proves the historical TCP/error exclusion through the batching path.
- `pumps/mongo_selective_test.go:TestMongoSelectivePump_GetItemSizeBytes_CountsRawRequestAndResponseOnce`
  covers SW-REQ-092 exact document-size arithmetic.
- `pumps/mongo_timeout_config_test.go` proves timeout setup order and
  `ConnectionTimeout: m.timeout` propagation without starting MongoDB.
- `pumps/mongo_mcdc_100_test.go:TestMongoSelectivePump_EnsureIndexes_LogBrowserDifferentName_FakeStore`
  proves same-key/different-name `logBrowserIndex` conflicts are idempotent.
- `pumps/mongo_mcdc_100_test.go:TestMongoSelectivePump_EnsureIndexes_LogBrowserDifferentName_NonSentinelErr`
  proves unrelated `logBrowserIndex` creation errors still propagate.
- Live-MongoDB tests are excluded from the local audit MC/DC scope (recorded
  as a known issue).

## Related requirements
`SW-REQ-098` decomposes the TT-5302 DocumentDB index compatibility rule for
this selective per-org index path: DocumentDB must not use the StandardMongo
collection-exists shortcut and must attempt foreground baseline indexes unless
`omit_index_creation` is set.

## Open questions
- Per-batch insert errors are swallowed (`mongo_selective.go:242-244`); the
  function always returns `nil`. This means upstream retry / backpressure
  cannot react to per-org failures. Honest obligation_class is `nominal`
  rather than `errors_propagated`.
- Same `context.Background()` issue as the standard pump (tracked under
  `mongo-pump-ignores-caller-context`).
- Per-org collection growth is unbounded; no capped-collection support like
  the standard pump.
- `AccumulateSet` loses a pending valid batch when the final input item is
  skipped for size or TCP/error classification; tracked under
  `mongo-selective-final-skipped-record-drops-pending-batch`.
