# KnownIssue / Risk Quality Audit

Date: 2026-06-26
Repo: TykTechnologies/tyk-pump
Branch reviewed locally: feat/reqproof-coverage

## Scope

This audit reviews active KnownIssue, accepted risk, and KI-backed tracked-debt
records for evidence quality, provenance, severity, and closure readiness. It
does not fix product behavior. Real unfixed behavior remains a KnownIssue or
accepted risk; historically fixed behavior belongs in DEFECT records.

Current inventory at audit start:

- Active KnownIssues: 144
- Active accepted risks: 13
- Active waivers: 0
- KI-backed code-signal debt rows: 167
- KI-backed MC/DC debt rows: 1
- KI-backed obligation-decomposition debt rows: 405
- KI-backed obligation-evidence debt rows: 407
- Deferred acceptance-criteria debt rows: 1

## Verdicts

- `valid`: record is well formed; evidence/reproducer matches the stated issue;
  provenance, affected requirements, severity, disposition, and review metadata
  are adequate.
- `needs_metadata_hardening`: behavior/evidence is sound, but fields such as
  review date, origin, owner, affected requirements, severity rationale, or
  release disposition need tightening.
- `needs_reproducer_hardening`: issue is plausible, but the reproducer is too
  weak, too broad, missing, or does not exercise the stated defect.
- `needs_split`: one record covers multiple defect classes that should be
  tracked independently.
- `needs_merge`: multiple records duplicate the same defect class.
- `stale_or_resolved`: current code or proof state indicates the record may no
  longer be active and should be closed or converted.
- `invalid`: record does not describe a real product/proof debt item.

## Review Log

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `elapsed-timesince-dst-obligation-misattributed` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static reproducer `rg -n "time\\.Since\\(" main.go pumps/kafka.go pumps/sqs.go retry/http-retry.go`; source sites are elapsed-duration metrics/logging, not date-shard business arithmetic. | Stamped review date `2026-11-30`; keep open as proof/spec attribution debt, not product defect. |
| `elasticsearch-api-key-auth-dropped-when-use-ssl` | KnownIssue | `valid` | Ran `go test -p=1 -count=1 -timeout=20m ./pumps -run '^TestElasticsearchPump_ApiKeyAuthDroppedWhenUseSSL_KI$'`; inspected `pumps/elasticsearch.go:getOperator`, where API-key transport is assigned before the `UseSSL` branch replaces `httpClient` with a TLS-only transport. | Stamped review date `2026-11-30`; no product fix made. |
| `elasticsearch-decode-base64-errors-silent-empty` | KnownIssue | `valid` | Ran `go test -p=1 -count=1 -timeout=20m ./pumps -run '^TestGetMapping_DecodeBase64MalformedInput_KI$'`; inspected `getMapping` malformed base64 behavior. | Already had review date `2026-11-30`; no change needed. |
| `elasticsearch-mcp-routing-non-bulk-ignored` | KnownIssue | `valid` | Ran `go test -p=1 -count=1 -timeout=20m ./pumps -run '^TestElasticsearchPump_WriteData_MCPIndexRouting_NonBulkBug$'`; inspected non-bulk `processData` builder using the default index instead of per-record `recordIndex`. | Stamped review date `2026-11-30`; no product fix made. |
| `elasticsearch-unbounded-reconnect-recursion` | KnownIssue | `valid` | Inspected `pumps/elasticsearch.go:connect` recursive retry and `WriteData` nil-operator reconnect recursion; evidence is the OpenGrep signal `go.recursion.unbounded-on-error`. | Stamped review date `2026-11-30`; no product fix made. |
| `elasticsearch-writedata-errors-swallowed` | KnownIssue | `valid` | Ran `go test -p=1 -count=1 -timeout=20m ./pumps -run '^TestElasticsearchPump_WriteData_V7ProcessDataIndexError$'`; inspected `WriteData` ignoring `operator.processData` errors. | Stamped review date `2026-11-30`; no product fix made. |
| `env-prefix-const-typo` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static source check: `config.go` declares and uses `ENV_PREVIX`; string value remains correct (`TYK_PMP`), so this is naming/docs debt rather than runtime failure. | Stamped review date `2026-11-30`; keep disposition `ship`. |
| `es-legacy-versions-need-deprecated-containers` | KnownIssue | `valid` | Inspected `pumps/elasticsearch.go` v3/v5/v6 branches and `pumps/elasticsearch_mcdc_100_test.go` legacy operator branch tests; issue is environment/capability debt for true legacy ES wire coverage. | Already had review date `2026-11-30`; no change needed. |

## Accepted Risk Slice

Read-only subagent audit completed for all 13 accepted risks.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `elasticsearch-unbounded-reconnect-stack-overflow` | AcceptedRisk | `valid` | Matches KI `elasticsearch-unbounded-reconnect-recursion`, reviewed through `2026-11-30`. | No change needed. |
| `graylog-moesif-record-fatal` | AcceptedRisk | `needs_split` | Risk accepts per-record fatal paths, but backing KI `graylog-moesif-logfatal-on-record-error` also covers Moesif parse/config fatal behavior. | Backlog: split the risk or broaden affected requirements, impact, and mitigation. |
| `mongo-pump-context-timeout-bypass` | AcceptedRisk | `needs_metadata_hardening` -> partially hardened | Backing KI `mongo-pump-ignores-caller-context` affects both `SYS-REQ-005` and `INT-REQ-004`; risk only listed `SYS-REQ-005`. | Added `INT-REQ-004` to accepted risk. |
| `no-dlq-on-pump-write-failure` | AcceptedRisk | `valid` | Matches KI `write-failure-after-pop-loses-records` and catalog class `dead_letter_or_reenqueue_on_total_write_failure`. | No change needed. |
| `per-pump-timeout-not-enforced-when-zero` | AcceptedRisk | `needs_reproducer_hardening` | Backing KI `pump-no-timeout-can-block-purge-cycle` currently points at an Elasticsearch HTTP timeout signal, not the `main.go` zero-timeout branch. | Backlog: add focused evidence for `GetTimeout()==0` using `context.WithCancel`. |
| `pump-config-typos-silently-ignored` | AcceptedRisk | `valid` | Matches KI `mapstructure-decode-silently-drops-unknown-keys` and catalog class `config_schema_strict`. | No change needed. |
| `pump-panic-crashes-process` | AcceptedRisk | `needs_merge` | KIs `no-panic-recovery-in-exec-pump-writing` and `pump-fanout-panic-not-recovered` describe the same accepted panic surface. | Backlog: canonicalize duplicate KIs and keep the stronger verification reference. |
| `pump-writedata-swallows-per-batch-errors` | AcceptedRisk | `valid` | Matches KI `pump-writedata-swallows-per-batch-errors`, reviewed through `2026-11-30`. | No change needed. |
| `serializer-protobuf-city-names-data-loss` | AcceptedRisk | `valid` | Matches KI `serializer-protobuf-loses-city-names`. | No change needed. |
| `sql-default-migration-today-only-data-loss` | AcceptedRisk | `valid` | Matches KI `sql-default-migration-today-only`. | No change needed. |
| `statsd-setup-fatal-aborts-pump` | AcceptedRisk | `valid` | Matches KI `logfatal-on-statsd-setup` and catalog class `process_exit_on_recoverable`. | No change needed. |
| `storage-pop-expire-non-atomic-data-loss` | AcceptedRisk | `needs_metadata_hardening` -> partially hardened | Backing KI `getanddeleteset-expire-fail-loses-records` includes `STK-REQ-002`; accepted risk only listed `INT-REQ-005`. | Added `STK-REQ-002` to accepted risk. |
| `uptime-pump-silent-failure` | AcceptedRisk | `valid` | Matches KI `uptime-write-has-no-error-return`. | No change needed. |

## KnownIssue Slice: Aggregate Through SQLite Docs

Read-only subagent audit completed for 18 KnownIssues from
`aggregate-counter-replay-not-idempotent` through
`docs-sqlite-still-listed-as-supported-sql-type`.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `aggregate-counter-replay-not-idempotent` | KnownIssue | `needs_reproducer_hardening` | Static source shows additive counters and Mongo `$inc`, but does not exercise duplicate replay. | Backlog: add duplicate-record tripwire for aggregate and/or Mongo aggregate path; add review date after reproducer is real. |
| `aggregate-mcdc-unreachable-string-assertion-ok` | KnownIssue | `stale_or_resolved` | Current `analytics/aggregate.go` contains the expected `//mcdc:ignore` comments. | Backlog: close/convert according to proof semantics and remove stale active-KI refs from `DEFECT-39` if required. |
| `analytics-key-shard-ceiling-hardcoded` | KnownIssue | `needs_metadata_hardening` | Active code fact at `main.go:267`; release disposition needs clearer assumption boundary under `SYS-REQ-030`. | Backlog: add review date and clarify disposition. |
| `analytics-timestamp-timezone-convention-unpinned` | KnownIssue | `needs_split` | Record combines timestamp convention, shard routing, aggregate/uptime serialization, and DST formatting paths. | Backlog: split or structure into narrower KIs with at least one SQL shard-boundary and one proto/location reproducer. |
| `analytics-trimstring-byte-truncation-not-unicode-safe` | KnownIssue | `needs_reproducer_hardening` | Active gap at `analytics/analytics.go:333`; static grep does not prove invalid UTF-8 output. | Backlog: add unit tripwire that cuts a multibyte rune and asserts current invalid output; add review date. |
| `common-setloglevel-nil-log-panic` | KnownIssue | `needs_reproducer_hardening` | Latent panic at `pumps/common.go:80`; static grep does not execute it. | Backlog: add `require.Panics` reproducer on zero-value `CommonPumpConfig.SetLogLevel`; add review date. |
| `config-pump-type-whitelist-not-enforced-at-load` | KnownIssue | `needs_reproducer_hardening` | Loader/init boundary gap across `config.go:262` and `main.go:201`. | Backlog: add test proving `LoadConfig` accepts an unknown pump type and only init skips it; add review date. |
| `csv-writedata-nil-file-handle-panic` | KnownIssue | `needs_reproducer_hardening` | Existing recover-based witnesses do not fail if the panic disappears. | Backlog: tighten witness to assert the panic explicitly; add review date. |
| `docs-elasticsearch-mcp-index-name-undocumented` | KnownIssue | `needs_reproducer_hardening` | Code-side field evidence exists, but docs-side absence is not directly asserted. | Backlog: add docs negative grep for `mcp_index_name`. |
| `docs-elasticsearch-version-range-incorrect` | KnownIssue | `valid` | Local docs still say `Elasticsearch (2.0 - 7.x)` while code supports 3/5/6/7 branches. | No change needed. |
| `docs-health-endpoint-liveness-vs-readiness-unclear` | KnownIssue | `needs_reproducer_hardening` | Reproducer prints misleading docs but does not pair with source evidence that `/health` is liveness-only. | Backlog: add source/test evidence from server health handler. |
| `docs-hybrid-enable-mcp-aggregation-undocumented` | KnownIssue | `needs_reproducer_hardening` | Code-side field evidence exists, but docs-side absence is not directly asserted. | Backlog: add docs negative grep for `enable_mcp_aggregation`. |
| `docs-kinesis-env-streamname-typo` | KnownIssue | `valid` | README still has the typo while local docs have the correct spelling. | No change needed. |
| `docs-mcp-pump-family-entirely-undocumented` | KnownIssue | `needs_metadata_hardening` -> hardened | Description covers `SW-REQ-038`, `SW-REQ-039`, `SW-REQ-044`, and `SW-REQ-045`, but record only listed `SW-REQ-044`. | Added missing affected requirements `SW-REQ-038`, `SW-REQ-039`, and `SW-REQ-045`. |
| `docs-missing-per-pump-config-sections` | KnownIssue | `needs_reproducer_hardening` | Reproducer lists registered pumps but does not prove which docs sections are missing. | Backlog: compare registered pump names to `pump-config.md` headings. |
| `docs-splunk-mutates-default-transport-undocumented` | KnownIssue | `needs_reproducer_hardening` | Source mutates `http.DefaultClient.Transport`, but docs absence is not directly asserted. | Backlog: add docs negative grep around Splunk section. |
| `docs-sql-migrate-sharded-tables-undocumented` | KnownIssue | `needs_reproducer_hardening` | Code-side field evidence exists, but docs-side absence is not directly asserted. | Backlog: add docs negative grep for `migrate_sharded_tables`. |
| `docs-sqlite-still-listed-as-supported-sql-type` | KnownIssue | `needs_metadata_hardening` -> hardened | Docs still list sqlite while `pumps/sql.go` rejects it; `git log -S sqlite -- pumps/sql.go` identifies removal commit. | Changed `introduced_in` from `inception` to `775f8e3` (`[TT-13341] Remove SQLite support from Tyk Pump`). |

## KnownIssue Slice: External Endpoints Through Kinesis

Read-only subagent audit completed for 29 KnownIssues from
`external-pump-endpoints-no-ssrf-allowlist` through
`kinesis-splitintobatches-zero-infinite-loop`.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `external-pump-endpoints-no-ssrf-allowlist` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static reproducer cites outbound endpoint fields across Hybrid, Influx, Splunk, Logz.io, and Elasticsearch. | Stamped review date `2026-11-30`; backlog: cite exact endpoint fields per pump in description. |
| `filterdata-base64-decode-silent-noop` | KnownIssue | `needs_reproducer_hardening` -> partially hardened | Focused test `TestFilterDataBase64DecodeFailurePreservesField` exists and was run by subagent. | Added canonical `// Reproduces: filterdata-base64-decode-silent-noop` and test evidence pointer. |
| `getanddeleteset-expire-fail-loses-records` | KnownIssue | `needs_reproducer_hardening` -> partially hardened | Existing test `TestTemporalStorageHandler_GetAndDeleteSet_ExpireFailureDecision` exercises successful pop followed by `Expire` failure. | Added canonical `// Reproduces: getanddeleteset-expire-fail-loses-records`; stamped review date `2026-11-30`. |
| `getanddeleteset-expire-ttl-assumes-clock-sync` | KnownIssue | `stale_or_resolved` | Current code uses relative Redis `EXPIRE`; `storage/clock_skew_tolerated_test.go` proves clock-skew tolerance for `SW-REQ-006`. | Backlog: close as stale or rewrite narrowly as `SW-REQ-007` evidence-gap record. |
| `graph-mcp-sql-sharded-indexes-untracked` | KnownIssue | `valid` | Source still shows Graph/MCP sharded paths lack explicit per-shard index contract/tests. | No change needed. |
| `graph-sql-aggregate-atomicity-fault-injection-missing` | KnownIssue | `needs_reproducer_hardening` | Atomicity gap lacks transaction/failure harness. | Backlog: add fake-driver or DB-backed failure-injection tests for `tx.Error`, deadlock/serialization failure, and caller-visible error propagation; add review date. |
| `graph-sql-aggregate-migrate-sharded-tables-ignored` | KnownIssue | `valid` | `GraphSQLAggregatePump.Init` still skips `HandleTableMigration` when sharded. | No change needed. |
| `graylog-moesif-logfatal-on-record-error` | KnownIssue | `needs_split` | Record mixes Graylog per-record fatal, Moesif per-record fatal, Moesif config-refresh fatal, and structurally unreachable marshal branches. | Backlog: split runtime crash surfaces and track unreachable marshal branches separately as MC/DC debt if needed. |
| `graylog-nil-client-recursive-writedata-duplicates-data` | KnownIssue | `needs_reproducer_hardening` | Static recursive fall-through claim is plausible but not directly exercised. | Backlog: add fake-client or hook-based test proving double-send after recursive recovery; add review date. |
| `health-endpoint-auth-not-enforced` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static source shows health route registers without auth middleware. | Stamped review date `2026-11-30`; backlog: clarify whether unauthenticated liveness is accepted posture or must be auth-gated. |
| `health-endpoint-is-liveness-only` | KnownIssue | `valid` | `/health` still always returns 200 and is separate from auth/rate-limit concerns. | No change needed. |
| `health-endpoint-rate-limit-not-enforced` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static source shows health route has no throttling/rate-limit wrapper. | Stamped review date `2026-11-30`. |
| `health-listener-bind-failure-logfatal` | KnownIssue | `needs_reproducer_hardening` | Current process-exit ratchet does not visibly bind to this KI. | Backlog: add subprocess or AST ratchet evidence for `server/server.go` listener `Fatal`. |
| `http-retry-post-idempotency-unenforced` | KnownIssue | `needs_reproducer_hardening` -> partially hardened | Existing test proves POST request body replay on retry. | Added canonical `// Reproduces: http-retry-post-idempotency-unenforced` and test evidence pointer; backlog: add duplicate-acceptance scenario where receiver commits but response fails. |
| `http-sdk-redirect-policy-implicit` | KnownIssue | `needs_metadata_hardening` -> reviewed | Pinned SDK source still confirms default HTTP redirect behavior. | Stamped review date `2026-11-30`. |
| `hybrid-getdialfn-leaks-conn-on-handshake-fail` | KnownIssue | `needs_reproducer_hardening` | Leak claim needs a fake `net.Conn`/dial reproducer. | Backlog: add connection-closing assertion via fake conn or hook; add review date. |
| `hybrid-rpc-retry-duration-not-deadline-bounded` | KnownIssue | `valid` | Retry count is bounded; elapsed deadline is not specified/tested. | No change needed. |
| `influx-v1-unbounded-reconnect-recursion` | KnownIssue | `valid` | `connect()` still recursively calls itself after sleep on client creation error. | No change needed. |
| `instrumentation-goroutines-no-recover-or-shutdown` | KnownIssue | `needs_split` | Record mixes panic-recovery debt and GC-monitor lifecycle debt; StatsD has Stop/Drain lifecycle. | Backlog: split panic recovery from shutdown/lifetime debt and narrow wording. |
| `instrumentation-statsd-channel-backpressure-blocks-emitters` | KnownIssue | `valid` | Synchronous sends to fixed `cmdChan` remain. | No change needed. |
| `kafka-logfatal-on-init-mech-and-timeout` | KnownIssue | `valid` | Subprocess tests exist and code still fatals on SCRAM/timeout parse failures. | No change needed. |
| `kafka-writedata-full-batch-allocation-unbounded` | KnownIssue | `valid` | Full `[]kafka.Message` allocation remains before backend write. | No change needed. |
| `kafka-writedata-marshal-err-structurally-unreachable` | KnownIssue | `valid` | Defensive `json.Marshal` error branch remains MC/DC/defensive-code debt. | No change needed. |
| `kafka-writedata-non-analytics-record-panic` | KnownIssue | `needs_metadata_hardening` -> reviewed | Bound reproducer exists and source still panics on unchecked assertion. | Stamped review date `2026-11-30`. |
| `kafka-writedata-swallows-write-errors` | KnownIssue | `needs_reproducer_hardening` | Needs direct fake-writer evidence that broker/cancel errors are logged and `WriteData` returns nil. | Backlog: add injectable writer/fake-writer tripwire. |
| `kinesis-batch-size-over-aws-putrecords-limit` | KnownIssue | `valid` | Focused helper test passed and helper still allows a 501-record batch. | No change needed. |
| `kinesis-putrecords-per-record-failures-return-nil` | KnownIssue | `needs_reproducer_hardening` | Needs mockable Kinesis client evidence for `FailedRecordCount` / per-record `ErrorCode` returning nil. | Backlog: add client seam or helper-level tripwire. |
| `kinesis-random-partition-key-not-idempotent` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static source evidence is accurate: random partition key is generated per record. | Stamped review date `2026-11-30`; backlog: consider deterministic test when randomness is injectable. |
| `kinesis-splitintobatches-zero-infinite-loop` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static evidence is accurate; live infinite-loop test is unsafe without a subprocess/timeout harness. | Stamped review date `2026-11-30`. |

## Verification Commands

- `go test -p=1 -count=1 -timeout=20m ./pumps -run '^(TestElasticsearchPump_ApiKeyAuthDroppedWhenUseSSL_KI|TestGetMapping_DecodeBase64MalformedInput_KI|TestElasticsearchPump_WriteData_MCPIndexRouting_NonBulkBug|TestElasticsearchPump_WriteData_V7ProcessDataIndexError)$'`
