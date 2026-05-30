# SW-REQ-035: MongoDB selective pump — per-organisation collection routing

## Intent
The `MongoSelectivePump` shall, on each purge, partition incoming non-MCP
analytics records by `OrgID` and route each org's records into a
per-organisation collection named `z_tyk_analyticz_<orgid>`. Records with an
empty `OrgID` shall be dropped. For each org collection the pump shall ensure
baseline indexes (`apiid`, a TTL index on `expireAt` which is skipped for
CosmosDB, and a composite `logBrowserIndex`) and then insert records in
size-bounded batches up to `MaxInsertBatchSizeBytes` (default 10 MiB),
skipping individual documents larger than `MaxDocumentSizeBytes`. Derived from
SYS-REQ-004 via Phase A decomposition of SW-REQ-018.

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
  `:AccumulateSet` — size-bounded batching.

## Evidence
- `pumps/mongo_selective_test.go` (re-annotated `Verifies: SW-REQ-035`).
- Live-MongoDB tests are excluded from the local audit MC/DC scope (recorded
  as a known issue).

## Open questions
- Per-batch insert errors are swallowed (`mongo_selective.go:242-244`); the
  function always returns `nil`. This means upstream retry / backpressure
  cannot react to per-org failures. Honest obligation_class is `nominal`
  rather than `errors_propagated`.
- Same `context.Background()` issue as the standard pump (tracked under
  `mongo-pump-ignores-caller-context`).
- Per-org collection growth is unbounded; no capped-collection support like
  the standard pump.
