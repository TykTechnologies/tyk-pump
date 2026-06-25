# SW-REQ-037: Graph MongoDB pump — GraphQL record forwarding

## Intent
The `GraphMongoPump` shall, on each purge, accumulate incoming records into
size-bounded batches via the embedded `MongoPump.AccumulateSet` (in
graph-only mode, records whose `GraphQLStats.IsGraphQL` is false are skipped),
preserve graph record `RawRequest` and `RawResponse` payloads even when their
size exceeds `MaxDocumentSizeBytes`, convert each surviving `AnalyticsRecord`
to a `GraphRecord` (assigning a fresh `bson.NewObjectID`), and concurrently
insert each batch into the single configured graph collection. The first
per-batch insert error shall be returned to the caller, and connection-closed
conditions shall be logged as a 'Detected connection failure!' warning. Derived
from SYS-REQ-004 via Phase A decomposition of SW-REQ-018.

## Motivation
GraphQL analytics carry a distinct shape (operation name, root field counts,
errors) that does not fit cleanly inside the standard analytics record. This
pump exists to keep GraphQL records out of the main analytics collection so
the Tyk Dashboard's standard analytics queries do not have to skip-filter
graph records. The pump re-uses the embedded `MongoPump.AccumulateSet` for
batching, but graph-mode records are intentionally exempt from the standard
Mongo raw-body rewrite because the graph payload belongs to the Graph Mongo
record.

## Code references
- `pumps/graph_mongo.go:GraphMongoPump.Init` — wires the embedded `MongoPump`.
- `pumps/graph_mongo.go:WriteData` — calls `m.AccumulateSet(data, true)`
  (the `true` flag enables graph-record mode in the shared helper).
- `pumps/mongo.go:464` — graph-mode `AccumulateSet` filters non-GraphQLStats
  records and passes `isForGraphRecords=true` into large-document handling.
- `pumps/mongo.go:526` — oversized raw-body rewrite is disabled when
  `isGraphRecord` is true.
- `pumps/graph_mongo.go` connection-failure path — logs 'Detected connection
  failure!' on `closed explicitly` / `was closed` errors.

## Evidence
- `pumps/graph_mongo_test.go` (re-annotated `Verifies: SW-REQ-037`).
- `pumps/mongo_test.go:TestMongoPump_AccumulateSetIgnoreDocSize` proves
  GraphQLStats-backed graph records retain `RawRequest` and `RawResponse`
  through graph-mode accumulation even when they exceed `MaxDocumentSizeBytes`.
- Live-MongoDB tests are excluded from the local audit MC/DC scope (known
  issue).

## Open questions
- Same `context.Background()` issue as the standard pump (tracked under
  `mongo-pump-ignores-caller-context`).
