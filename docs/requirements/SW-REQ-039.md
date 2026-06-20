# SW-REQ-039: MCP MongoDB aggregate pump — per-API two-step upsert

## Intent
The `MCPMongoAggregatePump` shall aggregate MCP-classified analytics records
per API via `analytics.AggregateMCPData`, then for each API perform a
two-step MongoDB upsert keyed by `{orgid, timestamp, owner_apiid}`: step
one applies `$inc`/`$set`/`$max` counters for both standard dimensions
(via `AnalyticsRecordAggregate.AsChange`) and MCP-specific dimensions
(`methods`, `primitives`, `names` via `addMCPDimensionUpdates`); step two
recalculates derived averages via `AsTimeUpdate`. The first upsert error per
API shall be returned to the caller. When `UseMixedCollection` is true, both
the per-org and the shared mixed-collection variant shall be written.
Derived from SYS-REQ-003 via Phase A decomposition of SW-REQ-018.

## Motivation
MCP analytics carry additional dimensions (the method invoked, the
primitive type and name) that the standard aggregate pipeline does not
project. This pump exists to preserve those dimensions in the aggregate
documents that drive the MCP analytics dashboards. The two-step pattern
mirrors the standard mongo-aggregate path (SW-REQ-060) but adds the
MCP-dimension update layer; the per-API partitioning (instead of per-org)
prevents cross-API merges in the mixed collection (the
[TT-17004 regression](https://github.com/TykTechnologies/tyk-pump/pull/989)
fix that produced the current implementation).

## Code references
- `pumps/mcp_mongo_aggregate.go:MCPMongoAggregatePump.WriteData` —
  orchestrator.
- `pumps/mcp_mongo_aggregate.go:DoMCPAggregatedWriting` — per-API upsert
  loop with `UseMixedCollection` plumbing.
- `pumps/mcp_mongo_aggregate.go:upsertMCPAggregate` — the two-step upsert.
- `pumps/mcp_mongo_aggregate.go:addMCPDimensionUpdates` — MCP-dimension
  `$inc`/`$max` builders.

## Evidence
- `pumps/mcp_mongo_aggregate_test.go` (re-annotated `Verifies: SW-REQ-039`):
  - `TestMCPMongoAggregatePump_WriteData_PerAPIPartitioning` — the
    TT-17004 regression test.
  - `TestMCPMongoAggregatePump_WriteData_Roundtrip` / `_MixedCollection` —
    end-to-end upsert verification.
  - `TestAddMCPDimensionUpdates_*` — exercise the dimension-update helper.
- The `atomicity` obligation's `negative` evidence requirement is deferred:
  MCP-mongo upsert atomicity negative evidence requires a fault-injecting
  MongoDB harness not available locally.
- Live-MongoDB tests are excluded from the local audit MC/DC scope (known
  issue).

## Open questions
- Same `context.Background()` issue as the standard mongo pump applies to
  the embedded write paths.
- Does *not* implement self-healing — only the non-MCP `MongoAggregatePump`
  has `ShouldSelfHeal`. Operators experiencing document-size errors on the
  MCP aggregate collection have no recourse short of dropping the
  collection and adjusting `AggregationTime` manually.
- Re-uses `MongoAggregatePump.ensureIndexes`, so the same index-ensure
  behaviour (SW-REQ-063) applies.
