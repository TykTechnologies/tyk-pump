# SW-REQ-014: MCP record extraction

## Intent
Realises MCP-aware record shaping for parent **SYS-REQ-002**. `(*AnalyticsRecord).ToMCPRecord` projects an analytics record into an `MCPRecord` for persistence, promoting the three MCP identity fields (`JSONRPCMethod`, `PrimitiveType`, `PrimitiveName`) from the embedded `MCPStats` sub-struct to top-level columns while keeping the full `AnalyticsRecord` embedded (`gorm:"embedded;embeddedPrefix:analytics_"` for SQL, `bson:",inline"` for Mongo). When `IsMCPRecord()` returns false (`MCPStats.IsMCP == false`), it returns a zero-value `MCPRecord`.

## Motivation
Promotion to top-level columns is what makes per-method/per-primitive queries (`WHERE jsonrpc_method = 'tools/call'`) index-friendly without descending into the embedded analytics blob. Keeping the full `AnalyticsRecord` embedded means MCP queries still have access to every standard dimension (org, API, response code, latency) — the projection is additive, not replacement. Trade-off: the same three identity fields exist in two places per row (top-level and inside the embedded analytics document under `mcp_stats`), so any consumer mutating one without the other risks divergence; in practice consumers treat the embedded copy as the source of truth and the top-level copy as a query index.

## Code references
- `analytics/mcp_record.go:40 (*AnalyticsRecord).ToMCPRecord` — `IsMCPRecord` gate, struct literal hoisting the three MCPStats fields.
- `analytics/mcp_record.go:12 MCPRecord` — the projection struct.
- `analytics/mcp_record.go:22 TableName` — falls back to `AnalyticsRecord.TableName()` when `MCPSQLTableName` global is unset.
- `analytics/analytics.go:104 IsMCPRecord` — predicate on `MCPStats.IsMCP`.
- `analytics/aggregate_mcp.go:168 mcpRec := record.ToMCPRecord()` — the production caller inside `AggregateMCPData`.

## Evidence
- `analytics/mcp_record_test.go:14 TestAnalyticsRecord_IsMCPRecord` — tagged `// Verifies: SW-REQ-014`; the predicate.
- `analytics/mcp_record_test.go:36 TestAnalyticsRecord_MCPStatsJSONMarshal` — JSON wire-format of `MCPStats`.
- `analytics/mcp_record_test.go:64 TestAnalyticsRecord_ToMCPRecord` — populated-record round-trip plus the non-MCP zero-value branch.
- `analytics/mcp_record_test.go:122 TestMCPRecord_GetObjectID`, `:128 TestMCPRecord_SetObjectID` — the `model.Object` interface stubs.

## Open questions
- The `GetObjectID` / `SetObjectID` methods are empty stubs (returning `""` and dropping the input). For Mongo this means `MCPRecord` cannot participate in standard ObjectID-keyed dedup/upsert flows that other records use; the MCP pump handles dedup at the aggregate layer instead (`MCPRecordAggregate`). Worth noting because a future "just use the persistent-model interface" refactor would silently lose dedup if it touched these stubs.
- The `analytics_` embedded-prefix means the SQL schema has both top-level `jsonrpc_method` *and* `analytics_mcp_stats_*` columns (if the persistent model serialises `MCPStats` at all under that path). Schema migrations need to be aware of the duplication.
