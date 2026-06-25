# SW-REQ-087: GraphQL RootFields projection is preserved

Documents: SW-REQ-087

## Contract

When `AnalyticsRecord.GraphQLStats.IsGraphQL` is true and
`GraphQLStats.RootFields` contains top-level GraphQL operation fields,
`AnalyticsRecord.ToGraphRecord` must copy those names into
`GraphRecord.RootFields`. Legacy raw request, response, tag, or schema payloads
must not become an alternate source for this field when `GraphQLStats.IsGraphQL`
is false.

This is a child of SW-REQ-013. SW-REQ-013 owns the broader GraphQLStats-to-
GraphRecord projection contract; SW-REQ-087 pins the TT-7977 RootFields field.

## Evidence

- `analytics/graph_record_test.go:TestAnalyticsRecord_ToGraphRecordNew` asserts
  `RootFields` is copied from a populated `GraphQLStats` source.
- `analytics/graph_record_test.go:TestAnalyticsRecord_ToGraphRecord_IgnoresLegacyGraphSourcesWithoutGraphQLStatsFlag`
  asserts legacy graph payloads do not create a GraphRecord when the structured
  GraphQLStats flag is false.
- `analytics/aggregate_test.go:TestAggregateGraphData_PartitionsSameOrgByAPIID`
  and Graph SQL pump tests provide downstream regression evidence that
  RootFields remain available to aggregate and SQL persistence paths.

## Known Issues

This requirement does not prove gateway-side RootFields extraction for every
GraphQL query shape. Pump proof starts at the `AnalyticsRecord.GraphQLStats`
input boundary; gateway production of that field is an upstream interface
assumption under SW-REQ-013.
