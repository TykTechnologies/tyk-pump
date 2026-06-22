# Audit Cycle Report — tyk-pump

<section class="exec-summary" markdown="1">

## Executive Summary

> **Verdict:** No new bugs introduced and none resolved in this window — posture unchanged.

| Signal | Count | Detail |
|--------|------:|--------|
| ✅ Resolved this cycle | **0** | — |
| 🔴 Introduced this release (still open) | **0** | 0 regression + 0 net-new |
| 🟥 …of which HIGH-severity | **0** | — |
| 📋 Pre-existing backlog (gated, reproduced) | **126** | — |
| 🔍 Bugs you fixed that we'd missed | **0** | — |

</section>
**Range:** _snapshot (no commit window)_

> ⚠️ no --from given: report is a point-in-time snapshot; every artifact is treated as in-range

<section class="summary-card" markdown="1">

## Summary

_Rows are non-overlapping: every open KnownIssue is counted exactly once — either as introduced this release (regression + net-new) or in the pre-existing still-open backlog._

| Bucket | Count |
|--------|------:|
| KnownIssues resolved this cycle | 0 |
| Defects introduced this release (0 regression + 0 net-new) | 0 |
| Pre-existing still-open backlog | 126 |
| Bugs upstream fixed that we'd missed | 0 |
| Historical coverage backfill (fixed before baseline; not in window) | 0 |
| Coverage gaps (missing_*) | 5 |
| Accepted risks (active) | 13 |

_Corpus totals: 126 KnownIssues, 6 ProblemReports._

</section>

## 1. KnownIssues Resolved This Cycle

_Issues we previously reported to you that this release fixes — verified by tripwire tests that now pass._

_None resolved in this window._
## 2. Defects Introduced In This Release

_The audit's headline finding: bugs whose introducing change lands inside this window (`snapshot`) and that are STILL OPEN at HEAD. 0 qualify: 0 regression and 0 net-new. Each is also tracked as an open known issue and carries a committed reproducer; its introducing commit is linked below._

### 2a. Regression(s) introduced this release

_No regressions introduced this release._

### 2b. Net-new defects introduced this release

_No net-new defects introduced this release._

## 3. Pre-Existing Still-Open Backlog

_Issues still present in this release, each gated to a requirement and reproduced._

_126 active KnownIssues total. The HIGH-severity items (and any auditor-pinned items) are listed in full below; the MEDIUM/LOW tail is summarized by count and component so the backlog's shape is visible without a 90-row table._

### 2a. HIGH-severity & pinned active issues (full detail)


| KI | Status | Severity | Age | Gated requirements | Title |
|----|--------|----------|-----|--------------------|-------|
| [`docs-mcp-pump-family-entirely-undocumented`](#docs-mcp-pump-family-entirely-undocumented) | open | 🟧 HIGH | since inception | SW-REQ-044 | Documentation gap: MCP pump family (4 backends) has zero men… |
| [`docs-missing-per-pump-config-sections`](#docs-missing-per-pump-config-sections) | open | 🟧 HIGH | since inception | SW-REQ-017 | Documentation gap: 11 registered pump types have no per-pump… |
| [`pump-writedata-swallows-per-batch-errors`](#pump-writedata-swallows-per-batch-errors) | open | 🟧 HIGH | since inception | INT-REQ-004, SW-REQ-034, SW-REQ-035, SW-REQ-036, SW-REQ-040, SW-REQ-046, SW-REQ-047, SW-REQ-049, SW-REQ-050, SW-REQ-051, SW-REQ-053, SW-REQ-056 | Multiple pumps log per-batch WriteData errors but return nil… |
| [`resurface-maprawdata-empty-request-panic`](#resurface-maprawdata-empty-request-panic) | open | 🟧 HIGH | since inception | SW-REQ-054 | Resurface mapRawData panics on AnalyticsRecord with empty Ra… |
| [`sql-batch-size-zero-infinite-loop`](#sql-batch-size-zero-infinite-loop) | open | 🟧 HIGH | since inception | SW-REQ-040, SW-REQ-041, SW-REQ-042, SW-REQ-043, SW-REQ-044, SW-REQ-045 | SQL-family batch loops can stall forever when BatchSize is z… |
| [`sqs-batchlimit-zero-infinite-loop`](#sqs-batchlimit-zero-infinite-loop) | open | 🟧 HIGH | since inception | SW-REQ-055 | SQS pump infinite-loops when AWSSQSBatchLimit is unset (defa… |


### 2b. MEDIUM / LOW backlog (summarized)

**Plus 120 more active KnownIssue(s) not listed individually** — 69 medium, 51 low.

These are tracked, each gated to a requirement and reproduced. By component area:

| Area | Count |
|------|------:|
| sql | 9 |
| docs | 7 |
| mongo | 6 |
| uptime | 6 |
| kafka | 5 |
| health | 4 |
| moesif | 4 |
| pump | 4 |
| retry | 4 |
| storage | 4 |
| analytics | 3 |
| elasticsearch | 3 |
| main | 3 |
| mcp | 3 |
| prometheus | 3 |
| resurface | 3 |
| splunk | 3 |
| sw | 3 |
| aggregate | 2 |
| getanddeleteset | 2 |
| graylog | 2 |
| http | 2 |
| hybrid | 2 |
| instrumentation | 2 |
| kinesis | 2 |
| syslog | 2 |
| temporal | 2 |
| common | 1 |
| config | 1 |
| csv | 1 |
| elapsed | 1 |
| env | 1 |
| es | 1 |
| external | 1 |
| filterdata | 1 |
| influx | 1 |
| logfatal | 1 |
| logzio | 1 |
| mapstructure | 1 |
| mcdc | 1 |
| metrics | 1 |
| no | 1 |
| preprocess | 1 |
| protobuf | 1 |
| pumps | 1 |
| serializer | 1 |
| setaggregatetimestamp | 1 |
| sqs | 1 |
| stdout | 1 |
| systemconfig | 1 |
| tls | 1 |
| write | 1 |

## 6. Historical Coverage Backfill — bugs fixed in prior releases, coverage added this cycle

_(internal) Bugs both introduced AND fixed before the baseline — not in the audited window. Listed only to record the coverage (requirement + tripwire) this audit added so they cannot silently regress._

> **These bugs were both introduced and fixed before the 1.12.0 baseline — this release did not introduce them and they are not in the audited window; listed here only to record the coverage (requirement + tripwire) this audit added so they cannot silently regress.**

_No historical-backfill defects in scope._
## 7. Bugs Upstream Fixed That We'd Missed

_Bugs YOUR release fixed that no prior audit had on record — i.e. latent across the whole window. For each, we extended our corpus (a reproducer test, an opengrep signal, and an obligation class) so this defect class is now caught automatically next cycle._

_No missed-coverage defects in this window._
## 8. Gaps / Missed Coverage (root-cause flags)

_(internal) Spec/test coverage we had not yet written — our own corpus maturity, not your code quality._

_(The 0 missed-coverage defect(s) in the internal meta-audit section are also coverage gaps by construction; they are cross-referenced there and intentionally excluded from this table to avoid double-counting.)_


| DEFECT | missing_req | missing_test | missing_mcdc | disposition | Title |
|--------|:-----------:|:------------:|:------------:|-------------|-------|
| [`DEFECT-1`](#defect-1) | no | yes | no | covered_by_requirement | MCP Mongo aggregate cross-API document merge |
| [`DEFECT-2`](#defect-2) | no | yes | no | covered_by_requirement | Syslog pump log fragmentation on multiline raw_request/raw_r… |
| [`DEFECT-3`](#defect-3) | no | yes | no | covered_by_requirement | SQL table-sharding pump skipped schema migration of sharded … |
| [`DEFECT-4`](#defect-4) | no | yes | no | covered_by_requirement | SQL pump panics (slice index out of range) when sharding ena… |
| [`DEFECT-5`](#defect-5) | yes | no | no | covered_by_requirement | Prometheus pump exposed full API keys as metric label values |

## 9. Accepted Risks (Active)

_Risks the owner has formally accepted, with a review-by date — open and tracked, not silently ignored._

| RISK | Severity/Status | Accepted by | Review due | Title |
|------|-----------------|-------------|-----------|-------|
| `elasticsearch-unbounded-reconnect-stack-overflow` | active | human:leo | 2026-08-30 | Elasticsearch pump's unbounded reconnect recursion can stack… |
| `graylog-moesif-record-fatal` | active | human:leo | 2026-08-30 | Graylog and Moesif pumps log.Fatal on per-record errors, cra… |
| `mongo-pump-context-timeout-bypass` | active | human:leo | 2026-08-30 | Ship mongo pump without per-pump write-timeout enforcement |
| `no-dlq-on-pump-write-failure` | active | human:leo | 2026-08-30 | No dead-letter queue: pump write failures after successful R… |
| `per-pump-timeout-not-enforced-when-zero` | active | human:leo | 2026-08-30 | Per-pump timeout=0 disables timeout enforcement entirely |
| `pump-config-typos-silently-ignored` | active | human:leo | 2026-08-30 | mapstructure.Decode in every pump Init silently drops unknow… |
| `pump-panic-crashes-process` | active | human:leo | 2026-08-30 | Pump goroutine panic crashes entire tyk-pump process (no rec… |
| `pump-writedata-swallows-per-batch-errors` | active | human:leo | 2026-11-30 | Pump.WriteData per-batch error swallowing across 11 pumps |
| `serializer-protobuf-city-names-data-loss` | active | human:leo | 2026-08-30 | Protobuf serializer round-trip drops City.Names map |
| `sql-default-migration-today-only-data-loss` | active | human:leo | 2026-08-30 | SQL default migration covers only current day; late-arriving… |
| `statsd-setup-fatal-aborts-pump` | active | human:leo | 2026-08-30 | StatsD setup failure log.Fatals the pump at startup |
| `storage-pop-expire-non-atomic-data-loss` | active | human:leo | 2026-08-30 | Redis Pop+Expire non-atomic: Expire failure silently loses p… |
| `uptime-pump-silent-failure` | active | human:leo | 2026-08-30 | UptimePump.WriteUptimeData has no error return; backend fail… |

## Appendix: Entity Detail

_One record per entity shown in this profile. Headings are the verbatim entity id, so the in-table links above resolve here._

#### docs-mcp-pump-family-entirely-undocumented
- **Title:** Documentation gap: MCP pump family (4 backends) has zero mentions in tyk-docs
- **Severity:** high _(basis: risk · correctness)_
- **Status:** open
- **Description:** pumps/init.go:39-42 registers mongo-mcp, mongo-mcp-aggregate, sql-mcp, sql-mcp-aggregate. SW-REQ-038/039/044/045 cover them. KIs mcp-sql-aggregate-mysql-create-index-syntax-broken and elasticsearch-mcp-routing-non-bulk-ignored describe MCP-specific bugs. None of this is reachable from tyk-docs: zero matches for 'MCP' across api-management/tyk-pump.md, shared/pump-config.md, or developer-support/release-notes/pump.md. Operators discovering MCP must read the source.
- **Root cause / remediation:** Add (a) an MCP analytics overview section to tyk-docs/content/api-management/tyk-pump.md, (b) per-pump config sections for the 4 MCP backends in tyk-docs/content/shared/pump-config.md (overlaps with KI docs-missing-per-pump-config-sections), (c) release-note entry in tyk-docs/content/developer-support/release-notes/pump.md for the version that introduced MCP.
- **Linked requirement(s):** SW-REQ-044
- **Reproducer / evidence tests:**
    - `grep -RIn -e 'MCP' -e 'mcp' /Users/buger/go/src/tyk-docs/tyk-docs/content/api-management/tyk-pump.md /Users/buger/go/src/tyk-docs/tyk-docs/content/shared/pump-config.md /Users/buger/go/src/tyk-docs/tyk-docs/content/developer-support/release-notes/pump.md || echo 'no matches'`
    - `grep -RIn -e 'MCP' -e 'mcp' /Users/buger/go/src/tyk-docs/tyk-docs/content/api-management/tyk-pump.md /Users/buger/go/src/tyk-docs/tyk-docs/content/shared/pump-config.md /Users/buger/go/src/tyk-docs/tyk-docs/content/developer-support/release-notes/pump.md || echo 'no matches'`

#### docs-missing-per-pump-config-sections
- **Title:** Documentation gap: 11 registered pump types have no per-pump configuration section in tyk-docs
- **Severity:** high _(basis: risk · correctness)_
- **Status:** open
- **Description:** pumps/init.go:9-46 registers 30 pump types; tyk-docs/content/shared/pump-config.md is missing per-pump tables for: influx2, segment, sqs, resurfaceio, mongo-graph, sql-graph, sql-graph-aggregate, mongo-mcp, mongo-mcp-aggregate, sql-mcp, sql-mcp-aggregate. Operators have no canonical reference for these pumps' meta blocks.
- **Root cause / remediation:** Add per-pump configuration sections in tyk-docs/content/shared/pump-config.md for all 11 missing pump types. Highest priority for the four MCP variants (entire feature surface invisible).
- **Linked requirement(s):** SW-REQ-017
- **Reproducer / evidence tests:**
    - `grep -E 'pmp\..*=' /Users/buger/go/src/tyk-pump/pumps/init.go | head -40`
    - `grep -E 'pmp\..*=' /Users/buger/go/src/tyk-pump/pumps/init.go | head -40`

#### pump-writedata-swallows-per-batch-errors
- **Title:** Multiple pumps log per-batch WriteData errors but return nil, breaking Pump-interface error-return contract
- **Severity:** high _(basis: risk · data_integrity)_
- **Status:** open
- **Description:** INT-REQ-004 contracts that Pump.WriteData returns an error on backend failure. In practice, several pumps log per-batch errors via their logger and return nil. Audit of pumps/init.go-registered implementations: pumps/mongo.go:464 (logs MongoErr, returns nil); pumps/mongo_selective.go:266 (logs InsertError, returns nil); pumps/mongo_aggregate.go:307 (logs MongoUpsertErr, returns nil); pumps/kinesis.go:185 (logs PutRecordErr, returns nil); pumps/sql.go:333 (logs Create-batch error, returns nil); pumps/graylog.go:174 (logs and returns nil); pumps/syslog.go:138 (logs and returns nil); pumps/logzio.go:185 (logs and returns nil); pumps/segment.go:121 (logs and returns nil); pumps/influx.go:158 (logs WritePoint error, returns nil); pumps/influx2.go:178 (logs FlushErr, returns nil). Affects the caller (main.execPumpWriting) which interprets nil as success and increments success counters.
- **Root cause / remediation:** Refactor each named pump's WriteData to wrap per-batch errors into a multierr or surface the last error; update tests + KI status to fixed.
- **Linked requirement(s):** INT-REQ-004, SW-REQ-034, SW-REQ-035, SW-REQ-036, SW-REQ-040, SW-REQ-046, SW-REQ-047, SW-REQ-049, SW-REQ-050, SW-REQ-051, SW-REQ-053, SW-REQ-056

#### resurface-maprawdata-empty-request-panic
- **Title:** Resurface mapRawData panics on AnalyticsRecord with empty RawRequest but non-empty RawResponse
- **Severity:** high _(basis: risk · availability)_
- **Status:** open
- **Description:** pumps/resurface.go:166 (mapRawData) splits the decoded raw request on whitespace via strings.Fields(req[0]) and indexes [1] to extract the path. When AnalyticsRecord.RawRequest is empty but RawResponse is not, the partial-raw guard at resurface.go:253 (len==0 && len==0) does NOT skip the record, and mapRawData proceeds to panic with 'index out of range [1] with length 0'. The whole pump goroutine crashes and the buffered channel is left in a broken state. Reachable from any deployment that produces records with one side of the raw HTTP transaction missing.
- **Root cause / remediation:** Either tighten the early-return guard in writeData to skip records with EITHER side empty (change && to ||), or harden mapRawData against empty rawReq/rawRes inputs by returning an error instead of indexing.
- **Linked requirement(s):** SW-REQ-054
- **Reproducer / evidence tests:**
    - `go test -count=1 -run 'TestResurfacePump_MapRawData_EmptyRequest_PanicKI' ./pumps`
    - `go test -count=1 -run 'TestResurfacePump_MapRawData_EmptyRequest_PanicKI' ./pumps`

#### sql-batch-size-zero-infinite-loop
- **Title:** SQL-family batch loops can stall forever when BatchSize is zero or negative
- **Severity:** high _(basis: risk · availability)_
- **Status:** open
- **Description:** The SQL, Graph SQL, MCP SQL, SQL aggregate, Graph SQL aggregate, and MCP SQL aggregate write paths iterate batches with loops of the form for i := 0; i < len(recs); i += BatchSize. If configuration or test wiring leaves BatchSize at zero, the loop never advances; if it is negative, slicing boundaries become invalid. The code does not consistently validate BatchSize before these loops, so input_size_bounded and malformed_input obligations for these paths are real tracked debt.
- **Root cause / remediation:** Validate BatchSize during Init for every SQL-family pump and defensively clamp or reject non-positive values before entering the batching loops.
- **Linked requirement(s):** SW-REQ-040, SW-REQ-041, SW-REQ-042, SW-REQ-043, SW-REQ-044, SW-REQ-045
- **Reproducer / evidence tests:**
    - `rg -n "for i := 0; i < len\\(recs\\); i \\+= .*BatchSize|for i := 0; i < len\\(typedData\\); i \\+= .*BatchSize" pumps/sql.go pumps/sql_aggregate.go pumps/graph_sql.go pumps/graph_sql_aggregate.go pumps/mcp_sql.go pumps/mcp_sql_aggregate.go`

#### sqs-batchlimit-zero-infinite-loop
- **Title:** SQS pump infinite-loops when AWSSQSBatchLimit is unset (default 0)
- **Severity:** high _(basis: risk · availability)_
- **Status:** open
- **Description:** pumps/sqs.go:178-197 (write method): the batching loop is 'for i := 0; i < len(messages); i += s.SQSConf.AWSSQSBatchLimit'. When AWSSQSBatchLimit is the int zero-value (not set in YAML or env), i never advances and the loop runs forever — sending the same batch every iteration. There is NO default applied in Init (pumps/sqs.go:97-128 has no defaulting); only AWSSQSBatchLimit is read from config and passed through. Furthermore the AWS SQS API caps SendMessageBatch entries at 10; any value > 10 will be rejected per-call. No validation enforces the 1..10 range.
- **Root cause / remediation:** In SQSPump.Init: 'if s.SQSConf.AWSSQSBatchLimit <= 0 || s.SQSConf.AWSSQSBatchLimit > 10 { s.SQSConf.AWSSQSBatchLimit = 10 }'. Document the AWS 10-entry cap in the docs config snippet.
- **Linked requirement(s):** SW-REQ-055
- **Reproducer / evidence tests:**
    - `rg -n "i \+= s\.SQSConf\.AWSSQSBatchLimit|AWSSQSBatchLimit" pumps/sqs.go`

#### DEFECT-1
- **Title:** MCP Mongo aggregate cross-API document merge
- **Severity:** high
- **Status:** covered_by_requirement
- **Introduced:** [`be39ad3`](https://github.com/TykTechnologies/tyk-pump-proof/commit/be39ad3) _(committed 2026-04-28)_
- **Fixing / upstream ref:** TT-17004
- **Description:** MCPMongoAggregatePump.DoMCPAggregatedWriting built its upsert match query
on only (orgid, timestamp), so concurrent aggregate writes from two MCP
proxies that landed in the same time bucket merged into a single MongoDB
document. The dashboard's per-api_id filter (apiid.<X>: $exists) still
matched that merged document; unwinding lists.names then returned counters
from BOTH proxies, and a query scoped to one proxy reported the sum
(e.g. 5+2=7) instead of just that proxy's traffic — silent cross-tenant
metric corruption.
Fixed by adding OwnerAPIID (bson "owner_apiid") to MCPRecordAggregate,
populating it from record.APIID in initMCPAggregateForRecord, and adding
it to the upsert match query so each (org, timestamp, api) gets its own
document. Original ticket TT-17004 (#989), tag v1.15.0-rc1.

- **Root cause / remediation:** A requirement for the MCP-mongo-aggregate upsert existed conceptually but
the per-(org,timestamp,api) document-partitioning key was never asserted
by a test partition: no test wrote two records from different APIs sharing
one (org,timestamp) and checked they produced two distinct documents.
The regression test added in the fix (PerAPIPartitioning) closes that gap.
- **Affected files:**
    - [`pumps/mcp_mongo_aggregate.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/mcp_mongo_aggregate.go)
    - [`analytics/aggregate_mcp.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/analytics/aggregate_mcp.go)
- **Linked requirement(s):** SW-REQ-039
- **Reproducer / evidence tests:**
    - [`pumps/mcp_mongo_aggregate_test.go:TestMCPMongoAggregatePump_WriteData_PerAPIPartitioning`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/mcp_mongo_aggregate_test.go)

#### DEFECT-2
- **Title:** Syslog pump log fragmentation on multiline raw_request/raw_response
- **Severity:** medium
- **Status:** covered_by_requirement
- **Introduced:** [`0596e82`](https://github.com/TykTechnologies/tyk-pump-proof/commit/0596e82) _(committed 2025-08-16)_
- **Fixing / upstream ref:** TT-15532
- **Description:** SyslogPump.WriteData emitted each analytics record to the syslog writer with
fmt.Fprintf("%s", message) where message was a Go map whose raw_request /
raw_response fields contained embedded newlines from real HTTP payloads. The
syslog daemon interpreted every newline as a record boundary, so a single
analytics record fragmented into many syslog entries — corrupting downstream
log ingestion and making records unparseable.
First fix (#884, commit df62011) JSON-marshalled the whole message; the
shipped backward-compatible fix (TT-15532, #886, commit 0596e82) kept the
original map[...] output format and instead escaped \n -> \\n only in
raw_request and raw_response, guaranteeing one single-line syslog entry per
record. Sibling commits: df62011 (superseded approach) + 0596e82 (final).

- **Root cause / remediation:** No test partition fed a record whose raw_request/raw_response contained
newlines and asserted single-line syslog output. The requirement's
single-line / \n-escaping guarantee was implicit until SW-REQ-050 was
written to state it explicitly; the regression test added with the fix
(WithMultilineHTTP) now exercises that partition.
- **Affected files:**
    - [`pumps/syslog.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/syslog.go)
- **Linked requirement(s):** SW-REQ-050
- **Reproducer / evidence tests:**
    - [`pumps/syslog_test.go:TestSyslogPump_WriteData_WithMultilineHTTP`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/syslog_test.go)

#### DEFECT-3
- **Title:** SQL table-sharding pump skipped schema migration of sharded tables
- **Severity:** high
- **Status:** covered_by_requirement
- **Introduced:** [`19fb73d`](https://github.com/TykTechnologies/tyk-pump-proof/commit/19fb73d) _(committed 2025-10-23)_
- **Fixing / upstream ref:** TT-13166
- **Description:** Before TT-13166 (#905, commit 19fb73d) every SQL pump (standard, aggregate,
graph) gated AutoMigrate behind `if !TableSharding { ... }`. With sharding
enabled the migration step was therefore skipped entirely, so newly created
per-day shard tables (tyk_analytics_<YYYYMMDD>, tyk_aggregated_<YYYYMMDD>,
graph shards) never received schema columns added by later releases —
inserts referencing the new columns failed against the stale shard. The fix
extracts a shared HandleTableMigration / MigrateAllShardedTables helper
(pumps/common.go) used by sql.go, sql_aggregate.go and graph_sql.go, adds the
MigrateShardedTables config flag, and also fixed a sub-bug where the config
flag did not overwrite correctly ("fixed bug that didn't allow for config
overwrite").
Residual default-mode gap: when MigrateShardedTables defaults to false, only
the current day's shard is migrated — tracked as KI
sql-default-migration-today-only (open, ship_with_known_issue) and the doc
gap as KI docs-sql-migrate-sharded-tables-undocumented.

- **Root cause / remediation:** The sharding-enabled branch had no test asserting that existing shard
tables get migrated to the latest schema; the AutoMigrate skip went
unnoticed. The fix adds migration_test.go covering HandleTableMigration's
three branches (non-sharded / migrate-all / current-day-only) and
MigrateAllShardedTables across dialects.
- **Affected files:**
    - [`pumps/common.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/common.go)
    - [`pumps/sql.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/sql.go)
    - [`pumps/sql_aggregate.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/sql_aggregate.go)
    - [`pumps/graph_sql.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/graph_sql.go)
- **Linked requirement(s):** SW-REQ-040
- **Reproducer / evidence tests:**
    - [`pumps/migration_test.go:TestHandleTableMigration`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/migration_test.go)
    - [`pumps/migration_test.go:TestMigrateAllShardedTables`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/migration_test.go)

#### DEFECT-4
- **Title:** SQL pump panics (slice index out of range) when sharding enabled and all records skipped
- **Severity:** high
- **Status:** covered_by_requirement
- **Introduced:** [`fbcb614`](https://github.com/TykTechnologies/tyk-pump-proof/commit/fbcb614) _(committed 2024-11-27)_
- **Fixing / upstream ref:** TT-12780
- **Description:** SQLPump.WriteData's sharding day-slice loop indexed typedData[startIndex]
unconditionally inside `if c.SQLConf.TableSharding`. When TableSharding was
enabled and the incoming batch was empty after the skip-api-id / MCP filter
drained every record (or an empty purge), startIndex (0) was >= len(typedData)
(0) and the access panicked with index-out-of-range, crashing the pump's
purge cycle. TT-12780 (#860) guards the access with
`c.SQLConf.TableSharding && startIndex < len(typedData)`.

- **Root cause / remediation:** The empty-input partition under TableSharding=true was never tested, so
the unguarded slice access shipped. The fix adds the empty_keys subtest
to TestSQLWriteDataSharded asserting WriteData returns no error and
creates no shard tables for an empty batch. Related but distinct from the
fan-out panic-recovery gap tracked by KI
no-panic-recovery-in-exec-pump-writing (that concerns missing recover()
in goroutines; this is a deterministic bounds bug now positively tested).
- **Affected files:**
    - [`pumps/sql.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/sql.go)
- **Linked requirement(s):** SW-REQ-040
- **Reproducer / evidence tests:**
    - [`pumps/sql_test.go:TestSQLWriteDataSharded`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/sql_test.go)

#### DEFECT-5
- **Title:** Prometheus pump exposed full API keys as metric label values
- **Severity:** high
- **Status:** covered_by_requirement
- **Introduced:** [`09aaa8d`](https://github.com/TykTechnologies/tyk-pump-proof/commit/09aaa8d) _(committed 2025-02-12)_
- **Fixing / upstream ref:** TT-13937
- **Description:** PrometheusMetric.GetLabelsValues mapped decoded.APIKey verbatim into the
"api_key"/"key" Prometheus label values. Any operator scraping /metrics — and
any downstream metrics store / dashboard — therefore received cleartext API
keys as high-cardinality label values, a credential-exposure / secrets-in-
telemetry problem. TT-13937 (#866, commit 09aaa8d) added the ObfuscateAPIKeys
(default false) and ObfuscateAPIKeysLength (default 4) config and an
obfuscateAPIKey() helper that, when enabled, masks the key to "****" + last 4
chars (or "--" for keys <= 4 chars).
Security note: the masking is OPT-IN; the default (ObfuscateAPIKeys=false)
still emits full keys, so the secrets-in-telemetry exposure persists unless an
operator turns it on.

- **Root cause / remediation:** No requirement in the corpus covers API-key obfuscation for the Prometheus
pump. SW-REQ-024 only states the pump "shall expose analytics as
Prometheus metrics for scraping" and carries an auth_required obligation
for the exposition endpoint — it says nothing about masking secret label
values. SW-REQ-048 (Splunk) is the only requirement mentioning
ObfuscateAPIKeys, and it is scoped to the Splunk pump, not Prometheus.
The behavior is exercised by tests (TestPrometheusGetLabelsValues has
obfuscation cases) but there is no requirement to anchor those tests to a
security guarantee, and the insecure default is unspecified.

HARDENING (2026-06-10, agent:claude-opus-4-8): the assurance gap was
closed end-to-end, not just recorded:
  * NEW requirement SW-REQ-074 (first authored under a placeholder id
    in the 209 range, then renumbered to close the 074-208 numbering
    gap so ids are contiguous 001-074) (component pumps-prometheus,
    assurance_level B, parent SYS-REQ-016 / privacy) models the masking
    guarantee in FRETish: "when obfuscate_api_keys_enabled
    pumps_prometheus shall always satisfy api_key_label_masked", with
    acceptance criteria pinning the ****+last-N contract, the "--"
    fully-masked branch, and the documented opt-in (off) default.
  * OBLIGATION class secrets_not_logged (CWE-532, OWASP-ASVS-v4 V7.1.1)
    attached to SW-REQ-074's obligation_checklist — the catalog's
    "secrets/PII never appear in logs/telemetry" class, the exact failure
    mode of this defect.
  * CODE annotated: pumps/prometheus.go obfuscateAPIKey carries
    // reqproof:implements SW-REQ-074 (implemented_by trace).
  * REGRESSION TEST + MC/DC: pumps/prometheus_test.go
    TestPrometheusObfuscateAPIKey asserts (a) enabled+long key masks to
    ****+last4 with the raw key NotContains the label, (b) enabled+short
    key fully masks to "--", (c) disabled returns the raw key (the
    documented opt-in default). All three SW-REQ-074 MC/DC witness rows
    are covered (// MCDC SW-REQ-074: ...). The F/T=FALSE violation row
    (obfuscation on yet label unmasked) is guarded by the NotContains
    assertions, so a regression that re-leaked the key fails the test and
    the mcdc_coverage obligation.
SW-REQ-074 is audit-clean (approvals_current, suspect_clean,
coverage_met, mcdc_coverage, obligation_evidence_complete,
variables_declared, cross_level_complete all pass for it).
- **Affected files:**
    - [`pumps/prometheus.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/prometheus.go)
- **Linked requirement(s):** SW-REQ-074
- **Reproducer / evidence tests:**
    - [`pumps/prometheus_test.go:TestPrometheusObfuscateAPIKey`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/prometheus_test.go)

#### DEFECT-6
- **Title:** Elasticsearch pump shutdown did not close ES clients
- **Severity:** medium
- **Status:** covered_by_requirement
- **Introduced:** inception
- **Fixing / upstream ref:** reqproof-coverage-authored-delta-review
- **Description:** ElasticsearchPump.Shutdown only flushed the bulk processor when bulk mode was
enabled. It did not close the per-version BulkProcessor or stop the elastic
client, and it did not guard repeated Shutdown calls. That left backend client
resources owned by the Elasticsearch pump alive after the pump host invoked
shutdown, weakening the clean-shutdown/resource-lifetime contract for the ES
pump family.

The current branch changes pumps/elasticsearch.go so each per-version
operator implements close(), Shutdown is idempotent, bulk flush errors are
preserved, and the operator close path is invoked for both bulk and non-bulk
configurations.

- **Root cause / remediation:** SW-REQ-070 already stated that Elasticsearch shutdown flushes the bulk
processor, and SYS/SW shutdown requirements cover clean pump lifecycle.
The implementation did not fully release the resources owned by that
lifecycle surface. Existing shutdown-path tests exercise the bulk and
non-bulk Shutdown paths; this report records the defect trail for the
implementation delta rather than minting a new requirement.
- **Affected files:**
    - [`pumps/elasticsearch.go`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/elasticsearch.go)
- **Linked requirement(s):** SW-REQ-070, SW-REQ-068, SW-REQ-069
- **Reproducer / evidence tests:**
    - [`pumps/elasticsearch_mcdc_test.go:TestElasticsearchPump_Shutdown_BulkPath`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/elasticsearch_mcdc_test.go)
    - [`pumps/elasticsearch_mcdc_test.go:TestElasticsearchPump_Shutdown_BulkDisabled`](https://github.com/TykTechnologies/tyk-pump-proof/blob/feat/reqproof-coverage/pumps/elasticsearch_mcdc_test.go)

---

## Appendix: Data-Gap Analysis

_Fields this report had to infer or reconstruct because the model does not carry
them as first-class data. Occurrences = records hitting the gap in this run._

| Field | Severity | Occurrences | Came from | Proposed model addition |
|-------|----------|------------:|-----------|-------------------------|
| `KnownIssue.resolved_at` | lossy | 0 | inferred from history[].at of the entry whose detail matches a status->fixed transition | add `resolved_at string` (RFC3339), set by `proof known-issue resolve` |
| `KnownIssue.resolved_in` | blocking | 0 | reconstructed from non-model `fixing_reference:` key, else scavenged #PR/SHA from history detail or remediation prose | add `resolved_in string` (fixing commit-ish); promote the de-facto `fixing_reference` YAML key into the model so it stops being dropped on load |
| `KnownIssue.created_at` | lossy | 0 | inferred from the first history[] entry (action=created) | add `created_at string`; age/SLA math currently depends on a history convention |
| `ProblemReport.detected_at` | lossy | 0 | source.date, else regression.detected_at (when a date), else first history entry | add a report-level `detected_at`; regression.detected_at is overloaded (sometimes a SHA, not a date) |
| `ProblemReport.regression.dwell` | cosmetic | 0 | read directly when present; frequently empty — not derivable without bisect | auto-derive dwell from introduced_in..fixed_in via git, or require it on regression closure |

