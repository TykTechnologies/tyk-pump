# SW-REQ-093: REST aggregate partitioning

## Intent
`analytics.AggregateData` shall build REST analytics aggregates only from
ordinary analytics records. Records whose classifiers mark them as GraphQL
(`GraphQLStats.IsGraphQL`) or MCP (`MCPStats.IsMCP`) are excluded because those
families have separate graph and MCP aggregate paths.

## Motivation
The c54eed3 graph-separation change made graph analytics distinct from standard
Mongo aggregation, but the existing proof split only covered monotonic counter
accumulation (`SW-REQ-011`) and MongoAggregate window selection
(`SW-REQ-058`). This requirement makes the aggregate partition itself
auditable: REST records must still count, while graph/MCP records must not be
mixed into REST aggregate documents.

## Formalization
```
when rest_aggregate_input_present analytics shall always satisfy rest_aggregate_partitioned
```

Variables are declared in `specs/software/variables/analytics.vars.yaml`.

## Code References
- `analytics/aggregate.go:AggregateData` skips records when
  `IsGraphRecord()` or `IsMCPRecord()` is true before incrementing REST
  aggregate counters.
- `analytics/aggregate.go:AggregateGraphData` and
  `analytics/aggregate_mcp.go:AggregateMCPData` are the separate graph and MCP
  aggregate paths.
- `pumps/mongo_aggregate.go:MongoAggregatePump.WriteData` filters MCP records
  before calling `AggregateData`; `AggregateData` then applies the graph
  partition.

## Evidence
- `analytics/aggregate_test.go:TestAggregateData_SkipGraphRecords` proves
  GraphQL-classified records are excluded from REST aggregates while ordinary
  records remain present, including the same-organisation case where a graph
  record would otherwise inflate REST hit counts.
- `analytics/aggregate_mcp_test.go:TestAggregateData_SkipsMCPRecords` proves
  MCP-classified records are excluded from REST aggregates while ordinary
  records remain present.
