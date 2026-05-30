# SYS-REQ-029: MongoDB `$inc` is atomic on a single document (environmental assumption)

## Intent
The MongoDB Go driver, in combination with the MongoDB server, guarantees atomic application of the `$inc` update operator on a single document under concurrent writers. This is an environmental assumption that satisfies parent **STK-REQ-001** by surfacing the aggregation-counter-integrity guarantee the mongo-aggregate pump relies on but cannot itself enforce.

## Motivation
The mongo-aggregate pump (SW-REQ-018 family) uses `$inc` operators to merge per-bucket counters across the per-pump goroutines (one goroutine per backend, possibly many records per cycle), and across pump restarts. If two `$inc` updates to the same document raced, counter values would be lost — silently. Tyk-pump cannot enforce atomicity at the application layer; it is a property of the MongoDB server's single-document-write contract. Surfacing the assumption as a SYS req makes the trust boundary explicit so that any future schema change splitting a document or moving counters across documents (which would require multi-document transactions) is recognised as breaking the assumption.

## Formalization
```
pumps_mongo_aggregate shall always satisfy inc_is_atomic
```
This is an environmental input invariant: across every MongoDB server tyk-pump is configured against, the `$inc` update operator applied to a single document must be atomic. There is no trigger — the assumption holds whenever the mongo-aggregate pump writes. Truth condition is owned by the MongoDB server contract (single-document write atomicity) plus the Go driver's faithful relay of that operator.

## Code references
- `analytics/aggregate.go:306-331 generateBSONFromProperty` — emits the `$inc` operators that accumulate hits, errors, latencies, request times.
- `analytics/aggregate.go:462 "$inc": model.DBM{}` — the canonical update document shape the aggregate pump issues.
- `pumps/mcp_mongo_aggregate.go:125-146 addMCPDimensionUpdates` — additional `$inc` use for the MCP variant.
- `pumps/mcp_mongo_aggregate.go:170` comment "first applying counters ($inc/$set/$max), then recalculating averages and lists" — the application-layer flow that assumes server-side atomicity.
- `go.mod:42 go.mongodb.org/mongo-driver v1.13.1` — pinned driver version the assumption is reviewed against.

## Evidence
- External-owner review: `assumption.external_owner: team:mongodb`, status `open`, reviewed via `proof req assumptions review` in Phase B (`verification.review.comment`: "external-owner reviewed (Redis protocol / MongoDB / Tyk gateway / MaxMind)"). Next review date: `2026-11-30`.
- MongoDB server documentation states single-document write operations are atomic; `$inc` is in scope of that guarantee.
- `pumps/mcp_mongo_aggregate_test.go:228` exercises the `$inc` doc shape — relies on the assumption being true to be a meaningful test.

## Open questions
- The assumption is strictly about *single-document* atomicity. Any future change that spread counters across multiple documents (e.g. one document per dimension key) would require multi-document transactions to preserve the same guarantee — not currently flagged.
- No KI linkage today; MongoDB's atomicity contract is considered stable upstream. If a future driver release changed the operator's behaviour (e.g. silently retrying causing double-increment under a network partition), the assumption review at `2026-11-30` would catch it.
- DocumentDB / Cosmos DB / other Mongo-wire-protocol stores inherit this assumption via wire-protocol compatibility, but their `$inc` semantics under contention have historically diverged — operators on those backends should re-verify.
