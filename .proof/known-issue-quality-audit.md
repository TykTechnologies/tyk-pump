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
- Raw requirement `ki_ref` deferral rows: 416 across 78 requirement files,
  covering 111 unique KnownIssue IDs
- Go `mcdc:ignore` rows: 291 across 66 Go files; 216 rows carry an explicit
  KnownIssue marker or name, covering 20 unique KnownIssue IDs
- Deferred acceptance-criteria debt rows: 1

Subagent inventory cross-check: all 13 accepted risks, all 111 unique
requirement `ki_ref` KnownIssue IDs, and all 20 unique MC/DC KnownIssue IDs are
represented in the per-item audit rows below.

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
| `elapsed-timesince-dst-obligation-misattributed` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static reproducer `rg -n "time\\.Since\\(" main.go pumps/kafka.go pumps/sqs.go retry/http-retry.go`; source sites are elapsed-duration metrics/logging, not date-shard business arithmetic. | Refreshed `.proof/evidence/ki-elapsed-timesince-dst-obligation-misattributed.yaml`; keep open as proof/spec attribution debt, not product defect. |
| `elasticsearch-api-key-auth-dropped-when-use-ssl` | KnownIssue | `valid` | Ran `go test -p=1 -count=1 -timeout=20m ./pumps -run '^TestElasticsearchPump_ApiKeyAuthDroppedWhenUseSSL_KI$'`; inspected `pumps/elasticsearch.go:getOperator`, where API-key transport is assigned before the `UseSSL` branch replaces `httpClient` with a TLS-only transport. | Stamped review date `2026-11-30`; refreshed `.proof/evidence/ki-elasticsearch-api-key-auth-dropped-when-use-ssl.yaml`; no product fix made. |
| `elasticsearch-decode-base64-errors-silent-empty` | KnownIssue | `valid` | Ran `go test -p=1 -count=1 -timeout=20m ./pumps -run '^TestGetMapping_DecodeBase64MalformedInput_KI$'`; inspected `getMapping` malformed base64 behavior. | Refreshed `.proof/evidence/ki-elasticsearch-decode-base64-errors-silent-empty.yaml`; already reviewed through `2026-11-30`. |
| `elasticsearch-mcp-routing-non-bulk-ignored` | KnownIssue | `valid` | Ran `go test -p=1 -count=1 -timeout=20m ./pumps -run '^TestElasticsearchPump_WriteData_MCPIndexRouting_NonBulkBug$'`; inspected non-bulk `processData` builder using the default index instead of per-record `recordIndex`. | Stamped review date `2026-11-30`; refreshed `.proof/evidence/ki-elasticsearch-mcp-routing-non-bulk-ignored.yaml`; no product fix made. |
| `elasticsearch-unbounded-reconnect-recursion` | KnownIssue | `valid` | Inspected `pumps/elasticsearch.go:connect` recursive retry and `WriteData` nil-operator reconnect recursion; evidence is the OpenGrep signal `go.recursion.unbounded-on-error`. | Attached and refreshed `.proof/evidence/ki-elasticsearch-unbounded-reconnect-recursion.yaml`; no product fix made. |
| `elasticsearch-writedata-errors-swallowed` | KnownIssue | `valid` | Ran `go test -p=1 -count=1 -timeout=20m ./pumps -run '^TestElasticsearchPump_WriteData_V7ProcessDataIndexError$'`; inspected `WriteData` ignoring `operator.processData` errors. | Stamped review date `2026-11-30`; refreshed `.proof/evidence/ki-elasticsearch-writedata-errors-swallowed.yaml`; no product fix made. |
| `env-prefix-const-typo` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static source check: `config.go` declares and uses `ENV_PREVIX`; string value remains correct (`TYK_PMP`), so this is naming/docs debt rather than runtime failure. | Stamped review date `2026-11-30`; keep disposition `ship`. |
| `es-legacy-versions-need-deprecated-containers` | KnownIssue | `valid` | Inspected `pumps/elasticsearch.go` v3/v5/v6 branches and `pumps/elasticsearch_mcdc_100_test.go` legacy operator branch tests; issue is environment/capability debt for true legacy ES wire coverage. | Already had review date `2026-11-30`; no change needed. |

## Accepted Risk Slice

Read-only subagent audit completed for all 13 accepted risks.
Follow-up sidecar audit found that the current AcceptedRisk schema does not
define structured `kind`, `linked_known_issues`, or `evidence_manifests`
fields. Those links remain prose/backing-KI mediated until proof supports them
as first-class risk fields.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `elasticsearch-unbounded-reconnect-stack-overflow` | AcceptedRisk | `valid` | Matches KI `elasticsearch-unbounded-reconnect-recursion`, reviewed through `2026-11-30`. | No change needed. |
| `graylog-moesif-record-fatal` | AcceptedRisk | `needs_metadata_hardening` -> broadened | Risk originally accepted per-record fatal paths, while backing KI `graylog-moesif-logfatal-on-record-error` also covers Moesif parse/config fatal behavior. | Broadened title, description, mitigation, customer impact, and affected requirements to include `SW-REQ-049` and `SW-REQ-052`. |
| `mongo-pump-context-timeout-bypass` | AcceptedRisk | `needs_metadata_hardening` -> partially hardened | Backing KI `mongo-pump-ignores-caller-context` affects both `SYS-REQ-005` and `INT-REQ-004`; risk only listed `SYS-REQ-005`. | Added `INT-REQ-004` to accepted risk. |
| `no-dlq-on-pump-write-failure` | AcceptedRisk | `valid` | Matches KI `write-failure-after-pop-loses-records` and catalog class `dead_letter_or_reenqueue_on_total_write_failure`. | No change needed. |
| `per-pump-timeout-not-enforced-when-zero` | AcceptedRisk | `needs_reproducer_hardening` -> hardened | Backing KI `pump-no-timeout-can-block-purge-cycle` pointed at an Elasticsearch HTTP timeout signal rather than the `main.go` zero-timeout branch. | Added and refreshed `.proof/evidence/ki-pump-no-timeout-can-block-purge-cycle.yaml`, linked it to the backing KI, and refreshed current `main.go` line anchors. |
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
| `aggregate-counter-replay-not-idempotent` | KnownIssue | `needs_reproducer_hardening` | Static source shows additive counters and Mongo `$inc`, but does not exercise duplicate replay. | Attached and refreshed `.proof/evidence/ki-aggregate-counter-replay-not-idempotent.yaml`; backlog remains to add duplicate-record tripwire for aggregate and/or Mongo aggregate path. |
| `aggregate-mcdc-unreachable-string-assertion-ok` | KnownIssue | `stale_or_resolved` | Current `analytics/aggregate.go` contains the expected `//mcdc:ignore` comments. | Backlog: close/convert according to proof semantics and remove stale active-KI refs from `DEFECT-39` if required. |
| `analytics-key-shard-ceiling-hardcoded` | KnownIssue | `needs_metadata_hardening` | Active code fact at `main.go:267`; release disposition needs clearer assumption boundary under `SYS-REQ-030`. | Attached and refreshed `.proof/evidence/ki-analytics-key-shard-ceiling-hardcoded.yaml`; backlog remains to clarify disposition. |
| `analytics-timestamp-timezone-convention-unpinned` | KnownIssue | `needs_split` | Record combines timestamp convention, shard routing, aggregate/uptime serialization, and DST formatting paths. | Attached and refreshed `.proof/evidence/ki-analytics-timestamp-timezone-convention-unpinned.yaml`; backlog remains to split or structure into narrower KIs with at least one SQL shard-boundary and one proto/location reproducer. |
| `analytics-trimstring-byte-truncation-not-unicode-safe` | KnownIssue | `needs_reproducer_hardening` | Active gap at `analytics/analytics.go:333`; static grep does not prove invalid UTF-8 output. | Attached and refreshed `.proof/evidence/ki-analytics-trimstring-byte-truncation-not-unicode-safe.yaml`; backlog remains to add unit tripwire that cuts a multibyte rune and asserts current invalid output. |
| `common-setloglevel-nil-log-panic` | KnownIssue | `needs_reproducer_hardening` | Latent panic at `pumps/common.go:80`; static grep does not execute it. | Attached and refreshed `.proof/evidence/ki-common-setloglevel-nil-log-panic.yaml`; backlog remains to add `require.Panics` reproducer on zero-value `CommonPumpConfig.SetLogLevel`. |
| `config-pump-type-whitelist-not-enforced-at-load` | KnownIssue | `needs_reproducer_hardening` | Loader/init boundary gap across `config.go:262` and `main.go:201`. | Attached and refreshed `.proof/evidence/ki-config-pump-type-whitelist-not-enforced-at-load.yaml`; backlog remains to add test proving `LoadConfig` accepts an unknown pump type and only init skips it. |
| `csv-writedata-nil-file-handle-panic` | KnownIssue | `needs_metadata_hardening` -> hardened | Existing focused tests cover create/open failure behavior and now have a live proof evidence manifest. | Attached and refreshed `.proof/evidence/ki-csv-writedata-nil-file-handle-panic.yaml`; backlog remains only if we want a stricter panic-specific witness. |
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
| `external-pump-endpoints-no-ssrf-allowlist` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static reproducer cites outbound endpoint fields across Hybrid, Influx, Splunk, Logz.io, and Elasticsearch. | Refreshed `.proof/evidence/ki-external-pump-endpoints-no-ssrf-allowlist.yaml`; backlog: cite exact endpoint fields per pump in description. |
| `filterdata-base64-decode-silent-noop` | KnownIssue | `needs_reproducer_hardening` -> partially hardened | Focused test `TestFilterDataBase64DecodeFailurePreservesField` exists and was run by subagent. | Added canonical `// Reproduces: filterdata-base64-decode-silent-noop`, test evidence pointer, and refreshed `.proof/evidence/ki-filterdata-base64-decode-silent-noop.yaml`. |
| `getanddeleteset-expire-fail-loses-records` | KnownIssue | `needs_reproducer_hardening` -> partially hardened | Existing test `TestTemporalStorageHandler_GetAndDeleteSet_ExpireFailureDecision` exercises successful pop followed by `Expire` failure. | Added canonical `// Reproduces: getanddeleteset-expire-fail-loses-records`; stamped review date `2026-11-30`; refreshed `.proof/evidence/ki-getanddeleteset-expire-fail-loses-records.yaml`. |
| `getanddeleteset-expire-ttl-assumes-clock-sync` | KnownIssue | `stale_or_resolved` | Current code uses relative Redis `EXPIRE`; `storage/clock_skew_tolerated_test.go` proves clock-skew tolerance for `SW-REQ-006`. | Backlog: close as stale or rewrite narrowly as `SW-REQ-007` evidence-gap record. |
| `graph-mcp-sql-sharded-indexes-untracked` | KnownIssue | `valid` | Source still shows Graph/MCP sharded paths lack explicit per-shard index contract/tests. | Refreshed `.proof/evidence/ki-graph-mcp-sql-sharded-indexes-untracked.yaml`. |
| `graph-sql-aggregate-atomicity-fault-injection-missing` | KnownIssue | `needs_reproducer_hardening` | Atomicity gap lacks transaction/failure harness. | Refreshed static `.proof/evidence/ki-graph-sql-aggregate-atomicity-fault-injection-missing.yaml`; backlog remains to add fake-driver or DB-backed failure-injection tests for `tx.Error`, deadlock/serialization failure, and caller-visible error propagation. |
| `graph-sql-aggregate-migrate-sharded-tables-ignored` | KnownIssue | `valid` | `GraphSQLAggregatePump.Init` still skips `HandleTableMigration` when sharded. | Refreshed `.proof/evidence/ki-graph-sql-aggregate-migrate-sharded-tables-ignored.yaml`. |
| `graylog-moesif-logfatal-on-record-error` | KnownIssue | `needs_split` | Record mixes Graylog per-record fatal, Moesif per-record fatal, Moesif config-refresh fatal, and structurally unreachable marshal branches. | Attached and refreshed `.proof/evidence/ki-graylog-moesif-logfatal-on-record-error.yaml` using existing subprocess fatal tests for the per-record subset; backlog remains to split Moesif config/read errors and unreachable marshal branches separately. |
| `graylog-nil-client-recursive-writedata-duplicates-data` | KnownIssue | `needs_reproducer_hardening` | Static recursive fall-through claim is plausible but not directly exercised. | Attached and refreshed `.proof/evidence/ki-graylog-nil-client-recursive-writedata-duplicates-data.yaml`; backlog remains to add fake-client or hook-based test proving double-send after recursive recovery. |
| `health-endpoint-auth-not-enforced` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static source shows health route registers without auth middleware. | Refreshed `.proof/evidence/ki-health-endpoint-auth-not-enforced.yaml`; backlog: clarify whether unauthenticated liveness is accepted posture or must be auth-gated. |
| `health-endpoint-is-liveness-only` | KnownIssue | `valid` | `/health` still always returns 200 and is separate from auth/rate-limit concerns. | Refreshed `.proof/evidence/ki-health-endpoint-is-liveness-only.yaml`. |
| `health-endpoint-rate-limit-not-enforced` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static source shows health route has no throttling/rate-limit wrapper. | Refreshed `.proof/evidence/ki-health-endpoint-rate-limit-not-enforced.yaml`. |
| `health-listener-bind-failure-logfatal` | KnownIssue | `needs_reproducer_hardening` | Current process-exit ratchet does not visibly bind to this KI. | Refreshed static `.proof/evidence/ki-health-listener-bind-failure-logfatal.yaml`; backlog remains to add subprocess or AST ratchet evidence for `server/server.go` listener `Fatal`. |
| `http-retry-post-idempotency-unenforced` | KnownIssue | `needs_reproducer_hardening` -> partially hardened | Existing test proves POST request body replay on retry. | Added canonical `// Reproduces: http-retry-post-idempotency-unenforced`, test evidence pointer, and refreshed `.proof/evidence/ki-http-retry-post-idempotency-unenforced.yaml`; backlog: add duplicate-acceptance scenario where receiver commits but response fails. |
| `http-sdk-redirect-policy-implicit` | KnownIssue | `needs_metadata_hardening` -> reviewed | Pinned SDK source still confirms default HTTP redirect behavior. | Attached and refreshed `.proof/evidence/ki-http-sdk-redirect-policy-implicit.yaml`; still static-only SDK source evidence. |
| `hybrid-getdialfn-leaks-conn-on-handshake-fail` | KnownIssue | `needs_reproducer_hardening` | Leak claim needs a fake `net.Conn`/dial reproducer. | Refreshed static `.proof/evidence/ki-hybrid-getdialfn-leaks-conn-on-handshake-fail.yaml`; backlog remains to add connection-closing assertion via fake conn or hook. |
| `hybrid-rpc-retry-duration-not-deadline-bounded` | KnownIssue | `valid` | Retry count is bounded; elapsed deadline is not specified/tested. | Refreshed `.proof/evidence/ki-hybrid-rpc-retry-duration-not-deadline-bounded.yaml`. |
| `influx-v1-unbounded-reconnect-recursion` | KnownIssue | `valid` | `connect()` still recursively calls itself after sleep on client creation error. | Attached and refreshed `.proof/evidence/ki-influx-v1-unbounded-reconnect-recursion.yaml`. |
| `instrumentation-goroutines-no-recover-or-shutdown` | KnownIssue | `needs_split` | Record mixes panic-recovery debt and GC-monitor lifecycle debt; StatsD has Stop/Drain lifecycle. | Attached and refreshed `.proof/evidence/ki-instrumentation-goroutines-no-recover-or-shutdown.yaml`; backlog remains to split panic recovery from shutdown/lifetime debt and narrow wording. |
| `instrumentation-statsd-channel-backpressure-blocks-emitters` | KnownIssue | `valid` | Synchronous sends to fixed `cmdChan` remain. | Attached and refreshed `.proof/evidence/ki-instrumentation-statsd-channel-backpressure-blocks-emitters.yaml`. |
| `kafka-logfatal-on-init-mech-and-timeout` | KnownIssue | `valid` | Subprocess tests exist and code still fatals on SCRAM/timeout parse failures. | No change needed. |
| `kafka-writedata-full-batch-allocation-unbounded` | KnownIssue | `valid` | Full `[]kafka.Message` allocation remains before backend write. | Attached and refreshed `.proof/evidence/ki-kafka-writedata-full-batch-allocation-unbounded.yaml`. |
| `kafka-writedata-marshal-err-structurally-unreachable` | KnownIssue | `valid` | Defensive `json.Marshal` error branch remains MC/DC/defensive-code debt. | No change needed. |
| `kafka-writedata-non-analytics-record-panic` | KnownIssue | `needs_metadata_hardening` -> reviewed | Bound reproducer exists and source still panics on unchecked assertion. | Stamped review date `2026-11-30`; tightened reproducer command and refreshed `.proof/evidence/ki-kafka-writedata-non-analytics-record-panic.yaml`. |
| `kafka-writedata-swallows-write-errors` | KnownIssue | `valid` | `TestKafkaPump_WriteData_WriteErrorPath` passed and proves writer errors are exercised without a returned error. | Tightened reproducer command and refreshed `.proof/evidence/ki-kafka-writedata-swallows-write-errors.yaml`. |
| `kinesis-batch-size-over-aws-putrecords-limit` | KnownIssue | `valid` | Focused helper test passed and helper still allows a 501-record batch. | Tightened reproducer command and refreshed `.proof/evidence/ki-kinesis-batch-size-over-aws-putrecords-limit.yaml`. |
| `kinesis-putrecords-per-record-failures-return-nil` | KnownIssue | `needs_reproducer_hardening` | Static source shows per-record failure logging and nil return, but a mockable Kinesis client witness is still missing. | Attached and refreshed `.proof/evidence/ki-kinesis-putrecords-per-record-failures-return-nil.yaml`; backlog remains to add client seam or helper-level tripwire. |
| `kinesis-random-partition-key-not-idempotent` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static source evidence is accurate: random partition key is generated per record. | Attached and refreshed `.proof/evidence/ki-kinesis-random-partition-key-not-idempotent.yaml`; backlog: consider deterministic test when randomness is injectable. |
| `kinesis-splitintobatches-zero-infinite-loop` | KnownIssue | `needs_metadata_hardening` -> reviewed | Static evidence is accurate; live infinite-loop test is unsafe without a subprocess/timeout harness. | Attached and refreshed `.proof/evidence/ki-kinesis-splitintobatches-zero-infinite-loop.yaml`. |

## KnownIssue Slice: StatsD Through Mongo

Local audit completed for 25 KnownIssues from `logfatal-on-statsd-setup`
through `mongo-standard-logbrowser-compatible-index-conflict`.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `logfatal-on-statsd-setup` | KnownIssue | `valid` | `instrumentation_helpers.go` still calls `log.Fatal` after `NewStatsDSink` failure; in-process execution is not practical because it exits the runner, so the AST ratchet is the correct tripwire. | Added `// Reproduces: logfatal-on-statsd-setup` to `TestProcessExitOnRecoverable_NoNewLogFatalSites`, linked the test evidence, stamped review date `2026-11-30`, and refreshed `.proof/evidence/ki-logfatal-on-statsd-setup.yaml`. |
| `logzio-segment-no-shutdown-flush` | KnownIssue | `valid` | Static tripwires prove Logz.io and Segment embed `CommonPumpConfig` and do not override `Shutdown`. | Added focused reproducer command and refreshed `.proof/evidence/ki-logzio-segment-no-shutdown-flush.yaml`; reviewed through `2026-11-30`. |
| `main-prefilter-debug-logs-unfiltered-records` | KnownIssue | `needs_reproducer_hardening` | Static source indicates debug logging can happen before `filterData`, but no log-capture tripwire proves sensitive ignored fields reach logs. | Attached and refreshed `.proof/evidence/ki-main-prefilter-debug-logs-unfiltered-records.yaml`; backlog remains to add a debug-log capture test. |
| `main-shutdown-pumps-serial-no-timeout` | KnownIssue | `needs_split` | Record combines serial shutdown/no per-pump timeout with an `execPumpWriting` goroutine leak after context timeout. | Attached and refreshed `.proof/evidence/ki-main-shutdown-pumps-serial-no-timeout.yaml`; backlog remains to split shutdown sequencing and write goroutine lifetime debt. |
| `main-startup-logfatal-on-transient-backend` | KnownIssue | `needs_reproducer_hardening` | Description correctly identifies transient storage startup `log.Fatal` sites, but current evidence is static fatal-site coverage rather than a focused startup-storage subprocess tripwire. | Attached and refreshed `.proof/evidence/ki-main-startup-logfatal-on-transient-backend.yaml`; backlog remains to add direct startup fatal-site subprocess evidence. |
| `mapstructure-decode-silently-drops-unknown-keys` | KnownIssue | `valid` | Cross-pump OpenGrep evidence identifies lenient `mapstructure.Decode` calls; accepted risk already covers the config typo posture. | Attached and refreshed `.proof/evidence/ki-mapstructure-decode-silently-drops-unknown-keys.yaml`; reproducer now scans production files for lenient `mapstructure.Decode` and absence of strict `ErrorUnused`/`DecoderConfig`. |
| `mcdc-pumps-below-95` | KnownIssue | `valid` | Current full audit reports code-level MC/DC meets configured thresholds, with one accepted KI-debt row; this record remains proof-coverage context rather than product debt. | No change needed; already reviewed through `2026-11-30`. |
| `mcp-mongo-aggregate-atomicity-fault-injection-missing` | KnownIssue | `valid` | This is explicitly a missing fault-injection harness record; static source/requirement check supports the proof-harness gap. | Stamped review date `2026-11-30`. |
| `mcp-sql-aggregate-background-index-concurrency-unbounded` | KnownIssue | `needs_reproducer_hardening` | Source still has background index signaling without an explicit lifecycle/cancellation contract, but no lifecycle/failure witness exists. | Attached and refreshed `.proof/evidence/ki-mcp-sql-aggregate-background-index-concurrency-unbounded.yaml`; backlog remains to add fake-DB or race/lifecycle witness. |
| `mcp-sql-aggregate-mysql-create-index-syntax-broken` | KnownIssue | `needs_reproducer_hardening` | Static SQL construction shows `CREATE INDEX IF NOT EXISTS`; a MySQL dialect witness would better prove the actual backend rejection. | Stamped review date `2026-11-30`; backlog: add MySQL syntax/rejection tripwire. |
| `metrics-label-cardinality-unbounded` | KnownIssue | `valid` | Manual code-signal evidence covers StatsD, DogStatsD, and Prometheus high-cardinality label/tag surfaces. | Attached and refreshed `.proof/evidence/ki-metrics-label-cardinality-unbounded.yaml`. |
| `moesif-config-response-readall-unbounded` | KnownIssue | `valid` | `pumps/moesif.go` still reads the full configuration response with `ioutil.ReadAll` before parsing. | Tightened reproducer command and refreshed `.proof/evidence/ki-moesif-config-response-readall-unbounded.yaml`. |
| `moesif-queueevent-error-swallowed` | KnownIssue | `valid` | `TestMoesifPump_WriteData_QueueEventError_KI` proves SDK enqueue failure is logged while `WriteData` returns nil. | Added test evidence pointer, stamped review date `2026-11-30`, and refreshed `.proof/evidence/ki-moesif-queueevent-error-swallowed.yaml`. |
| `moesif-raw-body-unbounded-json-mask` | KnownIssue | `needs_reproducer_hardening` | Source shows unbounded `json.Unmarshal` and recursive masking, but there is no oversized/deeply nested body tripwire. | Attached and refreshed `.proof/evidence/ki-moesif-raw-body-unbounded-json-mask.yaml`; backlog remains to add bounded-size/depth failure test. |
| `moesif-sampling-randomness-obligation-misattributed` | KnownIssue | `needs_metadata_hardening` -> hardened | Static source shows intentional probabilistic sampling; subagent review found stale wording because `SW-REQ-052` now explicitly specifies sampling. | Rewrote KI and `SW-REQ-052` deferral to state the remaining debt is randomness-signal attribution to `determinism`, not missing sampling semantics; attached and refreshed `.proof/evidence/ki-moesif-sampling-randomness-obligation-misattributed.yaml`. |
| `mongo-aggregate-atomicity-fault-injection-missing` | KnownIssue | `valid` | This is explicitly a missing fault-injection harness record for Mongo aggregate update sequencing. | Stamped review date `2026-11-30`. |
| `mongo-aggregate-last-document-query-ignores-timeout` | KnownIssue | `valid` | Source still calls `m.store.Query(context.Background(), ...)` in `getLastDocumentTimestamp`. | Attached and refreshed `.proof/evidence/ki-mongo-aggregate-last-document-query-ignores-timeout.yaml`. |
| `mongo-cap-collection-non64-arch-unreachable` | KnownIssue | `valid` | 32-bit guard remains structurally unreachable on the supported 64-bit release matrix. | No change needed; already reviewed through `2026-11-30`. |
| `mongo-pump-ignores-caller-context` | KnownIssue | `valid` | `TestMongoPump_WriteData_IgnoresCallerCtx_KI` and code-signal evidence show canceled caller context is ignored in the standard Mongo write path; sibling Mongo-family paths remain covered by current context-drop signals. | Added memory-guarded reproducer command, corrected current source anchors, documented the container-backed reproducer, and refreshed `.proof/evidence/ki-mongo-pump-ignores-caller-context.yaml`. |
| `mongo-pump-init-connect-logfatal-unreachable` | KnownIssue | `valid` | Mongo connection/config `log.Fatal` branches remain structural process-exit gaps tied to MC/DC ignores. | No change needed; already reviewed through `2026-11-30`. |
| `mongo-pump-writeuptime-nil-on-bad-msgpack` | KnownIssue | `valid` | Existing Mongo and MongoSelective bad-msgpack KI tests assert the current panic behavior. | Tightened reproducer command to cover both tests, changed release disposition to `ship_with_known_issue`, documented the container-backed reproducer, refreshed source anchors, and refreshed `.proof/evidence/ki-mongo-pump-writeuptime-nil-on-bad-msgpack.yaml`. |
| `mongo-selective-final-skipped-record-drops-pending-batch` | KnownIssue | `valid` | Existing tripwire proves a final skipped MongoSelective record can drop a pending valid batch; provenance points to `83a16f3`. | Added refreshed manifest `.proof/evidence/ki-mongo-selective-final-skipped-record-drops-pending-batch.yaml` and tightened reproducer command. |
| `mongo-standard-final-skipped-record-drops-pending-batch` | KnownIssue | `valid` | Existing tripwire proves standard Mongo can drop a pending valid batch when the final record is skipped; provenance points to `83a16f3`. | Added refreshed manifest `.proof/evidence/ki-mongo-standard-final-skipped-record-drops-pending-batch.yaml` and tightened reproducer command. |
| `mongo-standard-insert-error-double-send-goroutine-leak` | KnownIssue | `valid` | Existing AST tripwire checks the errCh double-send/early-return structure; provenance points to `14ccaba`. | Added refreshed manifest `.proof/evidence/ki-mongo-standard-insert-error-double-send-goroutine-leak.yaml` and tightened reproducer command. |
| `mongo-standard-logbrowser-compatible-index-conflict` | KnownIssue | `valid` | Existing fake-store tripwire proves compatible log-browser index conflicts are returned on non-StandardMongo paths; provenance points to `8458e11`. | Added refreshed manifest `.proof/evidence/ki-mongo-standard-logbrowser-compatible-index-conflict.yaml` and tightened reproducer command. |

## KnownIssue Slice: Panic Through Resurface

Read-only subagent audit completed for 16 KnownIssues from
`no-panic-recovery-in-exec-pump-writing` through
`resurface-writedata-blocks-on-queue-full`.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `no-panic-recovery-in-exec-pump-writing` | KnownIssue | `needs_merge` | Duplicate of the `main.go` fanout panic surface; stronger subprocess evidence is on `pump-fanout-panic-not-recovered`. | Backlog: merge into the canonical fanout panic KI or move the subprocess evidence here. |
| `preprocess-decode-error-leaves-nil-hole-in-keys` | KnownIssue | `needs_reproducer_hardening` | Existing focused test only uses `Pumps=nil`, so it proves decode logging, not the nil slot reaching `filterData`. | Backlog: add a bad serialized record plus filtering/decoding pump test that reaches the nil-hole panic. |
| `prometheus-init-mutates-default-mux` | KnownIssue | `needs_metadata_hardening` | Source shows global default mux mutation, but linked obligation class should be lifecycle/global isolation, not request-timeout boundedness. | Attached and refreshed `.proof/evidence/ki-prometheus-init-mutates-default-mux.yaml`; backlog remains to correct obligation mapping and add subprocess or mux-swap evidence for second init/same path panic. |
| `prometheus-metric-maps-race` | KnownIssue | `needs_reproducer_hardening` | Static grep is accurate for unsynchronized metric maps, but no race witness exists. | Attached and refreshed `.proof/evidence/ki-prometheus-metric-maps-race.yaml`; backlog remains to add focused `go test -race` witness for concurrent `Inc`/`Observe`/`Expose`. |
| `prometheus-metrics-auth-not-enforced` | KnownIssue | `valid` | Static source and `SW-REQ-024 auth_required` deferral match the unauthenticated metrics endpoint. | Attached and refreshed `.proof/evidence/ki-prometheus-metrics-auth-not-enforced.yaml`. |
| `protobuf-decode-nil-submessage-panic` | KnownIssue | `needs_reproducer_hardening` | Requirement links are present, but there is no unit test asserting current panic on malformed payloads or missing submessages. | Attached and refreshed `.proof/evidence/ki-protobuf-decode-nil-submessage-panic.yaml`; backlog remains to add tests for non-byte input and payloads missing `Geo`, `Network`, or `Latency`. |
| `pump-fanout-no-global-concurrency-limit` | KnownIssue | `needs_reproducer_hardening` | Static `go execPumpWriting` evidence is accurate but does not quantify observed concurrency. | Attached and refreshed `.proof/evidence/ki-pump-fanout-no-global-concurrency-limit.yaml`; backlog remains to add counter-based many-pump test that records peak concurrent `WriteData` calls. |
| `pump-fanout-panic-not-recovered` | KnownIssue | `valid` | Subprocess reproducer exists and passed; this should be the canonical fanout panic KI. | Corrected root-level test paths, added focused reproducer command, refreshed `.proof/evidence/ki-pump-fanout-panic-not-recovered.yaml`; merge duplicate `no-panic-recovery-in-exec-pump-writing` into this record later. |
| `pump-no-per-pump-circuit-breaker` | KnownIssue | `valid` | Focused reproducer passed and proves a failing pump is invoked repeatedly without backoff. | Corrected root-level test paths, added focused reproducer command, and refreshed `.proof/evidence/ki-pump-no-per-pump-circuit-breaker.yaml`. |
| `pump-no-timeout-can-block-purge-cycle` | KnownIssue | `needs_reproducer_hardening` | Current evidence points at an Elasticsearch HTTP timeout signal, not the `GetTimeout()==0` branch in `main.go`. | Backlog: add blocking-pump test proving timeout zero uses `context.WithCancel` and blocks until released. |
| `pump-writedata-swallows-per-batch-errors` | KnownIssue | `needs_reproducer_hardening` | The record claims many pump implementations, but current executable evidence only covers the SQL member. | Added refreshed SQL-member manifest `.proof/evidence/ki-pump-writedata-swallows-per-batch-errors.yaml`; backlog remains to add per-pump witnesses or a table of static tripwires for every named implementation. |
| `pumps-logfatal-on-config-decode` | KnownIssue | `needs_split` | Record now mixes pure `mapstructure.Decode` fatals with non-decode setup fatals and a Mongo panic-call signal. | Backlog: split decode `log.Fatal` debt from non-decode init/setup fatal debt. |
| `resurface-disabled-writedata-closes-channel` | KnownIssue | `needs_metadata_hardening` | Source claim is plausible, but the KI lacks an explicit `SW-REQ-054` deferral and a concurrency/subprocess reproducer. | Backlog: add deferral and concurrent `Flush` plus `WriteData` send-on-closed-channel tripwire. |
| `resurface-maprawdata-empty-request-panic` | KnownIssue | `valid` | Direct panic reproducer exists and passed. | Tightened reproducer command and refreshed `.proof/evidence/ki-resurface-maprawdata-empty-request-panic.yaml`. |
| `resurface-worker-errors-swallowed` | KnownIssue | `needs_reproducer_hardening` | Source evidence is accurate, but no worker-path test proves async log/drop with nil `WriteData` return. | Attached and refreshed `.proof/evidence/ki-resurface-worker-errors-swallowed.yaml`; backlog remains to add worker-path malformed/non-analytics test. |
| `resurface-writedata-blocks-on-queue-full` | KnownIssue | `needs_reproducer_hardening` | Static select shape proves possibility, but not observed blocking behavior. | Attached and refreshed `.proof/evidence/ki-resurface-writedata-blocks-on-queue-full.yaml`; backlog remains to add bounded queue-full test that observes blocking, then cancels to release. |

## KnownIssue Slice: Retry Through SQL Batch

Read-only subagent audit completed for 20 KnownIssues from
`retry-4xx-bodyread-fail-causes-retry` through
`sql-batch-size-zero-infinite-loop`.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `retry-4xx-bodyread-fail-causes-retry` | KnownIssue | `needs_reproducer_hardening` | Real path exists in `retry/http-retry.go`, but evidence is static; `INT-REQ-006` ownership remains weak. | Refreshed `.proof/evidence/ki-retry-4xx-bodyread-fail-causes-retry.yaml`; backlog remains to add 4xx failing-body-reader test, assert no second attempt/permanent error, and re-check `INT-REQ-006`. |
| `retry-backoff-duration-not-deadline-bounded` | KnownIssue | `needs_metadata_hardening` | Valid spec/proof debt, but wording should separate library default elapsed behavior from declared product SLA. | Attached and refreshed `.proof/evidence/ki-retry-backoff-duration-not-deadline-bounded.yaml`; backlog remains to clarify no declared elapsed-time product bound exists. |
| `retry-buffers-full-request-body-in-memory` | KnownIssue | `valid` | Code and obligation mapping match `io.ReadAll(req.Body)` in retry middleware. | Tightened reproducer command and refreshed `.proof/evidence/ki-retry-buffers-full-request-body-in-memory.yaml`. |
| `retry-row5-mcdc-backoff-not-injectable` | KnownIssue | `needs_metadata_hardening` | Row-5 test exists, but metadata should classify this as proof-harness/testability debt. | Tightened reproducer command and refreshed `.proof/evidence/ki-retry-row5-mcdc-backoff-not-injectable.yaml`; backlog remains to clarify testability-debt wording. |
| `serializer-protobuf-loses-city-names` | KnownIssue | `needs_reproducer_hardening` | Real bug exists in protobuf geo city handling, but current evidence is static only. | Backlog: add `TestProtobuf_GeoCityNamesLoss_KI` with `// Reproduces`. |
| `serializer-protobuf-loses-graphql-error-path` | KnownIssue | `valid` | Focused KI test exists and passed; affected requirements and obligations align. | Added focused reproducer command and refreshed `.proof/evidence/ki-serializer-protobuf-loses-graphql-error-path.yaml`. |
| `setaggregatetimestamp-mcdc-unreachable-not-ok` | KnownIssue | `needs_metadata_hardening` | Structural MC/DC debt is real, but code now has the `//mcdc:ignore` and remediation still says to add it. | Backlog: update remediation/status to reflect current ignore and keep/close per KI-ignore policy. |
| `splunk-filtertags-skips-consecutive-matches` | KnownIssue | `needs_reproducer_hardening` | Existing FilterTags test uses non-consecutive matching tags and misses the slice-mutation bug. | Attached and refreshed `.proof/evidence/ki-splunk-filtertags-skips-consecutive-matches.yaml`; backlog remains to add KI test with consecutive dropped tags and assert current retained middle tag. |
| `splunk-newsplunkclient-mutates-default-transport` | KnownIssue | `needs_reproducer_hardening` | Code mutates `http.DefaultClient.Transport`, but the static witness is weaker than a deterministic snapshot/restore test. | Tightened reproducer command to a static global-transport mutation scan, changed release disposition to `ship_with_known_issue`, and refreshed `.proof/evidence/ki-splunk-newsplunkclient-mutates-default-transport.yaml`; backlog remains for a stronger behavioral test. |
| `splunk-writedata-non-analytics-record-panic` | KnownIssue | `needs_reproducer_hardening` | Unchecked assertion is real; current source assertion is at `pumps/splunk.go:187`, but the KI test can pass even if the panic disappears. | Refreshed source reference and `.proof/evidence/ki-splunk-writedata-non-analytics-record-panic.yaml`; backlog remains to change the tripwire to `require.Panics` while KI is open. |
| `sql-aggregate-atomicity-fault-injection-missing` | KnownIssue | `needs_reproducer_hardening` | Correct proof gap, but current reproducer is a grep for missing evidence. | Backlog: add fake-driver or DB-backed failure injection for create/transaction/deadlock paths. |
| `sql-aggregate-background-index-concurrency-unbounded` | KnownIssue | `needs_reproducer_hardening` | Lifecycle gap exists, but there is no bounded failure/cancellation witness. | Backlog: add non-container dry-run/fake-DB or race witness. |
| `sql-aggregate-mysql-create-index-if-not-exists-unsupported` | KnownIssue | `needs_reproducer_hardening` | Static DDL evidence is real, but no MySQL syntax/rejection witness exists. | Attached and refreshed `.proof/evidence/ki-sql-aggregate-mysql-create-index-if-not-exists-unsupported.yaml`; backlog remains to add SQL-generation/dry-run or focused MySQL witness. |
| `sql-aggregate-mysql-excluded-keyword-broken` | KnownIssue | `needs_reproducer_hardening` | Record spans SQL, Graph, and MCP aggregate paths and static evidence now covers all three aggregate paths. | Attached and refreshed `.proof/evidence/ki-sql-aggregate-mysql-excluded-keyword-broken.yaml`; backlog remains for backend rejection witness or split if fixes diverge. |
| `sql-aggregate-no-deadlock-retry` | KnownIssue | `needs_reproducer_hardening` | Static search proves no retry, but not deadlock/serialization behavior. | Attached and refreshed `.proof/evidence/ki-sql-aggregate-no-deadlock-retry.yaml`; backlog remains to add fake-driver deadlock/serialization error test proving no retry/current returned error. |
| `sql-aggregate-sharded-shared-db-race` | KnownIssue | `needs_reproducer_hardening` | Shared `c.db` mutation is real, but no race/concurrency witness exists. | Backlog: add focused `go test -race` sharded concurrent `WriteData` test. |
| `sql-aggregate-sharded-upsert-targets-base-table` | KnownIssue | `valid` | High-severity reviewed KI still matches current code and upstream fix note. | Attached and refreshed `.proof/evidence/ki-sql-aggregate-sharded-upsert-targets-base-table.yaml`; no product fix here, keep as KnownIssue. |
| `sql-aggregate-upsert-order-undocumented` | KnownIssue | `needs_metadata_hardening` | Valid documentation/proof gap, but product disposition still needs tightening. | Stamped review date `2026-11-30`; backlog: document unordered/id-keyed semantics in `SW-REQ-067`. |
| `sql-background-index-concurrency-unbounded` | KnownIssue | `needs_reproducer_hardening` | Standard SQL background-index issue is real, but current test skips the panic path. | Attached and refreshed `.proof/evidence/ki-sql-background-index-concurrency-unbounded.yaml`; backlog remains to add minimized non-container race/panic/lifecycle witness. |
| `sql-batch-size-zero-infinite-loop` | KnownIssue | `needs_split` | Init now defaults zero batch size, while negative/non-positive malformed input remains real; title conflates zero, negative, and six pump families. | Attached and refreshed `.proof/evidence/ki-sql-batch-size-zero-infinite-loop.yaml`; backlog remains to split or retire stale zero-config portion and keep a precise non-positive validation KI. |

## KnownIssue Slice: SQL Defaults Through Stdout

Local audit completed for 8 KnownIssues from `sql-default-migration-today-only`
through `stdout-writedata-swallows-ctx-error`.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `sql-default-migration-today-only` | KnownIssue | `valid` | `TestMCDC_KI_DefaultMigrationTodayOnly` directly proves default `MigrateShardedTables=false` leaves prior-day shard schema untouched. | Stamped review date `2026-11-30`; attached and refreshed `.proof/evidence/ki-sql-default-migration-today-only.yaml`. |
| `sql-ensureindex-name-mismatch-prevents-skip` | KnownIssue | `valid` | Static code path remains: `HasIndex` name lacks the underscore used by `createIndex`; duplicate create is guarded by DDL. | No change needed; already reviewed through `2026-11-30`. |
| `sql-schema-no-version-policy` | KnownIssue | `valid` | Static source check correctly shows AutoMigrate-only schema management and no schema-version manifest/table policy. | Attached and refreshed `.proof/evidence/ki-sql-schema-no-version-policy.yaml`. |
| `sql-standard-mysql-create-index-if-not-exists-unsupported` | KnownIssue | `needs_reproducer_hardening` | Static DDL construction is real, but there is no focused MySQL syntax/rejection witness. | Attached and refreshed `.proof/evidence/ki-sql-standard-mysql-create-index-if-not-exists-unsupported.yaml`; backlog remains a dialect witness. |
| `sqs-batch-partial-failures-ignored` | KnownIssue | `valid` | Added focused mock SQS test proving `SendMessageBatchOutput.Failed` is ignored when API-level error is nil. | Added evidence pointer, stamped review date `2026-11-30`, and refreshed `.proof/evidence/ki-sqs-batch-partial-failures-ignored.yaml`. |
| `sqs-batchlimit-zero-infinite-loop` | KnownIssue | `valid` | Executing the zero-limit path would hang; added bounded static tripwire for the loop shape. | Added evidence pointer, security-detail metadata, and refreshed `.proof/evidence/ki-sqs-batchlimit-zero-infinite-loop.yaml`; already reviewed through `2026-11-30`. |
| `sqs-malformed-record-sends-empty-entry` | KnownIssue | `valid` | Existing KI test proves a malformed input leaves a zero-value SQS entry in the sent batch. | Stamped review date `2026-11-30`; refreshed `.proof/evidence/ki-sqs-malformed-record-sends-empty-entry.yaml`. |
| `stdout-writedata-swallows-ctx-error` | KnownIssue | `valid` | Added focused KI test proving canceled context returns nil instead of `ctx.Err()`. | Added evidence pointer, stamped review date `2026-11-30`, and refreshed `.proof/evidence/ki-stdout-writedata-swallows-ctx-error.yaml`. |

## KnownIssue Slice: Storage Through Write Failure

Subagent audit completed for 20 KnownIssues from
`storage-connector-singleton-race` through
`write-failure-after-pop-loses-records`.

| Item | Type | Verdict | Evidence Checked | Action |
| --- | --- | --- | --- | --- |
| `storage-connector-singleton-race` | KnownIssue | `needs_reproducer_hardening` | Static singleton race evidence remains plausible, but no focused race witness proves concurrent connector initialization. | Attached and refreshed `.proof/evidence/ki-storage-connector-singleton-race.yaml`; backlog remains to add a `go test -race` or deterministic concurrent initialization witness. |
| `storage-createconnector-kv-list-err-unreachable` | KnownIssue | `valid` | `go test -count=1 -timeout 180s ./storage` passed; the listed KV-list error branch remains structural/unreachable proof debt. | No metadata change; already reviewed through `2026-11-30`. |
| `storage-ensureconnection-error-path-unreachable` | KnownIssue | `valid` | `go test -count=1 -timeout 180s ./storage` passed; the ensure-connection error path remains structural/unreachable proof debt. | No metadata change; already reviewed through `2026-11-30`. |
| `storage-retry-maxelapsed-zero-is-unbounded` | KnownIssue | `valid` | `TestGetTemporalStorageExponentialBackoff` passed and still covers the unbounded retry elapsed-time posture. | Tightened reproducer command and refreshed `.proof/evidence/ki-storage-retry-maxelapsed-zero-is-unbounded.yaml`. |
| `sw-req-009-demo-randomness-obligation-misattributed` | KnownIssue | `valid` | Static source and requirement inspection show demo randomness is proof attribution debt, not the analytics requirement owner. | Attached and refreshed `.proof/evidence/ki-sw-req-009-demo-randomness-obligation-misattributed.yaml`. |
| `sw-req-060-hash-format-obligation-misattributed` | KnownIssue | `valid` | Hash-format obligation ownership remains misattributed to the broad aggregate requirement rather than the true hash contract. | Attached and refreshed `.proof/evidence/ki-sw-req-060-hash-format-obligation-misattributed.yaml`. |
| `sw-req-073-parameterized-read-obligation-misattributed` | KnownIssue | `valid` | Parameterized-read ownership remains a proof/spec attribution issue across uptime reads rather than direct `SW-REQ-073` behavior. | Attached and refreshed `.proof/evidence/ki-sw-req-073-parameterized-read-obligation-misattributed.yaml`. |
| `syslog-init-logfatal-on-invalid-transport` | KnownIssue | `needs_metadata_hardening` | `TestSyslogPump_initConfigs_InvalidTransport_InProcess` passed and proves the fatal path is represented; metadata still needs clearer process-exit disposition wording. | Added focused reproducer command and refreshed `.proof/evidence/ki-syslog-init-logfatal-on-invalid-transport.yaml`; backlog: tighten release rationale. |
| `syslog-initwriter-logfatal-on-dial-error` | KnownIssue | `needs_metadata_hardening` | `TestSyslogPump_initWriter_BadDial_(Subprocess|InProcess)` passed and covers dial-error fatal behavior; metadata still needs clearer process-exit disposition wording. | Added focused reproducer command and refreshed `.proof/evidence/ki-syslog-initwriter-logfatal-on-dial-error.yaml`; backlog: tighten release rationale. |
| `systemconfig-omitdetailedrecording-unused` | KnownIssue | `needs_metadata_hardening` | Static config evidence remains valid, but the record needs a sharper distinction between unused configuration and user-visible product behavior. | Stamped review date `2026-11-30`. |
| `temporal-storage-operations-ignore-caller-cancellation` | KnownIssue | `valid` | Static evidence still shows storage/retry paths use background contexts rather than propagating caller cancellation. | Attached and refreshed `.proof/evidence/ki-temporal-storage-operations-ignore-caller-cancellation.yaml`. |
| `temporal-storage-wire-format-unversioned` | KnownIssue | `needs_reproducer_hardening` | Wire-format compatibility risk is real, but there is no versioned fixture/backward-compatibility tripwire. | Attached and refreshed `.proof/evidence/ki-temporal-storage-wire-format-unversioned.yaml`; backlog remains to add golden fixture evidence. |
| `tls-insecure-skip-verify-allowed` | KnownIssue | `valid` | Static evidence still shows configuration surfaces that allow `InsecureSkipVerify`; this is an accepted operator-risk posture. | Attached and refreshed `.proof/evidence/ki-tls-insecure-skip-verify-allowed.yaml`. |
| `uptime-aggregate-erasstr-itoa-always-nonempty` | KnownIssue | `needs_metadata_hardening` | `TestAggregateUptimeData_TCPErrorWithURL` passed; record should explain that `strconv.Itoa` always produces a non-empty string and makes the branch unreachable. | Tightened reproducer command and refreshed `.proof/evidence/ki-uptime-aggregate-erasstr-itoa-always-nonempty.yaml`; backlog remains to update remediation/status around current `mcdc:ignore`. |
| `uptime-aggregate-nil-errormap-on-tcp-then-http` | KnownIssue | `needs_reproducer_hardening` | The configured reproducer name does not currently exist; available uptime tests pass but do not prove the TCP-then-HTTP nil-map case. | No metadata change; already reviewed through `2026-11-30`; backlog: add the missing focused test or correct the record. |
| `uptime-onconflict-request-time-is-zero-unreachable` | KnownIssue | `needs_metadata_hardening` | `TestOnConflictUptimeAssignments_RequestTimeZeroSkipped` passed; record remains structural proof debt for an unreachable branch. | Tightened reproducer command and refreshed `.proof/evidence/ki-uptime-onconflict-request-time-is-zero-unreachable.yaml`; backlog remains to update remediation/status around current `mcdc:ignore`. |
| `uptime-pump-init-error-ignored` | KnownIssue | `needs_reproducer_hardening` | Static source evidence is accurate, but no focused test proves init errors are ignored by the uptime startup path. | Attached and refreshed `.proof/evidence/ki-uptime-pump-init-error-ignored.yaml`; backlog remains to add startup-path witness. |
| `uptime-pump-not-shutdown` | KnownIssue | `needs_reproducer_hardening` | Static source evidence is accurate, but no lifecycle test proves uptime pump shutdown is omitted. | Attached and refreshed `.proof/evidence/ki-uptime-pump-not-shutdown.yaml`; backlog remains to add shutdown-path witness. |
| `uptime-write-has-no-error-return` | KnownIssue | `valid` | Static interface/source evidence still shows uptime writes have no caller-visible error return. | Attached and refreshed `.proof/evidence/ki-uptime-write-has-no-error-return.yaml`. |
| `write-failure-after-pop-loses-records` | KnownIssue | `needs_reproducer_hardening` | Static proof shows pop-before-write without DLQ/requeue, but no end-to-end failing pump tripwire demonstrates post-pop loss. | Attached and refreshed `.proof/evidence/ki-write-failure-after-pop-loses-records.yaml`; backlog remains to add a storage-pop plus write-failure witness. |

## Verification Commands

- `go test -p=1 -count=1 -timeout=20m ./pumps -run '^(TestElasticsearchPump_ApiKeyAuthDroppedWhenUseSSL_KI|TestGetMapping_DecodeBase64MalformedInput_KI|TestElasticsearchPump_WriteData_MCPIndexRouting_NonBulkBug|TestElasticsearchPump_WriteData_V7ProcessDataIndexError)$'`
- `PATH=/Users/buger/go/bin:$PATH PROOF_MONITOR_MIN_FREE_PERCENT=3 PROOF_MONITOR_KILL_FREE_PERCENT=2 PROOF_MONITOR_KILL_SAMPLES=5 ./bin/run-monitored go test -p=1 -count=1 -timeout=180s -run '^(TestSQSPump_WriteData_PartialBatchFailureIgnored_KI|TestSQSPump_WriteData_MalformedRecordLeavesEmptyEntry_KI|TestSQSPumpBatchLimitZeroInfiniteLoop_KI|TestStdOutPump_WriteData_ContextCancelled_KI)$' ./pumps`
- `PATH=/Users/buger/go/bin:$PATH PROOF_MONITOR_MIN_FREE_PERCENT=3 PROOF_MONITOR_KILL_FREE_PERCENT=2 PROOF_MONITOR_KILL_SAMPLES=5 ./bin/run-monitored go test -p=1 -count=1 -timeout=180s -run '^TestMCDC_KI_DefaultMigrationTodayOnly$' ./pumps`
- `PATH=/Users/buger/go/bin:$PATH PROOF_MONITOR_MIN_FREE_PERCENT=3 PROOF_MONITOR_KILL_FREE_PERCENT=2 PROOF_MONITOR_KILL_SAMPLES=5 ./bin/run-monitored /Users/buger/go/bin/proof audit --check annotation_validity --check known_issue_complete --check known_issues_reviewed --max-findings 0`
- `PATH=/Users/buger/go/bin:$PATH PROOF_MONITOR_MIN_FREE_PERCENT=3 PROOF_MONITOR_KILL_FREE_PERCENT=2 PROOF_MONITOR_KILL_SAMPLES=5 ./bin/run-monitored /Users/buger/go/bin/proof known-issue check`
- `/Users/buger/go/bin/proof evidence validate --strict $(find .proof/evidence -name 'ki-*.yaml' | sort)` -> `valid: 29 evidence profile result(s)`
- `/Users/buger/go/bin/proof evidence validate --strict $(find .proof/evidence -name 'ki-*.yaml' | sort)` -> `valid: 30 evidence profile result(s)` after adding timeout-zero evidence.
- `/Users/buger/go/bin/proof evidence validate --strict $(find .proof/evidence -name 'ki-*.yaml' | sort)` -> `valid: 41 evidence profile result(s)` after adding the static health/hybrid/graph/external/elapsed evidence batch.
- `/Users/buger/go/bin/proof evidence validate --strict $(find .proof/evidence -name 'ki-*.yaml' | sort)` -> `valid: 52 evidence profile result(s)` after adding the Kinesis/Kafka/Resurface/Retry/Moesif/Splunk/Uptime evidence batch.
- `/Users/buger/go/bin/proof evidence validate --strict $(find .proof/evidence -name 'ki-*.yaml' | sort)` -> `valid: 65 evidence profile result(s)` after adding the backend/static KI evidence batch and refreshing the Moesif attribution record.
- `/Users/buger/go/bin/proof evidence validate --strict $(find .proof/evidence -name 'ki-*.yaml' | sort)` -> `valid: 81 evidence profile result(s)` after adding the aggregate/config/CSV/protobuf/Splunk/storage/SQL proof evidence batch.
- `/Users/buger/go/bin/proof evidence validate --strict $(find .proof/evidence -name 'ki-*.yaml' | sort)` -> `valid: 92 evidence profile result(s)` after adding the analytics/main/prometheus/instrumentation/Kinesis/Graylog evidence batch.
- `/Users/buger/go/bin/proof evidence validate --strict $(find .proof/evidence -name 'ki-*.yaml' | sort)` -> `valid: 107 evidence profile result(s)` after adding the retry/SDK/Resurface/SQL/storage/uptime static-tripwire evidence batch.

Latest focused Proof result: `Errors: 0`, `Warnings: 0` for
`annotation_validity`, `known_issue_complete`, and `known_issues_reviewed`.
Latest `proof known-issue check` result after the evidence/security-detail
slice: exit code `0`; summary
`status=open:50,reviewed:94 severity=high:7,low:55,medium:82 cve_surface=possible:7 security_relevant=7`.
Remaining closure debt is `31` missing current reproducer-evidence manifests;
security-detail metadata findings are closed. The remaining evidence debt is
reflected in the per-item `needs_reproducer_hardening` and
`needs_metadata_hardening` verdicts.

Tracked-debt follow-up: the only live stakeholder `witness_deferred`
(`STK-REQ-002:AC-003`) now carries `owner: tyk-team`,
`review_date: 2026-11-30`, and
`release_disposition: ship_with_known_issue`. The stale deferred-witness
backlog Section B was reduced from 10 historical rows to the one current row.
