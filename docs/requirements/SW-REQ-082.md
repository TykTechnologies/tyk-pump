# SW-REQ-082: Standard Mongo inserts non-MCP records

Documents: SW-REQ-082

The standard Mongo pump writes analytics records into the configured standard
collection. Graph-only filtering belongs to graph-specific pumps; the standard
Mongo pump must not drop a record merely because `GraphQLStats.IsGraphQL` is
true or because the record carries the legacy `tyk-graph-analytics` tag.

## Contract

When `MongoPump.WriteData` receives valid non-MCP `AnalyticsRecord` values, it
shall insert all of them into the configured standard Mongo collection. This
includes ordinary analytics records, GraphQL-classified records, and legacy
tag-only graph records. MCP records remain excluded from the standard collection
under SW-REQ-034.

## Evidence

- `pumps/mongo_test.go:TestMongoPump_WriteData` writes ordinary records and a
  mixed ordinary/GraphQL-classified batch through the real Mongo storage path,
  then queries the configured collection and asserts every input record was
  inserted.
- `pumps/mongo_test.go:TestMongoPump_AccumulateSet` proves standard-mode
  batching retains both `GraphQLStats.IsGraphQL` records and records carrying
  only the legacy graph analytics tag.
- `pumps/mongo_test.go:TestMongoPump_WriteData_MCPOnlyReturnsBeforeStorage`
  covers the no-insertable-record vacuous case for all-MCP input.

## Known Issues

Mongo standard still carries separate KI-backed debt for caller context
propagation, timeout bounding, config strictness, and process-exit behavior.
Those are not fixed by this requirement.
