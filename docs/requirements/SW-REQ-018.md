# SW-REQ-018: MongoDB pump family — write records/aggregates with error propagation

## Intent
The MongoDB pump family shall write analytics records (and, for the aggregate
variants, pre-aggregated documents) to MongoDB collections, returning
connection and per-insert errors to the caller so the core can log, time out,
or retry as appropriate. The family covers six distinct backends sharing only
the storage technology: standard, selective per-org, time-bucket aggregate,
GraphQL-specific, MCP-specific, and MCP-aggregate. Derived from SYS-REQ-004
(independent per-backend delivery).

## Motivation
MongoDB is the historical default backend for Tyk Pump and remains the
recommended target for the Tyk Dashboard. The family has grown organically:
the standard pump batches inserts using `AccumulateSet` to respect Mongo's
16MB document limit; the aggregate pump bucketises by org/time and includes
self-healing logic (`ShouldSelfHeal` halves `AggregationTime` after document-size
errors and retries — see `mongo_aggregate.go:325-332`); the selective variant
splits records into per-org collections; graph and MCP variants exist purely to
keep specialised record types out of the main analytics stream.

Lumping all six under one requirement was a pragmatic Phase 0 choice, but it
hides genuinely different behaviour: only the aggregate pump retries; only the
selective pump partitions by org; the graph/MCP variants skip non-matching
records. The single-error-propagation guarantee remains true for all six, but
that's a relatively thin contract.

## Code references
- `pumps/mongo.go:32-37` — `MongoPump` definition; `WriteData` at line 402
  uses `AccumulateSet` (line 464) to batch inserts respecting Mongo's
  per-document size limit (`handleLargeDocuments`, line 483).
- `pumps/mongo_selective.go:203` — `MongoSelectivePump.WriteData` shards
  inserts into per-org collections.
- `pumps/mongo_aggregate.go:299-342` — `MongoAggregatePump.WriteData` bucketises
  via `analytics.AggregateData`, calls `DoAggregatedWriting` (upsert), and
  retries with halved `AggregationTime` on document-size errors when
  `EnableAggregateSelfHealing` is set.
- `pumps/graph_mongo.go:101` — `GraphMongoPump.WriteData` filters to graph
  records only.
- `pumps/mcp_mongo.go:159`, `pumps/mcp_mongo_aggregate.go` — MCP record variants
  (raw and aggregated). Standard pumps skip MCP records via `IsMCPRecord()`
  (`mongo.go:411`, `mongo_aggregate.go:302`).

## Evidence
- `pumps/mongo_test.go`, `pumps/mongo_selective_test.go`,
  `pumps/mongo_aggregate_test.go`, `pumps/graph_mongo_test.go`,
  `pumps/mcp_mongo_test.go`, `pumps/mcp_mongo_aggregate_test.go` —
  per-variant unit tests, but most require a running MongoDB and are
  excluded from the local audit MC/DC scope (recorded as a known issue).

## Open questions
- Per-file logic varies dramatically (standard batching vs selective per-org
  vs aggregate self-healing vs graph/MCP filtering). This is exactly the gap
  that Phase A (per-implementation decomposition) should address — each
  variant deserves its own SW-REQ with its own guarantees (e.g. "aggregate
  pump shall retry on doc-size error", "selective pump shall route by orgID").
- The standard pump's `WriteData` does not return until all goroutines drain
  the `errCh`, but only the *first* non-nil error is returned — the requirement
  text says "propagating", which is technically satisfied but understates
  multi-error behaviour.
- The aggregate self-healing recursion has no recursion-depth bound;
  pathological inputs could loop until `AggregationTime` hits 1.
- MCP-record skipping is implicit per-pump (`IsMCPRecord()` guards in
  `mongo.go`/`mongo_aggregate.go`) and worth its own requirement in Phase A.
