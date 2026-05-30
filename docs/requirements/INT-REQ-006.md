# INT-REQ-006: Pump-to-backend schema mapping tolerance

## Intent
Each backend pump maps the canonical `analytics.AnalyticsRecord` onto its
backend's storage schema (BSON document for Mongo, table row for SQL, JSON
document for Elastic, etc.). This requirement asserts two properties of
that mapping: it must produce a backend-shaped record, and it must
tolerate unknown or future record fields without failing the write. It
satisfies SYS-REQ-004.

## Motivation
The core ships an evolving `AnalyticsRecord` struct (new fields are added
when the gateway grows new instrumentation, e.g. `GraphQLStats`, then
`MCPStats`). A backend pump compiled against an older snapshot of that
struct should still write the fields it knows about and silently ignore
the rest — otherwise every new field forces a coordinated release across
the pump and every backend integration. The tolerance property is what
makes the analytics pipeline forward-compatible across mixed-version
deployments (older Tyk Pump talking to a newer gateway, for example).

The mapping requirement is the *positive* side: each pump must actively
shape the record into its backend's schema (column names for SQL,
serialised payload for Kafka, metric labels for Prometheus) rather than
dump the Go struct verbatim.

## Code references
- Canonical record: `analytics/analytics.go:46` `AnalyticsRecord`, with
  GORM column tags (`gorm:"column:..."`) and `json` tags driving the SQL
  and JSON-shaped mappings respectively.
- Per-pump `WriteData` implementations do the schema mapping, e.g.:
  - SQL: `pumps/sql.go:269` `WriteData` → typed slice + day-sharded INSERT.
  - Mongo: `pumps/mongo.go` `WriteData` → `bson.M` documents.
  - Elasticsearch: `pumps/elasticsearch.go` `WriteData` → bulk index
    payload.
  - Kafka: `pumps/kafka.go` `WriteData` → JSON message per record.
- Unknown-field tolerance comes from the fact that the records reach each
  pump as `[]interface{}` of `analytics.AnalyticsRecord` values (typed at
  `main.go:327` `keys[i] = interface{}(decoded)`) — pumps cherry-pick the
  fields they care about and ignore the rest. The struct tags
  (`json:"-"`, `bson:"-"`, `gorm:"-:all"`) on fields like
  `GraphQLStats`/`MCPStats` (`analytics/analytics.go:79-80`) ensure
  newer fields do not leak into older backend schemas by default.
- MCP-record skip in non-MCP pumps: `pumps/sql.go:277-279`
  `if rec.IsMCPRecord() { continue }`.

## Evidence
- Per-pump `*_test.go` files exercise `WriteData` with representative
  records, including ones with later-added fields (e.g.
  `pumps/sql_test.go`, `pumps/sql_mysql_test.go`,
  `pumps/sql_pgxv5_test.go`, `pumps/mongo_test.go`,
  `pumps/elasticsearch_test.go`).
- `analytics/analytics_test.go` covers the struct-tag-driven SQL
  column mapping (`TestTableName` and similar).

## Open questions
- Tolerance is *de facto* — it comes from the use of struct tags and
  per-pump field selection, not from a written-down contract. A backend
  pump author could trivially add `db.Save(rec)` and that would no longer
  hold (GORM would attempt to create columns).
- No SYS req mandates a backward-compatibility test that adds an unknown
  field to `AnalyticsRecord` and asserts each pump still writes. Without
  that test, the "tolerance" claim is hard to regress-check.
- The `IsMCPRecord` skip is hand-coded in `sql.go`, `mongo.go`,
  `elasticsearch.go`, etc. A new non-MCP pump that forgets the check
  would write MCP records into a non-MCP backend — silent schema drift.
