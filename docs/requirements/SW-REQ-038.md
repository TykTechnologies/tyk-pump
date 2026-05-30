# SW-REQ-038: MCP MongoDB pump — model-context-protocol record forwarding

## Intent
The `MCPMongoPump` shall, on each purge, retain only records for which
`AnalyticsRecord.IsMCPRecord()` is true, accumulate them into size-bounded
batches via the embedded `MongoPump.AccumulateSet`, convert each batch to
`MCPRecord` instances with fresh `bson.NewObjectID` values, and concurrently
insert each batch into the single configured MCP collection. The first
per-batch insert error shall be returned to the caller, and `closed
explicitly` errors shall be logged as a connection-failure warning. If the
collection name is unset at write time, `WriteData` returns
`fmt.Errorf("no collection name")` rather than calling `log.Fatal` as the
standard pump does. Derived from SYS-REQ-004 via Phase A decomposition of
SW-REQ-018.

## Motivation
MCP (Model Context Protocol) records describe tool/primitive invocations
issued by AI clients through Tyk Gateway, carrying fields the standard
analytics record does not (JSONRPC method, primitive type/name). They are
stored in a dedicated collection so MCP-specific dashboards can query without
filtering the entire analytics stream. Sharing the embedded
`MongoPump.AccumulateSet` keeps the batching/sizing logic in one place; only
the per-record filter and the MCPRecord conversion are unique.

## Code references
- `pumps/mcp_mongo.go:MCPMongoPump.WriteData` — orchestrator.
- `pumps/mcp_mongo.go:filterMCPData` — applies `IsMCPRecord()`.
- `pumps/mcp_mongo.go:convertToMCPObjects` — `AnalyticsRecord` → `MCPRecord`.
- `pumps/mcp_mongo.go:insertMCPDataSet` — concurrent per-batch insert with
  shared `errCh`.

## Evidence
- `pumps/mcp_mongo_test.go` (re-annotated `Verifies: SW-REQ-038`).
- Live-MongoDB tests are excluded from the local audit MC/DC scope (known
  issue).

## Open questions
- Same `context.Background()` issue as the standard pump (tracked under
  `mongo-pump-ignores-caller-context`).
- The pump returns a normal error on missing collection rather than crashing
  the process; this is more defensive than `MongoPump.Init` but inconsistent
  with the rest of the family — worth a follow-up to harmonise.
