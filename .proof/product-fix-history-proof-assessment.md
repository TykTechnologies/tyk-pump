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
| 19fb73d | 2025-10-23 | SQL table-sharding migration skipped sharded tables. | `caught`: DEFECT-3 and review `REVIEW-19fb73d-sql-shard-migration` tie TT-13166 to SW-REQ-040, INT-REQ-007 migration witnesses, `TestHandleTableMigration`, and `TestMigrateAllShardedTables`; residual default-mode prior-day shard, Graph SQL aggregate startup migration, and schema-version policy gaps remain KI-tracked. |
| 0596e82 / df62011 | 2025-08-15/16 | Syslog multiline raw request/response fragmented one analytics record into multiple syslog lines. | Covered now by DEFECT-2, SW-REQ-050, and encoding_safety evidence. |
| fbcb614 | 2024-11-27 | SQL pump panicked when table sharding was enabled and all records were skipped. | Covered now by DEFECT-4, SW-REQ-040, and boundary evidence. |
| 544ccb3 | 2024-12-05 | Sharded SQL pumps did not create needed indexes on shard tables. | Mostly covered by SQL sharding/index requirements and obligations. Some backend-specific DDL behavior remains KI/debt where applicable. |
| 866bdc2 | 2026-02-09 | CA certificate configuration for Elasticsearch/Kafka/Splunk TLS clients. | `missed_before_hardening`: shared TLS parsing was covered, but per-backend positive CA-root attachment was weak. Hardened via `cert_chain_validated` on SW-REQ-016/021/048/068, CA `RootCAs` evidence in Common/Kafka/Splunk/Elasticsearch tests, and review `REVIEW-866bdc2-ca-cert`; operator `ssl_insecure_skip_verify` remains KI-tracked debt. |
| 1c20a08 | 2026-06-10 | RFC3339-compliant log format option. | `missed_before_hardening`: current branch lacks upstream TT-17281. Not treated as an active product bug because this branch only documents `text`/`json`; proof now pins current legacy timestamp behavior via SW-REQ-033, obligation `log_timestamp_format_declared`, and logger formatter evidence. |
| 8a614d6 | 2026-06-09 | Add original_path and listen_path observability fields. | `missed_before_hardening`: current branch lacks upstream TT-7519 fields, so no active KI. Hardened INT-REQ-003, SW-REQ-008, and SW-REQ-009 to require additive AnalyticsRecord fields to update schema/transformer/helper evidence explicitly. |

## Older Runtime/Product Fixes

| Commit | Date | Product issue | Current proof posture |
| --- | --- | --- | --- |
| d9d64dc | 2024-01-29 | Some GraphQL pumps did not read env configuration correctly. | Likely surfaced today through config-loading/config-schema obligations, but exact graph pump env key coverage should be checked. |
| 956b66a | 2024-01-31 | Default Mongo driver migration to mongo-go. | Mostly outside a single behavioral proof claim unless modeled as backend compatibility. Current Mongo reqs would catch functional regressions, not the driver policy itself. |
| 775f8e3 | 2025-02-26 | SQLite support removed from SQL pumps. | Docs/support matrix issues are partly covered by docs KIs. Runtime rejection is only caught if support matrix is modeled explicitly. |
| 35e1e74 | 2024-01-04 | Splunk backoff retry behavior. | Partially covered by retry budget/timeout/failure-observable obligations. Exact Splunk retry semantics depend on dedicated evidence. |
| 407c373 | 2024-01-26 | Migration to storage library changed storage/retry behavior. | Current storage requirements and KIs cover retry max elapsed, context/cancellation, pop/expire data loss, and wire-format compatibility. It would surface gaps, not guarantee all migration behavior. |
| 7fa0754 | 2023-03-06 | Graph aggregate sharding and graph record errors. | Mostly covered now by Graph SQL/Mongo requirements and temporal-window/routing obligations. Would not have been fully caught before graph requirements were decomposed. |
| 17072c0 | 2023-02-21 | RootFields missing from Graph Mongo/SQL pumps. | Current field-preservation and graph requirements should catch regression if tests are annotated. This was likely not discoverable before RootFields was specified. |
| 8e42170 | 2022-11-03 | Edge case for unresolved subgraph schema. | Current graph malformed-input/edge-case obligations are relevant. Exact coverage depends on whether unresolved-schema test evidence is attached. |
| d13b62e | 2022-11-02 | Mongo aggregate 16 MiB self-healing and configurable aggregation window. | Current Mongo aggregate requirements/KIs cover document-size bounding and self-healing better than before. Exact size-threshold arithmetic needs targeted tests. |
| ff7574e | 2022-10-25 | Mongo graph records ignored max_document_size_bytes. | Likely surfaced by input_size_bounded/document-size obligations today. Regression coverage should be checked for graph-specific path. |
| 4c490cb | 2022-10-13 | Mongo selective document size was miscalculated. | Same class as size-bound evidence; exact arithmetic requires targeted witness. |
| c54eed3 | 2022-08-23 | Mongo pumps needed to ignore graph analytics in standard paths. | Current graph/Mongo classification requirements likely catch this class. |
| 2e26ba7 | 2022-06-10 | Avoid double discard of aggregations. | Current filtering/aggregation requirements help, but exact duplicate-discard semantics need a regression witness. |
| 224f919 | 2022-05-17 | Mongo indexes on DocumentDB; omit-index creation compatibility. | Partially covered by Mongo index/config obligations. Backend-specific DocumentDB compatibility remains evidence-sensitive. |
| 14ccaba | 2020-12-22 | Mongo pumps timeout configurability. | Current proof surfaces related gaps strongly: mongo-pump-ignores-caller-context, pump-no-timeout-can-block-purge-cycle, per-pump timeout risks. |
| 34e1a2c | 2020-08-19 | omit_detailed_recording missing on syslog pump. | Current privacy requirement SYS-REQ-015 and filtering evidence should catch this class if syslog path is included in the witness. |
| 416d1c7 / 0adb849 | 2020-05/08 | Elasticsearch base64 decode handling. | Current base64 decode requirement and KI filterdata-base64-decode-silent-noop surface malformed/failed decode gaps, but ES-specific decode behavior needs exact evidence. |
| 402dab8 | 2020-06-23 | Health endpoint only available from localhost. | Current health requirements/KIs cover liveness/readiness/auth/rate-limit, but bind-address accessibility should be explicitly modeled if we want guaranteed catch. |
| 6a8ab73 | 2020-03-12 | Mongo index already-exists error should be ignored. | Current index/idempotency obligations are relevant; exact already-exists behavior needs a Mongo index regression test. |
| 58da62f | 2019-11-19 | Mongo aggregate field names with unsupported '.' characters corrupted data. | Current encoding/data-integrity obligations are relevant, but this exact field-name sanitization contract should be explicit to guarantee catch. |
| 8bfdb36 | 2019-11-05 | Mongo document size calculated incorrectly and useful data skipped. | Current document-size/input-bound obligations help. Exact skip behavior requires targeted evidence. |
| aa7a88e | 2019-10-03 | Mongo selective pump wrote TCP records incorrectly. | Current record-classification/filtering requirements likely catch if TCP/non-HTTP partition is witnessed. |
| 3bb755d | 2019-08-23 | Elasticsearch analytics missing alias field. | General field-preservation requirements help, but alias must be explicitly listed/traced to be guaranteed. |
| 51af27d | 2019-08-13 | Influx write was inside loop, causing duplicate/cumulative writes and severe slowness. | Current output-cardinality and error/backpressure obligations are adjacent, but this exact "one backend write per completed batch, not cumulative per record" contract is not guaranteed unless added. |
| d4d1cf7 | 2019-08-08 | Mongo aggregate mixed collection lost Lists data. | Current aggregate field-preservation should catch if Lists is represented in evidence. |
| c02a2cb | 2019-07-16 | HTTP 400 was not counted as an error in aggregates. | Current analytics aggregate requirements likely catch if response-code boundary evidence includes 400. |

## Reviewed Git Commit Set

The table below records the product/runtime git commits considered during the
history review. It is intentionally broader than the detailed assessment above,
so a later reviewer can see what was included and what was filtered out.

| Commit | Date | Subject | Review disposition |
| --- | --- | --- | --- |
| 140ef71 | 2026-06-17 | [TT-16778] Fix sharded aggregate SQL writes | `surfaced_as_debt`: upstream fixed, current branch not fixed. Added KI `sql-aggregate-sharded-upsert-targets-base-table`, local obligation `routing_target_consistent`, and code signal `go.sql-aggregate-upsert-without-table-target`. |
| 1c20a08 | 2026-06-10 | [TT-17281] Add RFC3339 compliant option for consistent Tyk Pump logs and field mapping | `missed_before_hardening`: upstream feature not present here. Hardened SW-REQ-033 with `log_timestamp_format_declared` and exact legacy timestamp-format witness so any later RFC3339/legacy contract change must update the spec/evidence explicitly. |
| 8a614d6 | 2026-06-09 | [TT-7519] add original_path and listen_path to observability | `missed_before_hardening`: upstream additive fields are absent here. Hardened serializer/interface/helper requirements so future additive record fields need explicit schema, transformer, and evidence updates. |
| be39ad3 | 2026-04-28 | [TT-17004] Fix MCP Mongo aggregate cross-API merge | `caught`: DEFECT-1 + SW-REQ-039 + monitored regression `TestMCPMongoAggregatePump_WriteData_PerAPIPartitioning`; proof MC/DC/evidence rows are covered. |
| bad4cd3 | 2026-04-24 | [TT-16809] Pump generate MCP analytics | `caught`: broad feature surface, not a DEFECT. Review `REVIEW-bad4cd3-mcp-analytics` ties current coverage to SW-REQ-012/014/038/039/044/045/069, serializer/interface evidence, and residual MCP KIs for ES non-bulk routing, SQL aggregate MySQL DDL/upsert, and Mongo aggregate atomicity debt. |
| 866bdc2 | 2026-02-09 | [TT-15674] Extend pumps config with ca cert option | `missed_before_hardening`: added `cert_chain_validated` obligations and per-backend valid-CA evidence for SW-REQ-016/021/048/068; skip-verify policy remains tracked by KI `tls-insecure-skip-verify-allowed`. |
| 19fb73d | 2025-10-23 | [TT-13166] regression when using sql table sharding pump | `caught`: DEFECT-3 + SW-REQ-040/INT-REQ-007 migration evidence cover the skipped-shard-migration regression; residual policy gaps remain under KIs `sql-default-migration-today-only`, `graph-sql-aggregate-migrate-sharded-tables-ignored`, and `sql-schema-no-version-policy`. |
| ebd5a6c | 2025-10-23 | [TT-14871] Expose Gateway-Only Latency in Tyk Metrics | Product metric field behavior. Requires explicit metric contract/evidence for guaranteed catch. |
| 50e5f51 | 2025-10-16 | [TT-14473] support for encrypted aws kinesis | Product security/config behavior. Relevant to TLS/encryption obligations. |
| 5965206 | 2025-10-13 | [TT-14473] support for encrypted aws kinesis | Product security/config behavior. Relevant to TLS/encryption obligations. |
| 33d9f48 | 2025-10-13 | [TT-15560] added batchbytes configs and unit testing for kafka | Product batching/config behavior. Relevant to input-size and batch-bound obligations. |
| 0596e82 | 2025-08-16 | [TT-15532] Alternative backward-compatible fix for syslog pump log fragmentation | Product defect. Covered by DEFECT-2. |
| df62011 | 2025-08-15 | Fix syslog pump log fragmentation issue | Superseded product defect fix. Final behavior covered by 0596e82/DEFECT-2. |
| 775f8e3 | 2025-02-26 | [TT-13341] Remove SQLite support from Tyk Pump | Product support-matrix change. Needs explicit support contract to enforce. |
| 544ccb3 | 2024-12-05 | TT-13421 create indexes on sharded sql pumps | Product defect/compatibility fix. Mostly covered by SQL index/sharding obligations. |
| fbcb614 | 2024-11-27 | TT-12780 prevent sql pump to panic when sharding enabled and skip api id is set | Product defect. Covered by DEFECT-4. |
| 25683f2 | 2024-08-01 | [TT-6671] AWS Kinesis pump | Product feature surface. Current Kinesis reqs/KIs cover some risks, not a historical defect. |
| 206c1d0 | 2024-06-06 | TT-12103 Adding FIPS Support | Product build/security mode. Excluded from detailed runtime-defect assessment unless FIPS behavior is modeled. |
| d9d64dc | 2024-01-29 | fix bug where some graphql pumps are not reading env properly | Product defect. Adjacent to config/env obligations. |
| 8ca8646 | 2024-01-26 | TT-10676 Upgrade Resurface Pump backend | Product backend behavior change. Relevant to Resurface queue/cancellation obligations. |
| 407c373 | 2024-01-26 | [TT-10520] Migrating from go-redis to storage library | Product storage/retry behavior. Current storage KIs capture several risks. |
| bf9e7e7 | 2024-01-24 | TT-10675 add SQS Pump Backend support | Product feature surface. Current SQS KIs cover batch-limit and partial-failure risks. |
| 35e1e74 | 2024-01-04 | TT-10564 Splunk backoff retry | Product retry behavior. Partially covered by retry budget/timeout obligations. |
| 6c14ba0 | 2023-11-08 | [TT-9476] Refactor graph pumps to use new GraphQLStats | Product graph behavior change. Relevant to graph requirements. |
| 9984a1e | 2023-10-30 | [TT-9476] add graphstats to analytics record proto | Product serialization/field behavior. Requires explicit field preservation evidence. |
| 7af2a45 | 2023-09-18 | [TT-100053] aggregate graph aggregate records by api_id | Product aggregation partitioning behavior. Relevant to graph aggregate requirements. |
| 657436f | 2023-08-29 | [TT-9468] New SQL Aggregate indexes | Product DB/index behavior. Relevant to SQL index obligations. |
| 69f5f4a | 2023-08-23 | TT-9873 Fix prometheus tracking path | Product metric-label/path defect. Needs explicit label/path evidence. |
| 3bf1f85 | 2023-08-23 | [TT-9855] fix index creation error on graph sql pump creation | Product DB/index defect. Relevant to Graph SQL index obligations. |
| 91dd8a0 | 2023-08-01 | [TT-9360] Changing Timeout from time.Duration to int | Product config/timeout behavior. Relevant to timeout parsing obligations. |
| 6820131 | 2023-06-12 | [TT-9126] Fix error log when omit_configfile option is enabled | Product startup/config observability. Lower assurance relevance. |
| 2d3c296 | 2023-05-25 | [TT-8884] added write data test for mongo pump and remove constraint | Product Mongo write behavior. Relevant to Mongo write evidence. |
| 0ed84b9 | 2023-05-17 | fix: include graph records in mongo pump | Product routing/classification defect. Relevant to graph/Mongo requirements. |
| 8173c7e | 2023-05-15 | TT-876 Fix/prometheus cardinality | Product observability/cardinality defect. Covered as KI/debt class. |
| 1fd5ba9 | 2023-04-27 | [TT-8793] Fixing Pump 1.8 bugs | Product Mongo collection-routing fixes. Relevant to Mongo collection/partition evidence. |
| ca22ae4 | 2023-03-16 | TT-8313 Hybrid pump refactor | Product backend behavior. Relevant to hybrid timeout/leak/KI obligations. |
| 7fa0754 | 2023-03-06 | [TT-7820] fix aggregate graph pump sharding and errors | Product graph/sharding defect. Mostly covered by graph requirements today. |
| 17072c0 | 2023-02-21 | [TT-7977] fix: include RootFields in graph mongo and sql pumps | Product field-preservation defect. Requires RootFields evidence. |
| e2f277a | 2023-01-12 | TT-7216 Decode Option For Raw Request/Response | Product privacy/decode feature. Current decode KIs cover malformed paths. |
| ca44921 | 2022-11-23 | [TT-5426] Updating timestamp of every record in Demo Mode | Product demo timestamp behavior. Lower runtime assurance relevance. |
| 5a25a2e | 2022-11-18 | [TT-5429] Tyk Pump Ignore Fields | Product privacy/filtering behavior. Covered by field-removal requirements. |
| 8e42170 | 2022-11-03 | [TT-6012] fix edge case where query is to unresolved subgraph schema | Product graph edge-case defect. Candidate for stronger explicit evidence. |
| d13b62e | 2022-11-02 | [TT-506] Self-Healing when hitting 16mb and configurable aggregation per time | Product Mongo aggregate size/self-healing behavior. Mostly modeled today. |
| 84f3855 | 2022-11-02 | TT-3067 Add ssl_insecure_skip_verify with Elasticsearch Pump | Product TLS security posture. Current tls-insecure-skip-verify KI captures risk. |
| ff7574e | 2022-10-25 | [TT-6012] mongo graph records ignore max_document_size_bytes | Product size-bound defect. Relevant to input-size obligations. |
| 155b05a | 2022-10-18 | TT-6799 Prometheus pump disable metric families | Product metrics config behavior. Relevant to metric exposition contract. |
| 97bbecc | 2022-10-18 | TT-6482 Histogram type label validation | Product metric label validation. Relevant to label schema/cardinality obligations. |
| 4c490cb | 2022-10-13 | TT-6550 Document size is mis-calculated in Mongo Selective Pump | Product size-bound defect. Candidate for explicit regression evidence. |
| c54eed3 | 2022-08-23 | ignore graph analytics for mongo pump(s) | Product classification/routing defect. Relevant to graph/Mongo reqs. |
| 1cdc76c | 2022-08-19 | TT-6343 Fixing mem address pointer in prom custom metrics | Product metric label/value defect. Candidate for explicit label-schema evidence. |
| df6d589 | 2022-06-22 | TT-5776 Racy filterData func bugfix | Product filtering/concurrency defect. Relevant to filterData/privacy obligations. |
| 2e26ba7 | 2022-06-10 | TT5516 avoid double discard of aggregations | Product aggregation/filtering defect. Candidate for explicit duplicate-discard evidence. |
| 224f919 | 2022-05-17 | TT-5302 Mongo pump indexes on DocumentDB | Product DB/index compatibility behavior. Partially covered by index obligations. |
| c0bc0e2 | 2022-05-11 | TT-4699 analytics serialization | Product serialization feature. Current serializer requirements cover codec behavior. |
| 14ccaba | 2020-12-22 | [TT-695] Mongo pumps Timeout | Product timeout behavior. Current timeout KIs strongly surface residual gaps. |
| 34e1a2c | 2020-08-19 | Fix omit_detailed_recording on syslog pump | Product privacy/filtering defect. Relevant to SYS-REQ-015. |
| 0adb849 | 2020-08-07 | [TN-6] Fix base64 ES decoding | Product decode behavior. Relevant to base64 decode requirements/KIs. |
| 402dab8 | 2020-06-23 | Fix health endpoint to be published outside server | Product health endpoint accessibility. Candidate for bind-address contract. |
| 416d1c7 | 2020-05-26 | fixing b64 decoding | Product decode behavior. Relevant to base64 decode requirements/KIs. |
| 6a8ab73 | 2020-03-12 | Ignore index exists error | Product DB/index idempotency defect. Candidate for index-idempotency evidence. |
| 58da62f | 2019-11-19 | Handle unsupported MongoDB characters | Product encoding/data-integrity defect. Candidate for backend-key encoding contract. |
| 8bfdb36 | 2019-11-05 | Calculate document size correctly and do not skip useful data | Product size-bound defect. Candidate for explicit size arithmetic evidence. |
| aa7a88e | 2019-10-03 | Fix selective pump to not add TCP records | Product classification/filtering defect. Relevant to Mongo selective requirements. |
| 3bb755d | 2019-08-23 | Elasticsearch analytics publishes alias | Product field-preservation defect. Needs alias field evidence. |
| 51af27d | 2019-08-13 | Fix influx pump write loop duplication | Product output cardinality/performance defect. Candidate for batch_write_once evidence. |
| d4d1cf7 | 2019-08-08 | Restore lists data to mongo aggregate mixed collection | Product field-preservation defect. Needs Lists evidence. |
| c02a2cb | 2019-07-16 | Include HTTP 400 in aggregate error count | Product boundary/counting defect. Needs response-code boundary evidence. |

## Existing Proof Records That Already Encode Lessons Learned

Problem reports:

- DEFECT-1: MCP Mongo aggregate cross-API document merge.
- DEFECT-2: Syslog pump log fragmentation on multiline raw_request/raw_response.
- DEFECT-3: SQL table-sharding pump skipped schema migration of sharded tables.
- DEFECT-4: SQL pump panic when sharding enabled and all records skipped.
- DEFECT-5: Prometheus pump exposed full API keys as metric label values.
- DEFECT-6: Elasticsearch pump shutdown did not close ES clients.

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

## Gaps To Consider Adding

These are not product-code fixes. They are candidates for future proof-modeling
or KnownIssue/problem-report work.

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

2. Strengthen Influx v1 duplicate/cumulative write contract.
   - Historical source: 51af27d.
   - Candidate obligation: output_cardinality_bounded or batch_write_once.
   - Desired evidence: N input records produce one completed backend write for
     the batch, not cumulative writes inside the record loop.

3. Strengthen Mongo field-name sanitization for aggregate dimensions.
   - Historical source: 58da62f.
   - Candidate obligation: encoding_safety / backend_key_encoding_safe.
   - Desired evidence: endpoint paths containing "." are preserved through a
     reversible or documented replacement policy.

4. Strengthen Graph unresolved-schema behavior.
   - Historical source: 8e42170.
   - Candidate obligation: malformed_recovers_or_errors_loudly / edge_case.
   - Desired evidence: unresolved subgraph schema does not panic and is routed
     according to documented graph semantics.

5. Strengthen Prometheus tracking path and label semantics.
   - Historical sources: 69f5f4a, 8173c7e, 1cdc76c.
   - Candidate obligations: output_cardinality_bounded, label_schema_stable.
   - Desired evidence: path normalization and custom metric label mapping do
     not expose pointer-like values, unbounded labels, or wrong path values.

6. Strengthen health endpoint bind-address contract.
   - Historical source: 402dab8.
   - Candidate obligation: externally_reachable_when_configured.
   - Desired evidence: configured listen address exposes the health endpoint as
     documented, not only localhost.

7. Strengthen Mongo/DocumentDB index idempotency.
   - Historical sources: 6a8ab73, 224f919.
   - Candidate obligation: idempotent_schema_setup.
   - Desired evidence: existing compatible index does not fail startup/write,
     and DocumentDB omit-index behavior is documented.

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
