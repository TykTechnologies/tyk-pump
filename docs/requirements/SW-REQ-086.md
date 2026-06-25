# SW-REQ-086: Graph SQL aggregate sharded writes use the selected table

Documents: SW-REQ-086

## Contract

When `GraphSQLAggregatePump` runs with `TableSharding=true`, each contiguous
day slice is aggregated and written through its selected
`<AggregateGraphSQLTable>_<YYYYMMDD>` table. The helper that performs the
upsert must bind the GORM `Create` call to that same selected table rather than
falling back to the model's default base table.

This is a child of SW-REQ-043. SW-REQ-043 keeps the broader Graph SQL aggregate
contract; SW-REQ-086 pins the TT-7820 shard-target invariant.

## Evidence

- `pumps/graph_sql_aggregate_test.go:TestGraphSQLAggregatePump_WriteData_Sharded`
  enables table sharding, writes GraphQL aggregate records from two dates,
  verifies both day-shard tables exist, asserts representative aggregate rows
  exist exactly once in each selected shard, and asserts the base aggregate
  table remains absent.

## Known Issues

This requirement does not close Graph SQL aggregate transaction/failure
injection, migration of old shards, shard-index completeness, timeout,
configuration schema, or timezone convention debt. Those remain tracked under
the KnownIssues linked from DEFECT-19 and SW-REQ-043.
