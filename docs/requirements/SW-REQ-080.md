# SW-REQ-080: Graph SQL migration table identity

## Intent
When `GraphSQLPump` is initialized with a configured table name, the
`analytics.GraphRecord` model table-name hook shall resolve to that configured
Graph SQL table before GORM migration generates table, index, or relationship
DDL.

## Motivation
TT-9855 (`3bf1f85`) fixed a Graph SQL startup migration failure where GORM
could consult `GraphRecord.TableName()` while building DDL and see the embedded
`AnalyticsRecord` default table instead of the configured Graph SQL table. The
fix binds `analytics.GraphSQLTableName` before `HandleTableMigration` calls
`AutoMigrate`.

This requirement pins that ordering as a contract. It is intentionally narrower
than the existing Graph/MCP sharded-index KI: this covers the non-sharded
configured-table migration identity, not a complete per-shard index policy.

## Code references
- `pumps/graph_sql.go` `GraphSQLPump.Init` sets `analytics.GraphSQLTableName`
  before `HandleTableMigration`.
- `analytics/graph_record.go` `(*GraphRecord).TableName` returns
  `GraphSQLTableName` when set and falls back to `AnalyticsRecord.TableName`
  only when the SQL Graph table binding is absent.

## Evidence
- `pumps/graph_sql_test.go`
  `TestGraphSQLPump_Init_BindsGraphRecordTableNameToConfiguredTable` runs the
  real Postgres `Init` path with a non-default table and asserts
  `GraphRecord.TableName()` resolves to that configured table, not
  `analytics.SQLTable`.

## Open questions
- `GraphSQLTableName` is a package-level binding. Multiple Graph SQL pump
  instances with different configured table names can overwrite this global;
  that residual multi-instance/shared-state risk is documented in DEFECT-10
  and remains outside this fixed TT-9855 closure.
