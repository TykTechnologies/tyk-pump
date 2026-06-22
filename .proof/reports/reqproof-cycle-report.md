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
**Audited window:** 28 May 2026 → 22 Jun 2026 (25 days) · baseline `b18d8ea` → `ac103ef`

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
| Coverage gaps (missing_*) | 0 |
| Accepted risks (active) | 13 |

_Corpus totals: 126 KnownIssues, 6 ProblemReports._

</section>

## 1. KnownIssues Resolved This Cycle

_Issues we previously reported to you that this release fixes — verified by tripwire tests that now pass._

_None resolved in this window._
## 2. Defects Introduced In This Release

_The audit's headline finding: bugs whose introducing change lands inside this window (`b18d8ea`..`ac103ef`) and that are STILL OPEN at HEAD. 0 qualify: 0 regression and 0 net-new. Each is also tracked as an open known issue and carries a committed reproducer; its introducing commit is linked below._

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

> **These bugs were both introduced and fixed before the `b18d8ea` 1.12.0 baseline — this release did not introduce them and they are not in the audited window; listed here only to record the coverage (requirement + tripwire) this audit added so they cannot silently regress.**

_No historical-backfill defects in scope._
## 7. Bugs Upstream Fixed That We'd Missed

_Bugs YOUR release fixed that no prior audit had on record — i.e. latent across the whole window. For each, we extended our corpus (a reproducer test, an opengrep signal, and an obligation class) so this defect class is now caught automatically next cycle._

_No missed-coverage defects in this window._
## 8. Gaps / Missed Coverage (root-cause flags)

_(internal) Spec/test coverage we had not yet written — our own corpus maturity, not your code quality._

_(The 0 missed-coverage defect(s) in the internal meta-audit section are also coverage gaps by construction; they are cross-referenced there and intentionally excluded from this table to avoid double-counting.)_

_No coverage-gap defects in this window._
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

