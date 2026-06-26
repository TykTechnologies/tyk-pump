# Product Fix History vs Current Proof Model

Date: 2026-06-23
Repo: TykTechnologies/tyk-pump
Branch reviewed locally: feat/reqproof-coverage
Upstream ref reviewed: origin/master at e8611d0

## Scope

This note records the product/runtime part of the history review: whether the
current proof model would catch historical tyk-pump defects if equivalent bugs
were present today.

Excluded from this document:

- Release engineering fixes: GitHub Actions, release-bot, dep-guard, workflow
  permissions, release timeout tuning, CI runner migration.
- CVE-only/dependency bump fixes unless the commit exposed a product behavior
  contract.
- Pure docs/comment changes unless they reflected an operator-visible product
  contract gap.

The question answered here is intentionally strict:

> If the current proof model existed before these historical fixes, would it
> have found the issue immediately?

The honest answer is no. The current model would catch or surface many of these
defect classes today, but some of that coverage exists because those historical
failures taught us what to specify, test, or track as KnownIssue debt.

## Overall Assessment

Current proof is strongest where the behavior is now explicitly modeled:

- SQL/Mongo sharding and aggregate routing.
- MCP and GraphQL record classification/routing.
- Mongo aggregate document-size and self-healing behavior.
- Prometheus secrets/cardinality/default path behavior.
- Syslog single-line output preservation.
- Base64 raw payload decode behavior.
- Per-pump timeout, backend failure, fanout, and panic-isolation obligations.
- Configuration strictness and malformed input handling as tracked debt.

Current proof is weaker where behavior is not yet modeled as a product contract:

- Exact feature semantics that were added after the fact but not decomposed into
  a requirement.
- Backend-specific arithmetic or write-loop mistakes without a targeted witness.
- Packaging/deployment behavior outside the pump runtime.
- Dependency/CVE scanner outcomes unless scanner output is ingested into proof.

Approximate classification from the reviewed product-fix history:

- Likely caught today as hard evidence/regression: 35-45%.
- Likely surfaced today as KnownIssue/deferred obligation/warning: 30-40%.
- Likely missed unless additional modeling is added: 15-25%.

The limiting factor is not MC/DC alone. MC/DC verifies decision coverage for
specified logic. It does not invent missing product requirements, missing
backend-mode partitions, or missing operator contracts.

## Recent Product Fixes

| Commit | Date | Product issue | Current proof posture |
| --- | --- | --- | --- |
| 140ef71 | 2026-06-17 | SQL aggregate sharded writes inserted into the model default table instead of the caller-provided shard table. | `surfaced_as_debt`: this branch lacks the upstream fix, so the active gap is tracked by KI `sql-aggregate-sharded-upsert-targets-base-table`, obligation `routing_target_consistent`, and signal `go.sql-aggregate-upsert-without-table-target`. |
| be39ad3 | 2026-04-28 | MCP Mongo aggregate cross-API document merge: upsert key lacked owner API id. | `caught`: current branch contains upstream fix; DEFECT-1 binds TT-17004 to SW-REQ-039, `owner_apiid` upsert-key behavior, and `TestMCPMongoAggregatePump_WriteData_PerAPIPartitioning` edge-case/output-cardinality evidence. |
| 19fb73d | 2025-10-23 | SQL table-sharding migration skipped sharded tables. | `caught`: DEFECT-3 and review `REVIEW-19fb73d-sql-shard-migration` tie TT-13166 to SW-REQ-040, INT-REQ-007 migration witnesses, `TestHandleTableMigration`, and `TestMigrateAllShardedTables`; residual default-mode prior-day shard, Graph SQL aggregate startup migration, and schema-version policy gaps remain KI-tracked under `sql-default-migration-today-only`, `graph-sql-aggregate-migrate-sharded-tables-ignored`, and `sql-schema-no-version-policy`. |
| 0596e82 / df62011 | 2025-08-15/16 | Syslog multiline raw request/response fragmented one analytics record into multiple syslog lines. | `caught`: DEFECT-2, SW-REQ-050, `encoding_safety`, and `output_cardinality_bounded` evidence cover the final backward-compatible behavior: legacy `map[...]` output with LF escaping in `raw_request`/`raw_response`; review `REVIEW-0596e82-syslog-fragmentation` records the superseded JSON approach in df62011. |
| fbcb614 | 2024-11-27 | SQL pump panicked when table sharding was enabled and all records were skipped. | `caught`: DEFECT-4, SW-REQ-040, and the `TestSQLWriteDataSharded/empty_keys` boundary witness cover the sharded empty-batch slice-bound regression; adjacent SQL-family KIs are linked in the defect disposition. |
| 544ccb3 | 2024-12-05 | Sharded SQL pumps did not create needed indexes on shard tables. | `missed_before_hardening`: standard SQL and SQL Aggregate shard-index behavior are now covered by DEFECT-7, `per_shard_index_created`, and Postgres shard-index witnesses; Graph/MCP SQL shard-index contracts and MySQL index DDL validity remain KI-backed debt (`graph-mcp-sql-sharded-indexes-untracked`, `sql-standard-mysql-create-index-if-not-exists-unsupported`, `mcp-sql-aggregate-mysql-create-index-syntax-broken`). |
| 866bdc2 | 2026-02-09 | CA certificate configuration for Elasticsearch/Kafka/Splunk TLS clients. | `missed_before_hardening`: shared TLS parsing was covered, but per-backend positive CA-root attachment was weak. Hardened via `cert_chain_validated` on SW-REQ-016/021/048/068, CA `RootCAs` evidence in Common/Kafka/Splunk/Elasticsearch tests, and review `REVIEW-866bdc2-ca-cert`; operator `ssl_insecure_skip_verify` remains KI-tracked debt. |
| 1c20a08 | 2026-06-10 | RFC3339-compliant log format option. | `missed_before_hardening`: current branch lacks upstream TT-17281. Not treated as an active product bug because this branch only documents `text`/`json`; proof now pins current legacy timestamp behavior via SW-REQ-033, obligation `log_timestamp_format_declared`, and logger formatter evidence. |
| 8a614d6 | 2026-06-09 | Add original_path and listen_path observability fields. | `missed_before_hardening`: current branch lacks upstream TT-7519 fields, so no active KI. Hardened INT-REQ-003, SW-REQ-008, and SW-REQ-009 to require additive AnalyticsRecord fields to update schema/transformer/helper evidence explicitly. |

## Older Runtime/Product Fixes

| Commit | Date | Product issue | Current proof posture |
| --- | --- | --- | --- |
| d9d64dc | 2024-01-29 | Some GraphQL pumps did not read env configuration correctly. | `missed_before_hardening`: config obligations were adjacent, and Graph SQL aggregate had an env subtest, but Graph Mongo did not directly witness its default env overlay and neither graph pump named the env-prefix contract. Hardened with DEFECT-46, local obligation `env_override_applied`, Graph Mongo default-env Init evidence, and Graph SQL aggregate env evidence. |
| 956b66a | 2024-01-31 | Default Mongo driver migration to mongo-go. | `policy_hardened`: this was a support/default-policy migration rather than DEFECT/DFX material. Mongo functional tests existed, but the blank-driver support/default policy was not an explicit requirement. Hardened with review `REVIEW-956b66a-mongo-driver-default`, `support_matrix_enforced` on SW-REQ-034/035/036, docs/spec text for the v1.9+ `mongo-go` default, and default-driver evidence for standard/selective/aggregate Mongo paths. The default-driver tests are intentionally obligation evidence rather than MC/DC witnesses for Mongo write-routing formulas. |
| 775f8e3 | 2025-02-26 | SQLite support removed from SQL pumps. | `missed_before_hardening`: now modeled by `support_matrix_enforced` on SQL-family requirements with SQLite-rejection evidence through production Init/Dialect paths; external tyk-docs support-matrix drift remains KI-backed debt under `docs-sqlite-still-listed-as-supported-sql-type`. |
| 35e1e74 | 2024-01-04 | Splunk backoff retry behavior. | `missed_before_hardening`: retry attempts and Splunk MaxRetries were partially covered, but request-body replay across retries was not explicit. Hardened with `request_body_replay_preserved` on SW-REQ-030, `retry_policy_explicit` on SW-REQ-048, a body-replay retry obligation witness, and review `REVIEW-35e1e74-splunk-retry`; residual risks remain KI-backed under `http-retry-post-idempotency-unenforced`, `retry-buffers-full-request-body-in-memory`, `retry-backoff-duration-not-deadline-bounded`, `retry-4xx-bodyread-fail-causes-retry`, `docs-splunk-mutates-default-transport-undocumented`, and `splunk-newsplunkclient-mutates-default-transport`. |
| 407c373 | 2024-01-26 | Migration to storage library changed storage/retry behavior. | `missed_before_hardening`: current proof covered pop/expire, retry, context/cancellation KIs, and wire-format debt, but full-drain adapter parity (`chunkSize=0` -> storage-library `-1`) and connection-mode option parity were only implicit. Hardened with `full_drain_semantics`, `backend_connection_mode_parity`, env/TLS/timeout obligations on SW-REQ-007, adapter/live-Redis evidence, and static storage-library option-parity evidence; residual storage debt remains KI-backed under `write-failure-after-pop-loses-records`, `getanddeleteset-expire-fail-loses-records`, `getanddeleteset-expire-ttl-assumes-clock-sync`, `storage-retry-maxelapsed-zero-is-unbounded`, `temporal-storage-operations-ignore-caller-cancellation`, `storage-connector-singleton-race`, and `temporal-storage-wire-format-unversioned`. |
| 7fa0754 | 2023-03-06 | Graph aggregate sharding and graph record errors. | `missed_before_hardening`: Graph SQL aggregate shard-target drift is now covered by DEFECT-19, SW-REQ-086, `routing_target_consistent` evidence, and `REVIEW-7fa0754-graph-sql-aggregate-shard-target`. The same commit's legacy graph parse-error refactor is superseded by the current SW-REQ-013 GraphQLStats-authoritative projection model, with malformed legacy payload evidence and graph error preservation tests; DEFECT-19 now cross-references DEFECT-23 for that parser/error half. |
| 17072c0 | 2023-02-21 | RootFields missing from Graph Mongo/SQL pumps. | `covered_after_hardening`: DEFECT-20 and SW-REQ-087 decompose RootFields projection from SW-REQ-013, add `structured_projection_preserved` evidence and MC/DC witness rows, and reuse current analytics/downstream graph tests that prove RootFields are copied from GraphQLStats and remain present in aggregate/SQL paths. |
| 8e42170 | 2022-11-03 | Edge case for unresolved subgraph schema. | `covered_after_hardening`: DEFECT-23, SW-REQ-013, and SW-REQ-037 now make GraphQLStats the authoritative GraphRecord source for analytics projection and Graph Mongo writes, so legacy raw request/response/schema parser failures such as unresolved federation `_entities` selections or empty legacy payloads cannot drop structured graph stats. `TestAnalyticsRecord_ToGraphRecord_IgnoresUnresolvedLegacySubgraphSchema` and `TestGraphMongoPump_WriteData` pin the current behavior. |
| d13b62e | 2022-11-02 | Mongo aggregate 16 MiB self-healing and configurable aggregation window. | `covered_after_hardening`: SW-REQ-058 covers the parent window policy; SW-REQ-107/108/109 now split valid configured windows, invalid default-to-60 behavior, and the WriteData-to-AggregateData active-window handoff. SW-REQ-062 covers backend size-error classification, aggregation-time halving, timestamp reset, repeat-until-success-or-floor retry behavior, and same-batch retry. Hardened with DEFECT-24 plus bounded timestamp-reset, AST handoff/retry-wiring, and fake-store behavioral tests, avoiding the historical 16 MiB stress test in routine runs. |
| ff7574e | 2022-10-25 | Mongo graph records ignored max_document_size_bytes. | `covered_after_hardening`: DEFECT-26 and SW-REQ-037 now make the graph-mode `MaxDocumentSizeBytes` exception explicit with a `boundary` obligation. Repaired GraphQLStats-backed evidence proves oversized Graph Mongo records retain `RawRequest`/`RawResponse` through `AccumulateSet(data, true)`, while standard Mongo rewrite behavior remains owned by SW-REQ-034. |
| 4c490cb | 2022-10-13 | Mongo selective document size was miscalculated. | `covered_after_hardening`: DEFECT-29 and SW-REQ-092 now pin the exact MongoSelective document-size formula, `document_size_accounting_exact`, and evidence that `RawRequest` and `RawResponse` are each counted once with the 1024-byte metadata allowance and strict greater-than threshold. MC/DC ownership remains on the exact size helper witness; AccumulateSet skip behavior is kept as obligation evidence. Current final-skipped-item batch loss remains KI-backed under `mongo-selective-final-skipped-record-drops-pending-batch`. |
| c54eed3 | 2022-08-23 | Mongo aggregate REST path needed to exclude graph analytics while standard raw Mongo later needed to retain them. | `covered_after_hardening`: the current REST aggregate graph/MCP partition is directly covered by SW-REQ-093 and `output_cardinality_bounded` evidence so GraphQL/MCP records do not re-enter REST aggregates, same-organisation graph records do not inflate REST hit counts, and ordinary REST records are retained. The standard-Mongo graph-record exclusion lineage is superseded by the later 0ed84b9/TT-8884 history and is covered by DEFECT-14/SW-REQ-082, now including current GraphQLStats and legacy tag-only standard-mode retention evidence. |
| 2e26ba7 | 2022-06-10 | Avoid double discard of aggregations. | `covered_after_hardening`: DEFECT-32 and SW-REQ-096 now own ignored-dimension retention for Mongo aggregate mixed collections with `aggregate_dimension_retention` evidence proving the final ignoring-writer pass succeeds, omits its ignored `apikey1`, and does not erase `apikey2` written by a non-ignoring writer. |
| 224f919 | 2022-05-17 | Mongo indexes on DocumentDB; omit-index creation compatibility. | `covered_after_hardening`: DEFECT-33 and SW-REQ-097/098/099 now cover standard/selective/aggregate DocumentDB index compatibility with `backend_ddl_valid` and `index_definition_matches_query` evidence: DocumentDB paths skip the StandardMongo collection-exists probe, honor `omit_index_creation`, and attempt exact foreground index definitions including key order, TTL=0 expiry metadata, and non-TTL ordinary-index metadata. |
| 14ccaba | 2020-12-22 | Mongo pumps timeout configurability. | `covered_after_hardening`: fixed historical connection-timeout configurability is now captured by DEFECT-34, `backend_connection_timeout_propagated`, and source-level witnesses that `initialisePumps` passes `pmp.Timeout` to `SetTimeout` before `Init(pmp.Meta)` and standard/selective/aggregate Mongo pass `m.timeout` to `persistent.ClientOpts.ConnectionTimeout`. These source-level timeout witnesses are obligation evidence rather than MC/DC witnesses for Mongo write-routing formulas. Standard Mongo first per-batch insert-error observation is covered by SW-REQ-034 and `TestMongoPump_WriteData_InsertErr`; remaining multi-batch double-send goroutine-lifetime debt is KI-backed under `mongo-standard-insert-error-double-send-goroutine-leak`. Residual live timeout debt remains KI-backed under `mongo-pump-ignores-caller-context`, `mongo-aggregate-last-document-query-ignores-timeout`, and `pump-no-timeout-can-block-purge-cycle`. |
| 34e1a2c | 2020-08-19 | omit_detailed_recording missing on syslog pump. | `missed_before_hardening`: SYS-REQ-015 and SW-REQ-016 covered the generic privacy transform/common setter surface, but no witness proved syslog backend output actually reflected per-pump omit_detailed_recording. Hardened SW-REQ-050 so syslog output must satisfy SYS-REQ-015 at the backend boundary, added local obligation `per_backend_privacy_transform_applied`, DEFECT-35, and a real UDP syslog witness that runs `filterData` with `SetOmitDetailedRecording(true)` and asserts emitted syslog contains empty `raw_request`/`raw_response` map fields and none of the original payload bytes. The witness now carries exact code-only MC/DC rows for the SYS-REQ-015 true privacy row and SW-REQ-050 UDP transport row. Top-level/global omit behavior remains KI-backed under `systemconfig-omitdetailedrecording-unused`. |
| 416d1c7 / 0adb849 | 2020-05/08 | Elasticsearch base64 decode handling. | `missed_before_hardening`: generic mapping and core base64-decode requirements were adjacent, and `TestGetMapping_ExtendedStatistics` already asserted decoded values, but SW-REQ-068 did not explicitly own Elasticsearch `decode_base64` textual mapping or the historical `[]byte` JSON re-encode failure. Hardened with DEFECT-36, local obligation `backend_decoded_payload_textual`, SW-REQ-068/docs text, typed decoded-payload obligation evidence in `TestGetMapping_ExtendedStatistics`, and a static signal for direct `DecodeString` assignment into backend maps. Current malformed ES decode behavior remains KI-backed under `elasticsearch-decode-base64-errors-silent-empty`. |
| 402dab8 | 2020-06-23 | Health endpoint only available from localhost. | `missed_before_hardening`: SW-REQ-032 covered liveness response, pprof gating, and several health endpoint KIs, but did not explicitly require the production listener to bind outside loopback. Hardened with DEFECT-37, local obligation `listener_bind_scope_external`, SW-REQ-032/docs text, source-level `ServeHealthCheck` listener-address obligation evidence, and a signal for loopback-only health listeners. Existing health debt remains KI-backed under `health-endpoint-is-liveness-only`, `health-endpoint-auth-not-enforced`, `health-endpoint-rate-limit-not-enforced`, `health-listener-bind-failure-logfatal`, and `docs-health-endpoint-liveness-vs-readiness-unclear`. |
| 6a8ab73 | 2020-03-12 | Mongo index already-exists error should be ignored. | `missed_before_hardening`: current proof had Mongo index requirements and DocumentDB hardening, but not the exact same-key/different-name `logBrowserIndex` idempotency contract from #234/#237. Hardened with DEFECT-38, local obligation `idempotent_schema_setup`, SW-REQ-034/SW-REQ-035 text, fake-store selective idempotency/non-sentinel evidence, and a standard-Mongo KI tripwire for the remaining non-StandardMongo conflict. StandardMongo already-existing collection short-circuit is covered; current standard non-StandardMongo conflict remains KI-backed under `mongo-standard-logbrowser-compatible-index-conflict`. |
| 58da62f | 2019-11-19 | Mongo aggregate field names with unsupported '.' characters corrupted data. | `missed_before_hardening`: aggregate tests covered counter accumulation and the helper branch, but not the product contract that tracked REST paths containing "." must be encoded before becoming Mongo update field paths. Hardened with DEFECT-39, local obligation `backend_field_key_safe`, SW-REQ-011/docs text, an aggregate-to-update-document witness, and a signal for raw endpoint path keys. |
| 8bfdb36 | 2019-11-05 | Mongo document size calculated incorrectly and useful data skipped. | `missed_before_hardening`: standard Mongo was adjacent to selective/graph size hardening, but SW-REQ-034 did not explicitly own the exact RawRequest+RawResponse+1024 formula or prove oversized records are rewritten rather than skipped. Hardened with DEFECT-40, `document_size_accounting_exact`, standard Mongo size/rewrite witnesses, and a signal for RawRequest-counted-twice estimates. A separate current final-skipped-item flush gap is KI-backed under `mongo-standard-final-skipped-record-drops-pending-batch`. |
| aa7a88e | 2019-10-03 | Mongo selective pump wrote TCP records incorrectly. | `missed_before_hardening`: SW-REQ-035 covered per-org routing and helper-level skip branches, but did not explicitly state that selective Mongo writes only non-MCP HTTP analytics and excludes TCP/error records (`ResponseCode == -1`) through batching. Hardened with DEFECT-41, SW-REQ-035/docs text, and an AccumulateSet witness that retains HTTP records while dropping TCP/error records. |
| 3bb755d | 2019-08-23 | Elasticsearch analytics missing alias field. | `missed_before_hardening`: generic mapping and field-preservation proof was adjacent, and `TestGetMapping_BasicFields` already asserted alias after later test work, but no Elasticsearch software requirement explicitly owned the Alias-to-"alias" projection. Hardened with DEFECT-42, SW-REQ-100, `structured_projection_preserved`, and direct populated/empty alias getMapping witnesses. |
| 51af27d | 2019-08-13 | Influx write was inside loop, causing duplicate/cumulative writes and severe slowness. | `missed_before_hardening`: SW-REQ-046 had the Influx v1 write path and an output-cardinality obligation, but only a single-record witness and no explicit "one backend write per purge batch, after accumulation" invariant. Hardened with DEFECT-43, SW-REQ-101, and a multi-record httptest witness proving exactly one /write request with one line-protocol row per input record. |
| d4d1cf7 | 2019-08-08 | Mongo aggregate mixed collection lost Lists data. | `missed_before_hardening`: mixed aggregate tests already asserted counters and later ignored-dimension retention asserted Lists.APIKeys incidentally, but no requirement explicitly owned the persisted Lists projection restored by d4d1cf7. Hardened with DEFECT-44, SW-REQ-102, `structured_projection_preserved`, BSON round-trip evidence for all restored Lists families, and live Mongo readback evidence for Lists.APIKeys in both per-org and mixed aggregate collections. |
| c02a2cb | 2019-07-16 | HTTP 400 was not counted as an error in aggregates. | `covered_after_hardening`: SW-REQ-103 now pins the REST aggregate HTTP error boundary (`ResponseCode >= 400`) with DEFECT-45, a 399/400/500 boundary regression test, and signal rule `go.aggregate-http-error-threshold-strict-greater-than` in `signal-rules/tyk-pump-phase-n.yaml`. |

## Reviewed Git Commit Set

The table below records the product/runtime git commits considered during the
history review. It is intentionally broader than the detailed assessment above,
so a later reviewer can see what was included and what was filtered out.

| Commit | Date | Subject | Review disposition |
| --- | --- | --- | --- |
| 140ef71 | 2026-06-17 | [TT-16778] Fix sharded aggregate SQL writes | `surfaced_as_debt`: upstream fixed, current branch not fixed. Added KI `sql-aggregate-sharded-upsert-targets-base-table`, local obligation `routing_target_consistent`, and code signal `go.sql-aggregate-upsert-without-table-target`. |
| 1c20a08 | 2026-06-10 | [TT-17281] Add RFC3339 compliant option for consistent Tyk Pump logs and field mapping | `missed_before_hardening`: upstream feature not present here. Hardened SW-REQ-033 with `log_timestamp_format_declared` and exact legacy timestamp-format witness so any later RFC3339/legacy contract change must update the spec/evidence explicitly. |
| 8a614d6 | 2026-06-09 | [TT-7519] add original_path and listen_path to observability | `missed_before_hardening`: upstream additive fields are absent here. Hardened INT-REQ-003, SW-REQ-008, and SW-REQ-009 so future additive AnalyticsRecord fields need explicit schema, transformer, helper, and evidence updates. |
| be39ad3 | 2026-04-28 | [TT-17004] Fix MCP Mongo aggregate cross-API merge | `caught`: DEFECT-1 + SW-REQ-039 + monitored regression `TestMCPMongoAggregatePump_WriteData_PerAPIPartitioning`; proof MC/DC/evidence rows are covered. |
| bad4cd3 | 2026-04-24 | [TT-16809] Pump generate MCP analytics | `caught`: broad feature surface, not a DEFECT. Review `REVIEW-bad4cd3-mcp-analytics` ties current coverage to SW-REQ-012/014/038/039/044/045/069, serializer/interface evidence, and residual MCP KIs `elasticsearch-mcp-routing-non-bulk-ignored`, `mcp-sql-aggregate-mysql-create-index-syntax-broken`, `mcp-sql-aggregate-background-index-concurrency-unbounded`, and `mcp-mongo-aggregate-atomicity-fault-injection-missing`. |
| 866bdc2 | 2026-02-09 | [TT-15674] Extend pumps config with ca cert option | `missed_before_hardening`: added `cert_chain_validated` obligations and per-backend valid-CA evidence for SW-REQ-016/021/048/068; skip-verify policy remains tracked by KI `tls-insecure-skip-verify-allowed`. |
| 19fb73d | 2025-10-23 | [TT-13166] regression when using sql table sharding pump | `caught`: DEFECT-3 + SW-REQ-040/INT-REQ-007 migration evidence cover the skipped-shard-migration regression; residual policy gaps remain under KIs `sql-default-migration-today-only`, `graph-sql-aggregate-migrate-sharded-tables-ignored`, and `sql-schema-no-version-policy`. |
| ebd5a6c | 2025-10-23 | [TT-14871] Expose Gateway-Only Latency in Tyk Metrics | `missed_before_hardening`: current code already exposed gateway latency through AnalyticsRecord helpers, StatsD timing fields, and the Prometheus `tyk_latency` histogram, but the proof model did not state the gateway projection explicitly. Hardened SW-REQ-009/023/024 and new child SW-REQ-104 with `structured_projection_preserved`, direct StatsD `latency_gateway` evidence, Prometheus `total`/`upstream`/`gateway` value evidence, and review `REVIEW-ebd5a6c-gateway-latency-metrics`. |
| 50e5f51 | 2025-10-16 | [TT-14473] support for encrypted aws kinesis | `missed_before_hardening`: fixed historical Kinesis KMS state-classification bug introduced with 5965206. Hardened with DEFECT-47, new SW-REQ-105, local obligation `kms_stream_state_reconciled`, and nil-`KeyId` DescribeStream evidence so KMS-encrypted streams without a reported key id are reconciled through `StartStreamEncryption` instead of misclassified as a different configured key. MC/DC witness triage is now per-test for the KMS state-machine rows. |
| 5965206 | 2025-10-13 | [TT-14473] support for encrypted aws kinesis | `covered_after_hardening`: additive Kinesis KMS support is now covered by SW-REQ-056 plus child SW-REQ-105 for exact stream-state reconciliation. The env override tests remain obligation evidence, while the KMS Init tests carry code-only MC/DC rows for the formal KMS variables. Existing Kinesis KIs continue to track unrelated partition-key idempotency, batch-size, timeout, and WriteData error-surfacing debt. |
| 33d9f48 | 2025-10-13 | [TT-15560] added batchbytes configs and unit testing for kafka | `missed_before_hardening`: additive Kafka batching/config behavior was tested but only implicitly modeled under broad SW-REQ-021. Hardened with new SW-REQ-106, local obligation `backend_batch_byte_limit_applied`, batch-bytes variables, direct evidence for positive/zero/omitted/env/invalid/negative cases, code-only MC/DC witness rows in the Kafka test files, and review `REVIEW-33d9f48-kafka-batch-bytes`. |
| 0596e82 | 2025-08-16 | [TT-15532] Alternative backward-compatible fix for syslog pump log fragmentation | `caught`: DEFECT-2 + SW-REQ-050 verify one syslog entry per record and legacy `map[...]` output with escaped raw HTTP newlines. |
| df62011 | 2025-08-15 | Fix syslog pump log fragmentation issue | `caught`: superseded product-defect fix. Review `REVIEW-0596e82-syslog-fragmentation` records why the final 0596e82 behavior, not df62011 JSON output, is the proof contract. |
| 775f8e3 | 2025-02-26 | [TT-13341] Remove SQLite support from Tyk Pump | `missed_before_hardening`: generic unsupported-type paths existed, but proof did not pin SQLite as a removed backend. Hardened with local obligation `support_matrix_enforced`, production `Init` SQLite-rejection witnesses across SQL standard/aggregate/Graph/MCP variants, corrected INT-REQ-007 proof text, and review `REVIEW-775f8e3-sqlite-support-removal`; the SQLite-rejection test is intentionally obligation evidence rather than formal MC/DC evidence for the SQL routing formulas. External tyk-docs drift remains KI-tracked by `docs-sqlite-still-listed-as-supported-sql-type`. |
| 544ccb3 | 2024-12-05 | TT-13421 create indexes on sharded sql pumps | `missed_before_hardening`: added DEFECT-7, local catalog obligations `per_shard_index_created` and `backend_ddl_valid`, standard/aggregate shard-index witnesses, and KIs for Graph/MCP shard-index contracts plus MySQL index DDL debt. |
| fbcb614 | 2024-11-27 | TT-12780 prevent sql pump to panic when sharding enabled and skip api id is set | `caught`: DEFECT-4 + SW-REQ-040 boundary evidence cover the empty post-filter batch under `TableSharding=true`; review `REVIEW-fbcb614-sql-empty-shard-batch` records the verdict. |
| 25683f2 | 2024-08-01 | [TT-6671] AWS Kinesis pump | `surfaced_as_debt`: additive Kinesis backend feature is covered by SW-REQ-017 registry evidence, SW-REQ-056 Kinesis env/batching behavior evidence, and SW-REQ-105 KMS follow-up coverage. The env override tests are intentionally obligation witnesses rather than formal KMS MC/DC witnesses. Review `REVIEW-25683f2-kinesis-feature` records the feature verdict; live product/documentation gaps are tracked as KIs, including `docs-kinesis-env-streamname-typo`, `kinesis-random-partition-key-not-idempotent`, `kinesis-splitintobatches-zero-infinite-loop`, `pump-writedata-swallows-per-batch-errors`, `kinesis-putrecords-per-record-failures-return-nil`, and new `kinesis-batch-size-over-aws-putrecords-limit`. |
| 206c1d0 | 2024-06-06 | TT-12103 Adding FIPS Support | `missed_before_hardening`: proof already covered local FIPS build availability via SYS-REQ-026, but did not model release distribution. Hardened with DEFECT-48, new SYS-REQ-036, local obligation `security_mode_artifact_consistent`, and static release-config/workflow evidence that FIPS-labelled packages/images use FIPS build/package/base-image variants. |
| 956b66a | 2024-01-31 | TT-10409 Default Mongo driver migration to mongo-go | `policy_hardened`: support/default-policy migration, not DEFECT/DFX material. Hardened with review `REVIEW-956b66a-mongo-driver-default`, `support_matrix_enforced` on SW-REQ-034/035/036, docs/spec text for the v1.9+ `mongo-go` default, and code-annotated default-driver evidence for standard/selective/aggregate Mongo paths. |
| d9d64dc | 2024-01-29 | fix bug where some graphql pumps are not reading env properly | `missed_before_hardening`: now covered by DEFECT-46; SW-REQ-037/SW-REQ-043 explicitly require graph pump env overrides, Graph Mongo default env override is witnessed in Init, and Graph SQL aggregate keeps its env Init evidence tied to `env_override_applied`. |
| 8ca8646 | 2024-01-26 | TT-10676 Upgrade Resurface Pump backend | `surfaced_as_debt`: product backend behavior change introducing Resurface async worker/channel lifecycle. Current proof covers the behavior through SW-REQ-054 and Resurface tests, while live risks remain KIs: `resurface-writedata-blocks-on-queue-full`, `resurface-worker-errors-swallowed`, `resurface-disabled-writedata-closes-channel`, and existing raw reconstruction KI `resurface-maprawdata-empty-request-panic`. Review `REVIEW-8ca8646-resurface-backend` records no DEFECT because the open issues are not fixed historical bugs. |
| 407c373 | 2024-01-26 | [TT-10520] Migrating from go-redis to storage library | `missed_before_hardening`: added `full_drain_semantics` and `backend_connection_mode_parity` to pin the legacy drain-all behavior and storage-library connector option mapping; residual storage debt remains KI-backed under `write-failure-after-pop-loses-records`, `getanddeleteset-expire-fail-loses-records`, `getanddeleteset-expire-ttl-assumes-clock-sync`, `storage-retry-maxelapsed-zero-is-unbounded`, `temporal-storage-operations-ignore-caller-cancellation`, `storage-connector-singleton-race`, and `temporal-storage-wire-format-unversioned`. |
| bf9e7e7 | 2024-01-24 | TT-10675 add SQS Pump Backend support | `missed_before_hardening`: SQS behavior was covered by SW-REQ-055, but registry support for the new backend name was not pinned as support-matrix evidence. Hardened SW-REQ-017 with `support_matrix_enforced` and an SQS registry witness; live SQS gaps remain KI-backed under `sqs-malformed-record-sends-empty-entry`, `sqs-batchlimit-zero-infinite-loop`, and `sqs-batch-partial-failures-ignored`. |
| 35e1e74 | 2024-01-04 | TT-10564 Splunk backoff retry | `missed_before_hardening`: added `request_body_replay_preserved` and explicit body-replay obligation evidence for the shared retry helper, plus Splunk `retry_policy_explicit` evidence for MaxRetries behavior. |
| 6c14ba0 | 2023-11-08 | [TT-9476] Refactor graph pumps to use new GraphQLStats | `missed_before_hardening`: current graph tests covered the GraphQLStats happy path, but SW-REQ-013 did not explicitly require structured projection from GraphQLStats or rejection of legacy tag/raw parser sources when IsGraphQL=false. Hardened with local obligation `structured_projection_preserved`, malformed-input obligations, and a legacy-source negative witness. Existing Graph SQL shard-index, aggregate shard-migration, timestamp, and SQL batch-size KIs remain the tracked operational debt. |
| 9984a1e | 2023-10-30 | [TT-9476] add graphstats to analytics record proto | `missed_before_hardening`: SW-REQ-008/INT-REQ-003 already required additive serializer fields to update schema, transformers, and evidence, and current GraphQLStats round-trip tests catch missing protobuf field 32 or transformer mappings. Hardened with a GraphQLStats wire-field-number witness and KI `serializer-protobuf-loses-graphql-error-path`, because protobuf preserves GraphError.Message but drops GraphError.Path. Existing protobuf city-name and malformed sparse-message KIs remain tracked debt. |
| 7af2a45 | 2023-09-18 | [TT-100053] aggregate graph aggregate records by api_id | `missed_before_hardening`: SW-REQ-043 already described per-(API, dimension) Graph SQL aggregate upserts and the live Postgres case covered two API IDs, but the proof model did not carry an explicit aggregate partition obligation and the fast analytics witness only exercised one API. Hardened with local obligation `aggregate_partition_isolated` and `TestAggregateGraphData_PartitionsSameOrgByAPIID`, so same-org/same-dimension GraphQL records now have direct unit-level obligation evidence that API IDs remain separate before SQL upsert. Added `atomicity`, `transaction_isolation_declared`, and `errors_propagated` as Graph SQL aggregate backend-write obligations, deferred to KI `graph-sql-aggregate-atomicity-fault-injection-missing` until a transaction/failure-injection harness exists. |
| 657436f | 2023-08-29 | [TT-9468] New SQL Aggregate indexes | `missed_before_hardening`: current proof had SW-REQ-066 for SQL Aggregate index creation and Postgres evidence for index presence, but the history row still had no verdict and the evidence did not assert the physical composite index definition matched the documented query path. Hardened with DEFECT-8, local obligation `index_definition_matches_query`, and Postgres catalog evidence that the index columns are exactly `(dimension, timestamp, org_id, dimension_value)`. Split SQL Aggregate-specific KI debt for MySQL `CREATE INDEX IF NOT EXISTS` and background index lifecycle into `sql-aggregate-mysql-create-index-if-not-exists-unsupported` and `sql-aggregate-background-index-concurrency-unbounded`. |
| 69f5f4a | 2023-08-23 | TT-9873 Fix prometheus tracking path | `missed_before_hardening`: current proof had broad SW-REQ-024 Prometheus evidence and two WriteData path-label tests, but the tracking-path policy was not a first-class requirement/obligation and the global `TrackAllPaths=true` branch lacked WriteData registry evidence. Hardened with DEFECT-9, SW-REQ-078 for untracked `path="unknown"`, SW-REQ-079 for opt-in path preservation, local obligation `metric_path_label_tracking_policy`, and a `TrackAllPaths` boundary witness. Broad metric label-cardinality risk remains KI-backed under `metrics-label-cardinality-unbounded`. |
| 3bf1f85 | 2023-08-23 | [TT-9855] fix index creation error on graph sql pump creation | `missed_before_hardening`: current SW-REQ-042 covered Graph SQL table creation and graph-record routing, but proof did not pin the GORM model-table identity used during migration. Hardened with DEFECT-10, SW-REQ-080, local obligation `migration_model_table_identity_bound`, and Postgres Init evidence that `GraphRecord.TableName()` resolves to the configured Graph SQL table before migration. Broader Graph/MCP sharded-index and SQL schema-version gaps remain KI-backed under `graph-mcp-sql-sharded-indexes-untracked`, `graph-sql-aggregate-migrate-sharded-tables-ignored`, and `sql-schema-no-version-policy`. |
| 91dd8a0 | 2023-08-01 | [TT-9360] Changing Timeout from time.Duration to int | `missed_before_hardening`: current Kafka tests exercised duration strings, numeric seconds, and env override behavior, but those witnesses were attached to the TLS-focused SW-REQ-021 contract, so proof did not make timeout unit parsing a first-class requirement. Hardened with DEFECT-11, SW-REQ-081, local obligation `timeout_config_units_preserved`, MC/DC rows for configured timeout application, and a static signal for Kafka timeout fields or writer deadlines that reintroduce `time.Duration * time.Second` unit confusion. Malformed Kafka timeout still log.Fatals and remains KI-backed under `kafka-logfatal-on-init-mech-and-timeout`. |
| 6820131 | 2023-06-12 | [TT-9126] Fix error log when omit_configfile option is enabled | `partially_missed_before_hardening`: SW-REQ-002 already modeled the omit-config-file gate and caught the destructive old behavior that cleared caller defaults, but it did not explicitly assert the ticket-title behavior: omit=true with a nonexistent config path must not emit file-read or JSON-unmarshal error logs. Hardened with DEFECT-12, local obligation `config_file_omission_suppresses_file_read`, and a `TestIgnoreConfig` log-capture subtest proving env-only startup preserves config and suppresses misleading file errors. |
| 2d3c296 | 2023-05-25 | [TT-8884] added write data test for mongo pump and remove constraint | `missed_before_hardening`: the live Mongo test already caught the graph-only accumulation regression, but SW-REQ-034 only modeled MCP filtering. Hardened with DEFECT-13 and SW-REQ-082 so standard Mongo must insert ordinary and GraphQL-classified non-MCP records into the standard collection. |
| 0ed84b9 | 2023-05-17 | fix: include graph records in mongo pump | `missed_before_hardening`: this fixed the c54eed3-era standard Mongo exclusion of GraphQL-classified records, but did so by switching standard WriteData into graph-only accumulation, later fixed by TT-8884. Hardened with DEFECT-14 and SW-REQ-082; DEFECT-13 now records 0ed84b9 as the origin of the ordinary-record drop. |
| 8173c7e | 2023-05-15 | TT-876 Fix/prometheus cardinality | `missed_before_hardening`: current Prometheus tests covered separator-bearing label values, but no requirement modeled aggregate label tuple-boundary preservation. Hardened with DEFECT-15 and SW-REQ-083; broad unbounded metric cardinality remains KI-backed under `metrics-label-cardinality-unbounded`. |
| 1fd5ba9 | 2023-04-27 | [TT-8793] Fixing Pump 1.8 bugs | `partially_missed_before_hardening`: current SW-REQ-035 already pinned selective per-org collection routing, while SW-REQ-059 broadly pinned mixed aggregate writes but did not prove the second average-update upsert kept the mixed target. Hardened with DEFECT-16 for MongoSelective, DEFECT-17 plus SW-REQ-084 for MongoAggregate mixed average-update routing, SW-REQ-059 output-cardinality evidence, and nonzero request-time assertions in the live Mongo aggregate mixed-collection test. |
| ca22ae4 | 2023-03-16 | TT-8313 Hybrid pump refactor | `partially_missed_before_hardening`: this was not only a refactor; it fixed Hybrid init/connect paths that previously called log.Fatal for recoverable missing-connection-string or MDCB connection failures. Hardened with DEFECT-18 and SW-REQ-085 (`process_exit_on_recoverable` + `errors_propagated`) using existing Hybrid init/connect tests. Residual Hybrid risks remain KI-backed under `hybrid-getdialfn-leaks-conn-on-handshake-fail`, `hybrid-rpc-retry-duration-not-deadline-bounded`, `tls-insecure-skip-verify-allowed`, `external-pump-endpoints-no-ssrf-allowlist`, and `pump-no-timeout-can-block-purge-cycle`. |
| 7fa0754 | 2023-03-06 | [TT-7820] fix aggregate graph pump sharding and errors | `missed_before_hardening`: SW-REQ-043 prose required day-slice routing and selected-table upserts, but its FRETish model did not carry the shard-target invariant and the sharded test only asserted non-empty shards. Hardened with DEFECT-19 and SW-REQ-086 (`routing_target_consistent` + `output_cardinality_bounded`), plus stricter sharded evidence proving representative rows land exactly once in both selected shards while the base table remains absent. The same commit's legacy raw GraphRecord parse-error refactor is now superseded by the current GraphQLStats projection model under SW-REQ-013. |
| 17072c0 | 2023-02-21 | [TT-7977] fix: include RootFields in graph mongo and sql pumps | `covered_after_hardening`: SW-REQ-013 prose and current tests already preserve `GraphQLStats.RootFields`, but the requirement was informal and had no MC/DC-bearing RootFields variable. Hardened with DEFECT-20 and SW-REQ-087 (`structured_projection_preserved`) so RootFields projection now has explicit FRETish variables and code-annotated witness rows; this pass also tightened SW-REQ-037/SW-REQ-042/SW-REQ-087 wording and DEFECT-20 evidence so downstream Graph Mongo, Graph SQL, aggregate, and Graph SQL aggregate persistence are named in the closure. |
| e2f277a | 2023-01-12 | TT-7216 Decode Option For Raw Request/Response | `partially_missed_before_hardening`: SYS-REQ-011 already modeled request/response decode toggles and current tests covered nominal decode, but there was no SW-level filterData child and the malformed-base64 test was using deprecated global flags instead of the per-pump toggles. Hardened with DEFECT-21 and SW-REQ-088 (`encoding_aware`), fixed the negative test setup, and kept malformed-base64 silent no-op as KI-backed debt under `filterdata-base64-decode-silent-noop`; malformed-input tests now document KI reachability/obligation evidence rather than satisfied decode MC/DC rows. |
| ca44921 | 2022-11-23 | [TT-5426] Updating timestamp of every record in Demo Mode | `missed_before_hardening`: demo generation was traced to SW-REQ-009, which covers analytics field accessor determinism rather than synthetic demo timestamps, and current tests only checked broad past/future ranges. Hardened with DEFECT-22 and SW-REQ-089 (`temporal_window_inclusive`), moving GenerateDemoData/WriteDemoData trace ownership and adding boundary evidence that records in a generated hour receive matching, spaced TimeStamp/date-part values. |
| 5a25a2e | 2022-11-18 | [TT-5429] Tyk Pump Ignore Fields | `covered_today`: current SYS-REQ-016/SW-REQ-076 model the operator-listed `ignore_fields` contract by JSON tag, including per-pump wiring through `filterData` and `AnalyticsRecord.RemoveIgnoredFields`; SW-REQ-076 has full spec/code MC/DC via `TestIgnoreFieldsFilterData` plus direct `RemoveIgnoredFields` error-path evidence. Cleaned stale test/docs ownership so `TestAnalyticsRecord_RemoveIgnoredFields` points to SW-REQ-076 instead of SW-REQ-009. |
| 8e42170 | 2022-11-03 | [TT-6012] fix edge case where query is to unresolved subgraph schema | `covered_after_hardening`: the historical failure was in the legacy raw GraphQL parser for federation/subgraph `_entities` selections, while current code intentionally projects GraphRecord from structured `GraphQLStats` instead of reparsing raw request/response/schema payloads. Hardened with DEFECT-23 by making SW-REQ-013 explicit about GraphQLStats-authoritative projection and adding unresolved-subgraph legacy payload evidence that structured projection still succeeds. This pass also tightened SW-REQ-037 and DEFECT-23 so the same commit's empty raw request/response/schema Graph Mongo behavior is explicitly covered by GraphQLStats-backed Graph Mongo write evidence. No static signal added because the legacy parser path is no longer present in current production code. |
| d13b62e | 2022-11-02 | [TT-506] Self-Healing when hitting 16mb and configurable aggregation per time | `covered_after_hardening`: current proof had SW-REQ-058 for the broad Mongo aggregate window policy and SW-REQ-062 for max-document-size self-healing, but docs overclaimed the skipped 16 MiB integration test, the valid/default/handoff window branches were not separately modeled, and retry behavior was backed only by predicate/AST evidence. Hardened with DEFECT-24, child requirements SW-REQ-107/SW-REQ-108/SW-REQ-109, corrected variable metadata for the actual size-error strings, bounded timestamp-reset evidence, AST handoff/retry-wiring evidence, and fake-store behavioral tests that verify first size-error retry success plus floor-at-1 error propagation without generating a huge aggregate. |
| 84f3855 | 2022-11-02 | TT-3067 Add ssl_insecure_skip_verify with Elasticsearch Pump | `covered_after_hardening`: current SW-REQ-068 models Elasticsearch custom TLS setup through shared `NewTLSConfig`, and SW-REQ-016 covers the shared TLS helper. Hardened with DEFECT-25 by attaching `tls_verification_explicit` directly to SW-REQ-068, annotating the zero-default TLS test for Elasticsearch `SSLInsecureSkipVerify`, adding ES env obligation evidence for `SSLINSECURESKIPVERIFY`, and adding a structural handoff witness that `getOperator` passes cert/key/CA/skip-verify fields to `NewTLSConfig`. Strict production certificate validation remains KI-backed debt under `tls-insecure-skip-verify-allowed`; the API-key+TLS transport overwrite is tracked by KI `elasticsearch-api-key-auth-dropped-when-use-ssl`. |
| ff7574e | 2022-10-25 | [TT-6012] mongo graph records ignore max_document_size_bytes | `covered_after_hardening`: current code already preserves oversized graph raw payloads in graph-mode `MongoPump.AccumulateSet`, but SW-REQ-037 did not explicitly own the exception to the standard Mongo `MaxDocumentSizeBytes` rewrite, and the historical regression test used a legacy graph tag that no longer marks records as GraphQL. Hardened with DEFECT-26, a Graph Mongo `boundary` obligation, and repaired GraphQLStats-backed evidence that oversized graph records retain exact RawRequest/RawResponse values while non-GraphQLStats records are excluded from graph-mode accumulation. |
| 155b05a | 2022-10-18 | TT-6799 Prometheus pump disable metric families | `covered_after_hardening`: existing tests exercised `DisabledMetrics`, but SW-REQ-024 only formalized default scrape path behavior and did not define the exact built-in family suppression contract. Hardened with DEFECT-27, SW-REQ-090, a direct `initBaseMetrics` implementation trace, local obligation `metric_family_disable_gate`, MC/DC witnesses for disabled-family absence, exact evidence for all five built-in family names plus isolated-registry registration probes and the unknown-name boundary, and a custom-metric boundary test proving `disabled_metrics` does not suppress operator-defined custom metrics. |
| 97bbecc | 2022-10-18 | TT-6482 Histogram type label validation | `covered_after_hardening`: current tests covered Prometheus histogram type-label insertion/deduplication, but SW-REQ-024 did not model histogram label schema stability and tests did not assert the exact final label order for every boundary. Hardened with DEFECT-28, SW-REQ-091, direct implementation traces for `InitVec`/`ensureLabels`/`Observe`, local obligation `metric_label_schema_stable`, MC/DC witnesses for histogram schema normalization, exact-label evidence for missing, existing, middle-position, duplicate, empty, and counter cases, and `InitVec` integration evidence that normalization happens before registration. |
| 4c490cb | 2022-10-13 | TT-6550 Document size is mis-calculated in Mongo Selective Pump | `covered_after_hardening`: SW-REQ-035 mentioned skipping documents over `MaxDocumentSizeBytes`, but did not make the exact MongoSelective size formula first-class. Hardened with DEFECT-29, SW-REQ-092, a direct `getItemSizeBytes` implementation trace, local obligation `document_size_accounting_exact`, signal rule `go.mongo-document-size-counts-raw-request-twice` in `signal-rules/tyk-pump-phase-n.yaml`, and direct evidence that `RawRequest` and `RawResponse` are each counted once with the 1024-byte metadata allowance and a strict greater-than threshold. The same review found current KI `mongo-selective-final-skipped-record-drops-pending-batch`, because a final skipped oversize item prevents flushing a pending valid batch; no product fix was made. |
| c54eed3 | 2022-08-23 | ignore graph analytics for mongo pump(s) | `covered_after_hardening`: the standard-Mongo GraphQL exclusion side is captured as fixed historical defect `DEFECT-14` under SW-REQ-082, while the current `AggregateData` graph/MCP partition was only incidentally tested under SW-REQ-011 monotonic-counter evidence. Hardened with SW-REQ-093 (`output_cardinality_bounded`) so REST aggregates explicitly exclude GraphQL/MCP records while preserving ordinary REST records, including the same-organisation case where a graph record would otherwise inflate REST hit counts. Follow-up hardening also made SW-REQ-082 cover standard-mode retention for current GraphQLStats records and legacy tag-only records. No additional DEFECT was created for the aggregate proof-model hardening; unfixed live issues remain KnownIssue records. |
| 1cdc76c | 2022-08-19 | TT-6343 Fixing mem address pointer in prom custom metrics | `missed_before_hardening`: current tests exercised `InitCustomMetrics`, but only asserted the count of appended custom metrics, so the historical custom metric identity-collapse bug could recur while preserving length and losing metric names, labels, or collectors. Hardened with DEFECT-30, SW-REQ-094, local obligation `custom_metric_identity_preserved`, exact backing-entry identity/name/label/type/enabled/aggregate-observation/vector evidence for multiple valid custom metrics, and invalid-sibling nonblocking evidence in both invalid-before-valid and valid-before-invalid order. No broad `&range variable` signal was added because current Go 1.25 range semantics make that syntax-level detector weaker than the product-level evidence. |
| df6d589 | 2022-06-22 | TT-5776 Racy filterData func bugfix | `missed_before_hardening`: current proof modeled filter correctness and per-backend error/timeout independence, but not the TT-5776 invariant that per-pump `filterData` transforms must not mutate the shared decoded batch seen by sibling pumps. Hardened with DEFECT-31, SW-REQ-095, local obligation `per_backend_input_isolation`, catalogue obligation `shared_state_synchronized`, and direct evidence that per-backend filtering, omit-detailed-recording, trimming, ignore-fields removal, and raw payload decoding return the backend-specific view while preserving the original dispatch slice length, order, and values. No static signal was added in this slice because the risky shape is a broad slice-alias-and-write pattern; deterministic product-level regression tests plus focused race runs are higher signal. |
| 2e26ba7 | 2022-06-10 | TT5516 avoid double discard of aggregations | `covered_after_hardening`: current tests already exercised the Mongo mixed-collection scenario, but proof had only tied it to mixed routing and average-update collection identity. Hardened with DEFECT-32, SW-REQ-096, local obligation `aggregate_dimension_retention`, and live Mongo evidence that a writer configured with `ignore_aggregations=["apikeys"]` completes its later write, omits its ignored `apikey1`, and does not remove an existing `apikey2` dimension contributed by a non-ignoring writer. |
| 224f919 | 2022-05-17 | TT-5302 Mongo pump indexes on DocumentDB | `covered_after_hardening`: current code already restricts the collection-exists shortcut to StandardMongo, but proof only explicitly modeled aggregate index lifecycle and left standard/selective DocumentDB compatibility implicit. Hardened with DEFECT-33, SW-REQ-097/098/099, catalogue obligations `backend_ddl_valid` and `index_definition_matches_query`, and fake-store evidence that standard, selective, and aggregate DocumentDB index ensure paths do not call the collection-exists probe, honor `omit_index_creation`, and attempt exact foreground index definitions including key order, TTL=0 expiry metadata, and non-TTL ordinary-index metadata. |
| c0bc0e2 | 2022-05-11 | TT-4699 analytics serialization | `covered_after_hardening`: feature-surface commit, not a fixed historical defect. Existing proof already modeled the introduced msgpack/protobuf codecs through SW-REQ-008, INT-REQ-001, INT-REQ-003, and SYS-REQ-031, including suffix selection, round-trip behavior, protobuf golden snapshots, GraphQL/MCP protobuf mappings, and gateway encoding assumptions. Hardened the weaker purge-keyspace side with local obligation `wire_format_suffix_dispatch` on SW-REQ-001 plus `TestStartPurgeLoopDrainsEverySerializerSuffixForEveryAnalyticsKey`, proving the base key and `_0.._9` shard keys are crossed with every registered serializer suffix before dispatch. This is obligation evidence, not the `purge_tick -> records_dispatched` MC/DC witness. Residual live serializer risks are KI-backed (`serializer-protobuf-loses-city-names`, `serializer-protobuf-loses-graphql-error-path`, `protobuf-decode-nil-submessage-panic`, `preprocess-decode-error-leaves-nil-hole-in-keys`, `temporal-storage-wire-format-unversioned`), so no DEFECT is created for this feature row. |
| 14ccaba | 2020-12-22 | [TT-695] Mongo pumps Timeout | `covered_after_hardening`: TT-695 fixed two behaviors: Mongo connection timeout configuration was applied too late / replaced by hard-coded dial timeout, and standard Mongo did not wait for insert goroutines/errors. Hardened with DEFECT-34, local obligation `backend_connection_timeout_propagated`, SW-REQ-034/035/036 timeout-propagation text, source-level `ConnectionTimeout: m.timeout` and `pmp.Timeout`-to-`SetTimeout`-before-`Init(pmp.Meta)` witnesses, and SW-REQ-034 `external_call_failure_observable` evidence via `TestMongoPump_WriteData_InsertErr`. The source-level timeout witnesses are obligation evidence, not MC/DC rows for the Mongo write-routing formulas. Current Mongo caller-context/write-query timeout debt remains open as KIs (`mongo-pump-ignores-caller-context`, `mongo-aggregate-last-document-query-ignores-timeout`, `pump-no-timeout-can-block-purge-cycle`), and standard Mongo multi-batch insert-error goroutine-lifetime debt remains open under `mongo-standard-insert-error-double-send-goroutine-leak`; neither is claimed fixed. |
| 34e1a2c | 2020-08-19 | Fix omit_detailed_recording on syslog pump | `missed_before_hardening`: current proof covered generic `filterData` redaction but not the syslog backend-output boundary that historically lacked `CommonPumpConfig`. Hardened with DEFECT-35, SW-REQ-050 satisfying SYS-REQ-015, obligation `per_backend_privacy_transform_applied`, and `TestSyslogPump_OmitDetailedRecordingRedactsForwardedPayloads` using a real syslog UDP message after the core per-pump privacy transform to prove empty raw payload fields and original-byte absence, with exact code-only MC/DC rows for the privacy true row and UDP transport row. |
| 0adb849 | 2020-08-07 | [TN-6] Fix base64 ES decoding | `missed_before_hardening`: TN-6's successful `decode_base64` contract is now owned by SW-REQ-068 and DEFECT-36. The regression witness proves `raw_request`/`raw_response` are decoded plaintext strings, not `[]byte` fields that JSON can re-encode; malformed decode input remains an open KI `elasticsearch-decode-base64-errors-silent-empty`, not claimed fixed. |
| 402dab8 | 2020-06-23 | Fix health endpoint to be published outside server | `missed_before_hardening`: now covered by DEFECT-37 and SW-REQ-032 `listener_bind_scope_external`; the obligation witness fails if `ServeHealthCheck` returns to `localhost:<port>` or another loopback-only bind. |
| 416d1c7 | 2020-05-26 | fixing b64 decoding | `missed_before_hardening`: same defect class as 0adb849 and now covered by DEFECT-36: successful Elasticsearch `decode_base64` mapping is specified and witnessed as decoded text, with the malformed-input gap tracked separately as KI `elasticsearch-decode-base64-errors-silent-empty`. |
| 6a8ab73 | 2020-03-12 | Ignore index exists error | `missed_before_hardening`: now covered by DEFECT-38 and `idempotent_schema_setup`; MongoSelective same-key/different-name `logBrowserIndex` conflicts are witnessed as nil, unrelated index errors still propagate, and standard Mongo non-StandardMongo recurrence remains KI-backed under `mongo-standard-logbrowser-compatible-index-conflict`. The selective fake-store tests are obligation evidence, not MC/DC rows for the per-org routing formula. |
| 58da62f | 2019-11-19 | Handle unsupported MongoDB characters | `missed_before_hardening`: now covered by DEFECT-39 and `backend_field_key_safe`; dotted tracked REST endpoint paths are witnessed as encoded in `Endpoints` map keys and Mongo update keys, while raw path identifiers remain preserved. |
| 8bfdb36 | 2019-11-05 | Calculate document size correctly and do not skip useful data | `missed_before_hardening`: now covered by DEFECT-40 and `document_size_accounting_exact`; standard Mongo size accounting counts RawRequest and RawResponse exactly once with a 1024-byte metadata allowance, and over-threshold records are retained with raw bodies rewritten. These document-size witnesses are obligation evidence, not MC/DC rows for the standard Mongo MCP-filter formula. |
| aa7a88e | 2019-10-03 | Fix selective pump to not add TCP records | `missed_before_hardening`: now covered by DEFECT-41 under SW-REQ-035; the regression witness proves ResponseCode -1 TCP/error records are excluded from selective Mongo batches while ordinary HTTP records for the same org remain routed to the per-org collection. This TCP/error exclusion witness is obligation evidence, not an MC/DC row for the per-org routing formula. |
| 3bb755d | 2019-08-23 | Elasticsearch analytics publishes alias | `missed_before_hardening`: now covered by DEFECT-42 and SW-REQ-100; the regression witnesses prove `getMapping` emits the Elasticsearch `"alias"` field from `AnalyticsRecord.Alias` and does not synthesize an alias when the source field is empty. |
| 51af27d | 2019-08-13 | Fix influx pump write loop duplication | `missed_before_hardening`: now covered by DEFECT-43 and SW-REQ-101; the Influx v1 round-trip witness proves a three-record purge produces one backend /write request carrying three line-protocol rows, closing the historical cumulative-prefix write-loop failure mode. |
| d4d1cf7 | 2019-08-08 | Restore lists data to mongo aggregate mixed collection | `missed_before_hardening`: now covered by DEFECT-44 and SW-REQ-102; BSON evidence proves the restored Lists families survive persistence encoding, and the mixed aggregate regression witness reads aggregate documents back from Mongo and proves the `Lists.APIKeys` projection survives with identifier and hit-count data. |
| c02a2cb | 2019-07-16 | Include HTTP 400 in aggregate error count | `covered_after_hardening`: DEFECT-45 + SW-REQ-103 make the HTTP error boundary explicit; `TestAggregateData_ResponseCode400CountsAsErrorBoundary` proves 400 and 500 increment error counters while 399 does not, and signal rule `go.aggregate-http-error-threshold-strict-greater-than` catches the historical strict-threshold pattern. |

## Existing Proof Records That Already Encode Lessons Learned

Problem reports:

- DEFECT-1: MCP Mongo aggregate cross-API document merge.
- DEFECT-2: Syslog pump log fragmentation on multiline raw_request/raw_response.
- DEFECT-3: SQL table-sharding pump skipped schema migration of sharded tables.
- DEFECT-4: SQL pump panic when sharding enabled and all records skipped.
- DEFECT-7: Sharded SQL tables were created without the expected indexes.
- DEFECT-5: Prometheus pump exposed full API keys as metric label values.
- DEFECT-6: Elasticsearch pump shutdown did not close ES clients.
- DEFECT-13: Standard Mongo WriteData used graph-only accumulation and dropped ordinary records.
- DEFECT-14: Standard Mongo excluded GraphQL-classified records.
- DEFECT-15: Prometheus aggregation split label values containing the internal separator.
- DEFECT-16: Mongo selective writes targeted the default collection.
- DEFECT-17: Mongo aggregate mixed average update targeted the per-org collection.
- DEFECT-18: Hybrid init used log.Fatal for recoverable MDCB startup failures.
- DEFECT-19: Graph SQL aggregate sharded writes targeted the wrong table.
- DEFECT-20: Graph pumps dropped GraphQL root field dimensions.
- DEFECT-21: Raw request and response payload decode was not configurable per pump.
- DEFECT-22: Demo mode records used wall-clock timestamps instead of synthetic generated-hour timestamps.
- DEFECT-23: Legacy GraphQL schema parsing failed unresolved subgraph entity selections.
- DEFECT-24: Mongo aggregate documents exceeded backend max size without self-healing.
- DEFECT-25: Elasticsearch pump lacked explicit TLS skip-verify and client certificate controls.
- DEFECT-26: Graph Mongo oversized raw payloads were rewritten instead of preserved.
- DEFECT-27: Prometheus disabled metric families were not specified as a contract.
- DEFECT-28: Prometheus histogram type label schema was unstable.
- DEFECT-29: MongoSelective document-size accounting counted the wrong payload fields.
- DEFECT-30: Prometheus custom metric registration reused one pointer identity.
- DEFECT-31: filterData mutated the shared fan-out batch.
- DEFECT-32: Mongo aggregate ignored-dimension discard removed surviving dimensions.
- DEFECT-33: Mongo DocumentDB index ensure used collection-listing shortcut.
- DEFECT-34: Mongo pump timeout configuration was not applied to connection setup.
- DEFECT-35: Syslog pump did not inherit omit-detailed-recording configuration.
- DEFECT-36: Elasticsearch decode_base64 stored decoded payload bytes as binary.
- DEFECT-37: Health endpoint was bound to localhost only.
- DEFECT-38: Mongo logBrowserIndex rename conflict was treated as schema setup failure.
- DEFECT-39: Dotted endpoint paths corrupted Mongo aggregate endpoint dimensions.
- DEFECT-40: Standard Mongo document-size guard counted RawRequest twice and skipped useful records.
- DEFECT-41: MongoSelective wrote TCP error records to per-organisation collections.
- DEFECT-42: Elasticsearch analytics mapping omitted alias.
- DEFECT-43: Influx v1 wrote cumulative batches inside the record loop.
- DEFECT-44: Mongo aggregate mixed collection lost Lists projection data.
- DEFECT-45: REST aggregate omitted HTTP 400 from error counters.
- DEFECT-46: Graph pumps skipped pump-specific environment overrides.
- DEFECT-47: Kinesis KMS encrypted stream with missing KeyId was misclassified as a different configured key.
- DEFECT-48: FIPS-labelled Docker images were wired to standard build IDs.

Newer requirements introduced or strengthened by the later history pass:

- SYS-REQ-036: FIPS-labelled release artifacts use FIPS build/package/base-image variants.
- SW-REQ-103: REST aggregate HTTP error boundary includes HTTP 400 and above.
- SW-REQ-104: Gateway-only latency is projected into StatsD and Prometheus metrics.
- SW-REQ-105: Kinesis KMS stream state is reconciled against the configured key.
- SW-REQ-106: Kafka `batch_bytes` is applied to `kafka.WriterConfig.BatchBytes`.
- SW-REQ-107/SW-REQ-108/SW-REQ-109: Mongo aggregate configured-window valid/default/handoff behavior.

Design/review records for additive, policy, superseded, or split-history rows:

- `REVIEW-bad4cd3-mcp-analytics`, `REVIEW-25683f2-kinesis-feature`, `REVIEW-bf9e7e7-sqs-support`, `REVIEW-8ca8646-resurface-backend`, `REVIEW-33d9f48-kafka-batch-bytes`, and `REVIEW-ebd5a6c-gateway-latency-metrics` cover additive backend/metric feature rows where the hardening is requirements/evidence/KI debt rather than a fixed historical DEFECT.
- `REVIEW-956b66a-mongo-driver-default`, `REVIEW-775f8e3-sqlite-support-removal`, and `REVIEW-206c1d0-fips-release-artifacts` cover support-matrix or release-artifact policy rows.
- `REVIEW-0596e82-syslog-fragmentation`, `REVIEW-5965206-50e5f51-kinesis-kms`, `REVIEW-0ed84b9-mongo-graph-inclusion`, `REVIEW-1fd5ba9-mongo-collection-targets`, `REVIEW-7fa0754-graph-sql-aggregate-shard-target`, and `REVIEW-fbcb614-sql-empty-shard-batch` preserve split/superseded fix history or caught-regression reasoning where one commit introduced, superseded, or validated behavior later pinned by a DEFECT or requirement.
- The remaining `REVIEW-*` records under `.proof/reviews/` attach the detailed commit-level reasoning for strengthened obligations and witnesses across SQL, GraphQL, Prometheus, Kafka, Splunk, Mongo, Elasticsearch, storage, demo data, omit-config, and raw-payload decode slices.

KnownIssues/risks relevant to historical fix classes:

- pump-writedata-swallows-per-batch-errors.
- write-failure-after-pop-loses-records.
- pump-no-timeout-can-block-purge-cycle.
- mongo-pump-ignores-caller-context.
- storage-pop-expire-non-atomic-data-loss.
- storage-retry-maxelapsed-zero-is-unbounded.
- mapstructure-decode-silently-drops-unknown-keys.
- no-panic-recovery-in-exec-pump-writing.
- pump-fanout-panic-not-recovered.
- prometheus-metric-maps-race.
- metrics-label-cardinality-unbounded.
- prometheus-init-mutates-default-mux.
- filterdata-base64-decode-silent-noop.
- analytics-timestamp-timezone-convention-unpinned.
- sql-default-migration-today-only.
- sql-aggregate-sharded-shared-db-race.
- sql-aggregate-atomicity-fault-injection-missing.
- sql-aggregate-no-deadlock-retry.
- sql-batch-size-zero-infinite-loop.
- external-pump-endpoints-no-ssrf-allowlist.
- influx-v1-unbounded-reconnect-recursion.
- logzio-segment-no-shutdown-flush.
- sql-aggregate-sharded-upsert-targets-base-table.
- docs-sqlite-still-listed-as-supported-sql-type.
- graph-mcp-sql-sharded-indexes-untracked.
- mcp-sql-aggregate-mysql-create-index-syntax-broken.
- mongo-standard-insert-error-double-send-goroutine-leak.
- mongo-standard-logbrowser-compatible-index-conflict.
- kinesis-batch-size-over-aws-putrecords-limit.
- kinesis-putrecords-per-record-failures-return-nil.
- kinesis-random-partition-key-not-idempotent.
- kinesis-splitintobatches-zero-infinite-loop.
- sqs-malformed-record-sends-empty-entry.
- resurface-writedata-blocks-on-queue-full.
- resurface-worker-errors-swallowed.
- resurface-disabled-writedata-closes-channel.
- docs-kinesis-env-streamname-typo.
- elasticsearch-api-key-auth-dropped-when-use-ssl.
- elasticsearch-decode-base64-errors-silent-empty.
- graph-sql-aggregate-atomicity-fault-injection-missing.
- graph-sql-aggregate-migrate-sharded-tables-ignored.
- kafka-logfatal-on-init-mech-and-timeout.
- mongo-aggregate-last-document-query-ignores-timeout.
- mongo-selective-final-skipped-record-drops-pending-batch.
- preprocess-decode-error-leaves-nil-hole-in-keys.
- protobuf-decode-nil-submessage-panic.
- resurface-maprawdata-empty-request-panic.
- serializer-protobuf-loses-city-names.
- serializer-protobuf-loses-graphql-error-path.
- sql-aggregate-background-index-concurrency-unbounded.
- sql-aggregate-mysql-create-index-if-not-exists-unsupported.
- sql-schema-no-version-policy.
- temporal-storage-wire-format-unversioned.
- tls-insecure-skip-verify-allowed.

## Remaining Follow-Ups

These are not product-code fixes. Items marked completed are retained as a
review trail; open items are follow-ups for future proof-modeling,
KnownIssue/problem-report work, or conversion after an upstream fix lands here.

1. Close or convert the active KI for TT-16778 / 140ef71 after merging the
   upstream product-code fix.
   - Current branch disposition: KnownIssue
     `sql-aggregate-sharded-upsert-targets-base-table`.
   - Affected requirements: SW-REQ-064, SW-REQ-067.
   - Defect class: wrong insert target for sharded aggregate upsert.
   - Proof hardening added: local obligation `routing_target_consistent` and
     code signal `go.sql-aggregate-upsert-without-table-target`.
   - Once the branch contains commit 140ef71, convert the history to a DEFECT
     record and bind the upstream regression witnesses
     SQLAggregateDoAggregatedWriting_UsesProvidedTable_SQLite and
     SQLAggregateDoAggregatedWriting_Sharded.

2. Influx v1 duplicate/cumulative write contract — completed in DEFECT-43.
   - Historical source: 51af27d.
   - Added requirement: SW-REQ-101.
   - Evidence: the multi-record Influx v1 httptest witness proves one
     completed backend write per purge batch with one line-protocol row per
     input record, not cumulative writes inside the record loop.

3. Strengthen Mongo field-name sanitization for aggregate dimensions.
   - Historical source: 58da62f.
   - Status: completed in DEFECT-39 via `backend_field_key_safe` on
     SW-REQ-011 plus aggregate update-key evidence for dotted tracked REST
     endpoint paths.

4. Strengthen Graph unresolved-schema behavior.
   - Historical source: 8e42170.
   - Status: completed in DEFECT-23 via SW-REQ-013
     `malformed_recovers_or_errors_loudly` / structured projection evidence
     and SW-REQ-037 Graph Mongo write evidence for structured GraphQLStats
     records without legacy raw request/response/schema payloads.

5. Prometheus tracking path and label semantics — completed with residual KI debt.
   - Historical sources: 69f5f4a, 8173c7e, 1cdc76c.
   - Added/strengthened requirements: SW-REQ-078, SW-REQ-079, SW-REQ-083,
     SW-REQ-094.
   - Evidence: tracking-path witnesses cover unknown-vs-preserved path policy,
     separator-bearing aggregate label values preserve tuple boundaries, and
     custom metric backing entries preserve distinct names, labels, types,
     enabled flags, and collectors.
   - Residual broad label-cardinality risk remains KI-backed under
     `metrics-label-cardinality-unbounded`.

6. Health endpoint bind-address contract — completed in DEFECT-37.
   - Historical source: 402dab8.
   - Added obligation: listener_bind_scope_external.
   - Evidence: `TestServeHealthCheck_BindsExternalInterface` verifies the
     production listener address binds `:<port>` and rejects loopback-only
     host literals.

7. Mongo/DocumentDB index idempotency — partially completed in DEFECT-33 and DEFECT-38.
   - Historical sources: 6a8ab73, 224f919.
   - Added obligations: `backend_ddl_valid`, `index_definition_matches_query`,
     and `idempotent_schema_setup`.
   - Evidence: DocumentDB index attempts are covered by SW-REQ-097/098/099;
     MongoSelective compatible logBrowserIndex rename conflicts are idempotent;
     standard Mongo non-StandardMongo compatible conflicts remain tracked under
     KI `mongo-standard-logbrowser-compatible-index-conflict`.

## Conclusion

The current proof model is useful for preventing recurrence of many known
product defect classes. It is also useful for making unfixed behavior visible as
KnownIssue or obligation debt instead of allowing silent overclaims.

It is not a historical oracle. If a behavior was never specified, not linked to
an obligation, not represented by code signals, and not exercised by evidence,
proof would not have found it automatically. The right way to improve from the
history review is to convert important historical failures into explicit
requirements, obligations, regression witnesses, DEFECT records for fixed bugs,
or KnownIssues for still-open behavior.
