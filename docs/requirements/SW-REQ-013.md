# SW-REQ-013: GraphQL record extraction

## Intent
Realises GraphQL-aware record shaping for parent **SYS-REQ-002**. `(*AnalyticsRecord).ToGraphRecord` projects an analytics record into a `GraphRecord` suitable for persistence in a SQL or Mongo store, copying the embedded `AnalyticsRecord` plus the GraphQL-specific fields from `GraphQLStats`: `OperationType` (mapped from the enum to a human-readable `"Query"`/`"Mutation"`/`"Subscription"`), `RootFields`, `Types`, `Errors`, `HasErrors`, `Variables`. If the record's `IsGraphRecord()` returns false (i.e. `GraphQLStats.IsGraphQL == false`), it returns a zero-value `GraphRecord` so the caller can decide what to do with non-GraphQL traffic.

## Motivation
GraphQL pumps need columns/document fields that don't exist on the base analytics record, but the source data already lives on `AnalyticsRecord.GraphQLStats`. A dedicated projection type keeps the persistence layer's schema explicit (`Types map[string][]string`, `RootFields []string`, `Errors []GraphError`) without polluting `AnalyticsRecord` itself. The extra `if a.ResponseCode >= 400 { record.HasErrors = true }` rule means HTTP-level failures are surfaced as GraphQL errors even when the resolver produced no `errors` array — useful for dashboards that key off `has_errors` regardless of whether the failure was transport or resolver.

## Code references
- `analytics/graph_record.go:49 (*AnalyticsRecord).ToGraphRecord` — `IsGraphRecord` gate, op-type switch, struct literal, HTTP-error override at line 72-74.
- `analytics/graph_record.go:14 GraphRecord` — the projection struct (fields tagged with `gorm:` for SQL, with the embedded `AnalyticsRecord` marked `bson:",inline"` for Mongo).
- `analytics/graph_record.go:29 TableName` — falls back to `AnalyticsRecord.TableName()` when `GraphSQLTableName` global is unset.
- `analytics/analytics.go:415 IsGraphRecord` — predicate on `GraphQLStats.IsGraphQL`.

## Evidence
- `analytics/graph_record_test.go:59 TestAnalyticsRecord_ToGraphRecordNew` — tagged `// Verifies: SW-REQ-013`; round-trips a populated `GraphQLStats` record and asserts each extracted field, including the `>= 400` override branch.

## Open questions
- `GraphSQLTableName` is a package-level variable mutated by the SQL pump before migration; if two SQL pumps with different table names are configured, the second `Init` overwrites the first. Not new (same pattern as `MCPSQLTableName`) but worth noting that the `TableName` resolution is global, not per-instance.
- `OperationType` is mapped to a string in `ToGraphRecord` but `GraphQLOperations` is also serialised as the raw int enum through the protobuf path (`serializer/protobuf.go:97`). Downstream consumers reading from one vs. the other see different value spaces.
- The `default:` arm of the op-type switch leaves `opType` as the empty string — a record with `OperationUnknown` ends up with `operation_type=""` in the DB rather than a marker value. Silent rather than wrong, but inconsistent with the protobuf path which uses `OPERATION_UNKNOWN`.
