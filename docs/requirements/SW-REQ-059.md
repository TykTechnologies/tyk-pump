# SW-REQ-059: Mongo aggregate — per-org collection sharding and mixed collection

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-036 (mongo-aggregate). It carries the per-organisation collection
naming and the `UseMixedCollection` mirror behaviour in isolation.

## Intent
The MongoAggregate pump shall route each per-organisation aggregate
document into a collection named `z_tyk_analyticz_aggregate_<orgid>` via
`GetCollectionName`. When `UseMixedCollection` is true, each per-org
aggregate shall additionally be written to the shared mixed collection
`AgggregateMixedCollectionName` (`Mixed=true` variant). Empty `orgid` shall
return an error from `GetCollectionName`. Derived from SYS-REQ-003.
SW-REQ-102 decomposes the persisted `Lists` projection needed by aggregate
listing queries.

## Motivation
Per-organisation collection sharding gives multi-tenant Tyk Dashboard
deployments per-tenant data isolation in the aggregate path (mirror of
SW-REQ-035 for the non-aggregate path). The mixed-collection mirror lets
operators run cross-org dashboards without having to fan-out queries.
Splitting these concerns out of the parent makes the per-org naming
contract auditable.

## Code references
- `pumps/mongo_aggregate.go:MongoAggregatePump.GetCollectionName` —
  per-org name derivation.
- `pumps/mongo_aggregate.go:MongoAggregatePump.WriteData` —
  `writingAttempts` loop that iterates per-org aggregates.
- `pumps/mongo_aggregate.go:DoAggregatedWriting` — per-doc upsert.
- `docs/requirements/SW-REQ-102.md` — child requirement for `Lists` projection
  persistence in per-org and mixed aggregate readback.

## Evidence
- `pumps/mongo_aggregate_test.go:TestDoAggregatedWritingWithIgnoredAggregations`
  (re-annotated `Verifies: SW-REQ-059`) — exercises both the per-org and
  the mixed-collection writes with `use_mixed_collection: true`, and provides
  SW-REQ-102 evidence that `Lists.APIKeys` is preserved on readback.

## Related requirements
`SW-REQ-096` covers the data-retention side of the same mixed-collection path:
a writer with `ignore_aggregations` configured must not delete dimension maps
that a previous non-ignoring mixed write already persisted.

## Open questions
- `AgggregateMixedCollectionName` is a typo (three `g`s) that has shipped
  for years; renaming would break operator-deployed indexes / backups.
  Treated as cosmetic.
