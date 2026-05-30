# SW-REQ-012: MCP aggregation by JSON-RPC method and primitive type

## Intent
Realises the MCP-specific aggregation grouping of parent **SYS-REQ-003**. `AggregateMCPData` walks a batch of `interface{}` records, filters them down to those for which `record.IsMCPRecord()` is true (i.e. `MCPStats.IsMCP == true`), groups them by `record.APIID`, and inside each per-API aggregate adds counters to three MCP-specific dimension maps via `incrementMCPDimensions`: `Methods` keyed by `JSONRPCMethod`, `Primitives` keyed by `PrimitiveType`, and `Names` keyed by a composite `PrimitiveType_PrimitiveName` label.

## Motivation
MCP traffic is operationally distinct from REST/GraphQL — the meaningful "API surface" is the JSON-RPC method (`tools/call`, `resources/read`, …) and the named primitive being invoked. Grouping by these dimensions instead of HTTP path is what lets operators answer "how many `tools/call` for tool X have we seen this hour" without post-processing. Restricting aggregation to MCP-flagged records (`IsMCPRecord` check) means MCP and non-MCP traffic don't pollute each other's aggregate buckets — the same APIID can be both, and the two paths produce two distinct documents (`MCPRecordAggregate` and the base `AnalyticsRecordAggregate`). The trade-off: per-API partitioning (`OwnerAPIID`) is required to prevent cross-API merge under MongoDB upsert (commit b18d8ea fixed exactly this bug for MCP aggregates).

## Code references
- `analytics/aggregate_mcp.go:159 AggregateMCPData` — top-level: type assert to `AnalyticsRecord`, skip via `!record.IsMCPRecord()` (line 164), call `ToMCPRecord`, init aggregate via `initMCPAggregateForRecord`, fold base via `incrementAggregate`, then `incrementMCPDimensions`.
- `analytics/aggregate_mcp.go:117 incrementMCPDimensions` — three `if non-empty` branches over `JSONRPCMethod`, `PrimitiveType`, `PrimitiveName`; each calls `incrementOrSetUnit` (see SW-REQ-011) and stamps `Identifier` + `HumanIdentifier`.
- `analytics/aggregate_mcp.go:24 MCPRecordAggregate` — embeds `AnalyticsRecordAggregate`; adds `OwnerAPIID`, `Methods`, `Primitives`, `Names` maps.
- `analytics/aggregate_mcp.go:56 TableName` — `tyk_mcp_analytics_aggregate` (mixed) or `z_tyk_mcp_analyticz_aggregate_<OrgID>` (per-org).
- `analytics/aggregate_mcp.go:147 AsTimeUpdate` — extends base `AsTimeUpdate` with `lists.methods` / `lists.primitives` / `lists.names`.
- `analytics/aggregate_mcp.go:98 initMCPAggregateForRecord` — seeds the per-API aggregate with org/time/expire fields.

## Evidence
- `analytics/aggregate_mcp_test.go:16 TestAggregateMCPData_SkipsNonMCPRecords` — tagged `// Verifies: SW-REQ-012`; the IsMCPRecord gate.
- `analytics/aggregate_mcp_test.go:56 TestAggregateMCPData_AggregatesByMethod`, `:87 TestAggregateMCPData_AggregatesByPrimitiveType`, `:111 TestAggregateMCPData_AggregatesByPrimitiveName` — per-dimension grouping.
- `analytics/aggregate_mcp_test.go:462 TestAggregateMCPData_MultipleAPIs`, `:487 TestAggregateMCPData_ErrorTracking`, `:515 TestAggregateMCPData_EmptyMethodSkipsDimension`, `:534 TestInitMCPAggregateForRecord_SetsTimeID`, `:552 TestAggregateMCPData_EmptyInput`, `:558 TestAggregateMCPData_NilInput` — boundary coverage.
- `analytics/aggregate_mcp_test.go:176 TestMCPAsTimeUpdate_ProducesListsAPIID`, `:253 TestMCPAsTimeUpdate_ProducesErrorListForNames`, `:316 TestMCPUpsertReadback_EmptyDocProducesEmptyLists`, `:346 TestMCPBSONRoundTrip_APIIDMapSurvivesReadback`, `:426 TestAggregateData_SkipsMCPRecords` — Mongo-projection and BSON round-trip coverage.

## Open questions
- `Names` is keyed by `fmt.Sprintf("%s_%s", PrimitiveType, PrimitiveName)` when both are set, but only `PrimitiveName` when `PrimitiveType == ""`. The composite-key behaviour is correct but undocumented in the req text and bites BSON-readback consumers that expect a plain primitive name.
- The req text says "grouping by JSON-RPC method *and* primitive type" — actual implementation is "grouping by APIID, then within each per-API aggregate, separate counter maps for method, primitive type, and primitive name". The cardinality is per-API, not the cross-product. Worth pinning in any future refinement.
