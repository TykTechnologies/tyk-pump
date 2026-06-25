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
| 0596e82 / df62011 | 2025-08-15/16 | Syslog multiline raw request/response fragmented one analytics record into multiple syslog lines. | `caught`: DEFECT-2, SW-REQ-050, `encoding_safety`, and `output_cardinality_bounded` evidence cover the final backward-compatible behavior: legacy `map[...]` output with LF escaping in `raw_request`/`raw_response`; review `REVIEW-0596e82-syslog-fragmentation` records the superseded JSON approach in df62011. |
| fbcb614 | 2024-11-27 | SQL pump panicked when table sharding was enabled and all records were skipped. | `caught`: DEFECT-4, SW-REQ-040, and the `TestSQLWriteDataSharded/empty_keys` boundary witness cover the sharded empty-batch slice-bound regression; adjacent SQL-family KIs are linked in the defect disposition. |
| 544ccb3 | 2024-12-05 | Sharded SQL pumps did not create needed indexes on shard tables. | `missed_before_hardening`: standard SQL and SQL Aggregate shard-index behavior are now covered by DEFECT-7, `per_shard_index_created`, and Postgres shard-index witnesses; Graph/MCP SQL shard-index contracts and MySQL index DDL validity remain KI-backed debt (`graph-mcp-sql-sharded-indexes-untracked`, `sql-standard-mysql-create-index-if-not-exists-unsupported`, `mcp-sql-aggregate-mysql-create-index-syntax-broken`). |
| 866bdc2 | 2026-02-09 | CA certificate configuration for Elasticsearch/Kafka/Splunk TLS clients. | `missed_before_hardening`: shared TLS parsing was covered, but per-backend positive CA-root attachment was weak. Hardened via `cert_chain_validated` on SW-REQ-016/021/048/068, CA `RootCAs` evidence in Common/Kafka/Splunk/Elasticsearch tests, and review `REVIEW-866bdc2-ca-cert`; operator `ssl_insecure_skip_verify` remains KI-tracked debt. |
| 1c20a08 | 2026-06-10 | RFC3339-compliant log format option. | `missed_before_hardening`: current branch lacks upstream TT-17281. Not treated as an active product bug because this branch only documents `text`/`json`; proof now pins current legacy timestamp behavior via SW-REQ-033, obligation `log_timestamp_format_declared`, and logger formatter evidence. |
| 8a614d6 | 2026-06-09 | Add original_path and listen_path observability fields. | `missed_before_hardening`: current branch lacks upstream TT-7519 fields, so no active KI. Hardened INT-REQ-003, SW-REQ-008, and SW-REQ-009 to require additive AnalyticsRecord fields to update schema/transformer/helper evidence explicitly. |

## Older Runtime/Product Fixes

| Commit | Date | Product issue | Current proof posture |
| --- | --- | --- | --- |
| d9d64dc | 2024-01-29 | Some GraphQL pumps did not read env configuration correctly. | Likely surfaced today through config-loading/config-schema obligations, but exact graph pump env key coverage should be checked. |
| 956b66a | 2024-01-31 | Default Mongo driver migration to mongo-go. | Mostly outside a single behavioral proof claim unless modeled as backend compatibility. Current Mongo reqs would catch functional regressions, not the driver policy itself. |
| 775f8e3 | 2025-02-26 | SQLite support removed from SQL pumps. | `missed_before_hardening`: now modeled by `support_matrix_enforced` on SQL-family requirements with SQLite-rejection evidence through production Init/Dialect paths; external tyk-docs support-matrix drift remains KI-backed debt. |
| 35e1e74 | 2024-01-04 | Splunk backoff retry behavior. | `missed_before_hardening`: retry attempts and Splunk MaxRetries were partially covered, but request-body replay across retries was not explicit. Hardened with `request_body_replay_preserved` on SW-REQ-030, `retry_policy_explicit` on SW-REQ-048, a body-replay retry witness, and review `REVIEW-35e1e74-splunk-retry`; residual idempotency/body-size/deadline risks remain KI-backed debt. |
| 407c373 | 2024-01-26 | Migration to storage library changed storage/retry behavior. | `missed_before_hardening`: current proof covered pop/expire, retry, context/cancellation KIs, and wire-format debt, but full-drain adapter parity (`chunkSize=0` -> storage-library `-1`) was only implicit. Hardened with local obligation `full_drain_semantics`, adapter and live-Redis evidence, and review `REVIEW-407c373-storage-library`; cluster/sentinel/TLS parity remains a current proof gap to cover separately. |
| 7fa0754 | 2023-03-06 | Graph aggregate sharding and graph record errors. | Mostly covered now by Graph SQL/Mongo requirements and temporal-window/routing obligations. Would not have been fully caught before graph requirements were decomposed. |
| 17072c0 | 2023-02-21 | RootFields missing from Graph Mongo/SQL pumps. | Current field-preservation and graph requirements should catch regression if tests are annotated. This was likely not discoverable before RootFields was specified. |
| 8e42170 | 2022-11-03 | Edge case for unresolved subgraph schema. | Current graph malformed-input/edge-case obligations are relevant. Exact coverage depends on whether unresolved-schema test evidence is attached. |
| d13b62e | 2022-11-02 | Mongo aggregate 16 MiB self-healing and configurable aggregation window. | `covered_after_hardening`: SW-REQ-058 covers configurable Mongo aggregate windows; SW-REQ-062 covers backend size-error classification, aggregation-time halving, timestamp reset, and same-batch retry. Hardened with DEFECT-24 plus bounded tests for the timestamp-reset side effect and static retry wiring, avoiding the historical 16 MiB stress test in routine runs. |
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
| 0596e82 | 2025-08-16 | [TT-15532] Alternative backward-compatible fix for syslog pump log fragmentation | `caught`: DEFECT-2 + SW-REQ-050 verify one syslog entry per record and legacy `map[...]` output with escaped raw HTTP newlines. |
| df62011 | 2025-08-15 | Fix syslog pump log fragmentation issue | `caught`: superseded product-defect fix. Review `REVIEW-0596e82-syslog-fragmentation` records why the final 0596e82 behavior, not df62011 JSON output, is the proof contract. |
| 775f8e3 | 2025-02-26 | [TT-13341] Remove SQLite support from Tyk Pump | `missed_before_hardening`: generic unsupported-type paths existed, but proof did not pin SQLite as a removed backend. Hardened with local obligation `support_matrix_enforced`, production `Init` SQLite-rejection witnesses across SQL standard/aggregate/Graph/MCP variants, corrected INT-REQ-007 proof text, and review `REVIEW-775f8e3-sqlite-support-removal`; external tyk-docs drift remains KI-tracked by `docs-sqlite-still-listed-as-supported-sql-type`. |
| 544ccb3 | 2024-12-05 | TT-13421 create indexes on sharded sql pumps | `missed_before_hardening`: added DEFECT-7, local catalog obligations `per_shard_index_created` and `backend_ddl_valid`, standard/aggregate shard-index witnesses, and KIs for Graph/MCP shard-index contracts plus MySQL index DDL debt. |
| fbcb614 | 2024-11-27 | TT-12780 prevent sql pump to panic when sharding enabled and skip api id is set | `caught`: DEFECT-4 + SW-REQ-040 boundary evidence cover the empty post-filter batch under `TableSharding=true`; review `REVIEW-fbcb614-sql-empty-shard-batch` records the verdict. |
| 25683f2 | 2024-08-01 | [TT-6671] AWS Kinesis pump | Product feature surface. Current Kinesis reqs/KIs cover some risks, not a historical defect. |
| 206c1d0 | 2024-06-06 | TT-12103 Adding FIPS Support | Product build/security mode. Excluded from detailed runtime-defect assessment unless FIPS behavior is modeled. |
| d9d64dc | 2024-01-29 | fix bug where some graphql pumps are not reading env properly | Product defect. Adjacent to config/env obligations. |
| 8ca8646 | 2024-01-26 | TT-10676 Upgrade Resurface Pump backend | Product backend behavior change. Relevant to Resurface queue/cancellation obligations. |
| 407c373 | 2024-01-26 | [TT-10520] Migrating from go-redis to storage library | `missed_before_hardening`: added `full_drain_semantics` to pin the legacy drain-all behavior through the storage-library adapter; existing KIs continue to carry pop/expire atomicity, unbounded retry, caller-cancellation, singleton race, and wire-format debt. |
| bf9e7e7 | 2024-01-24 | TT-10675 add SQS Pump Backend support | `missed_before_hardening`: SQS behavior was covered by SW-REQ-055, but registry support for the new backend name was not pinned as support-matrix evidence. Hardened SW-REQ-017 with `support_matrix_enforced` and an SQS registry witness; added KI `sqs-malformed-record-sends-empty-entry`; SQS batch-limit, malformed-entry, and partial-failure product gaps remain KI-backed debt. |
| 35e1e74 | 2024-01-04 | TT-10564 Splunk backoff retry | `missed_before_hardening`: added `request_body_replay_preserved` and explicit body-replay evidence for the shared retry helper, plus Splunk `retry_policy_explicit` evidence for MaxRetries behavior. |
| 6c14ba0 | 2023-11-08 | [TT-9476] Refactor graph pumps to use new GraphQLStats | `missed_before_hardening`: current graph tests covered the GraphQLStats happy path, but SW-REQ-013 did not explicitly require structured projection from GraphQLStats or rejection of legacy tag/raw parser sources when IsGraphQL=false. Hardened with local obligation `structured_projection_preserved`, malformed-input obligations, and a legacy-source negative witness. Existing Graph SQL shard-index, aggregate shard-migration, timestamp, and SQL batch-size KIs remain the tracked operational debt. |
| 9984a1e | 2023-10-30 | [TT-9476] add graphstats to analytics record proto | `missed_before_hardening`: SW-REQ-008/INT-REQ-003 already required additive serializer fields to update schema, transformers, and evidence, and current GraphQLStats round-trip tests catch missing protobuf field 32 or transformer mappings. Hardened with a GraphQLStats wire-field-number witness and KI `serializer-protobuf-loses-graphql-error-path`, because protobuf preserves GraphError.Message but drops GraphError.Path. Existing protobuf city-name and malformed sparse-message KIs remain tracked debt. |
| 7af2a45 | 2023-09-18 | [TT-100053] aggregate graph aggregate records by api_id | `missed_before_hardening`: SW-REQ-043 already described per-(API, dimension) Graph SQL aggregate upserts and the live Postgres case covered two API IDs, but the proof model did not carry an explicit aggregate partition obligation and the fast analytics witness only exercised one API. Hardened with local obligation `aggregate_partition_isolated` and `TestAggregateGraphData_PartitionsSameOrgByAPIID`, so same-org/same-dimension GraphQL records now have direct unit-level evidence that API IDs remain separate before SQL upsert. Added `atomicity`, `transaction_isolation_declared`, and `errors_propagated` as Graph SQL aggregate backend-write obligations, deferred to KI `graph-sql-aggregate-atomicity-fault-injection-missing` until a transaction/failure-injection harness exists. |
| 657436f | 2023-08-29 | [TT-9468] New SQL Aggregate indexes | `missed_before_hardening`: current proof had SW-REQ-066 for SQL Aggregate index creation and Postgres evidence for index presence, but the history row still had no verdict and the evidence did not assert the physical composite index definition matched the documented query path. Hardened with DEFECT-8, local obligation `index_definition_matches_query`, and Postgres catalog evidence that the index columns are exactly `(dimension, timestamp, org_id, dimension_value)`. Split SQL Aggregate-specific KI debt for MySQL `CREATE INDEX IF NOT EXISTS` and background index lifecycle into `sql-aggregate-mysql-create-index-if-not-exists-unsupported` and `sql-aggregate-background-index-concurrency-unbounded`. |
| 69f5f4a | 2023-08-23 | TT-9873 Fix prometheus tracking path | `missed_before_hardening`: current proof had broad SW-REQ-024 Prometheus evidence and two WriteData path-label tests, but the tracking-path policy was not a first-class requirement/obligation and the global `TrackAllPaths=true` branch lacked WriteData registry evidence. Hardened with DEFECT-9, SW-REQ-078 for untracked `path="unknown"`, SW-REQ-079 for opt-in path preservation, local obligation `metric_path_label_tracking_policy`, and a `TrackAllPaths` boundary witness. Broad metric label-cardinality risk remains KI-backed under `metrics-label-cardinality-unbounded`. |
| 3bf1f85 | 2023-08-23 | [TT-9855] fix index creation error on graph sql pump creation | `missed_before_hardening`: current SW-REQ-042 covered Graph SQL table creation and graph-record routing, but proof did not pin the GORM model-table identity used during migration. Hardened with DEFECT-10, SW-REQ-080, local obligation `migration_model_table_identity_bound`, and Postgres Init evidence that `GraphRecord.TableName()` resolves to the configured Graph SQL table before migration. Broader Graph/MCP sharded-index and SQL schema-version gaps remain KI-backed debt. |
| 91dd8a0 | 2023-08-01 | [TT-9360] Changing Timeout from time.Duration to int | `missed_before_hardening`: current Kafka tests exercised duration strings, numeric seconds, and env override behavior, but those witnesses were attached to the TLS-focused SW-REQ-021 contract, so proof did not make timeout unit parsing a first-class requirement. Hardened with DEFECT-11, SW-REQ-081, local obligation `timeout_config_units_preserved`, MC/DC rows for configured timeout application, and a static signal for Kafka timeout fields or writer deadlines that reintroduce `time.Duration * time.Second` unit confusion. Malformed Kafka timeout still log.Fatals and remains KI-backed under `kafka-logfatal-on-init-mech-and-timeout`. |
| 6820131 | 2023-06-12 | [TT-9126] Fix error log when omit_configfile option is enabled | `partially_missed_before_hardening`: SW-REQ-002 already modeled the omit-config-file gate and caught the destructive old behavior that cleared caller defaults, but it did not explicitly assert the ticket-title behavior: omit=true with a nonexistent config path must not emit file-read or JSON-unmarshal error logs. Hardened with DEFECT-12, local obligation `config_file_omission_suppresses_file_read`, and a `TestIgnoreConfig` log-capture subtest proving env-only startup preserves config and suppresses misleading file errors. |
| 2d3c296 | 2023-05-25 | [TT-8884] added write data test for mongo pump and remove constraint | `missed_before_hardening`: the live Mongo test already caught the graph-only accumulation regression, but SW-REQ-034 only modeled MCP filtering. Hardened with DEFECT-13 and SW-REQ-082 so standard Mongo must insert ordinary and GraphQL-classified non-MCP records into the standard collection. |
| 0ed84b9 | 2023-05-17 | fix: include graph records in mongo pump | `missed_before_hardening`: this fixed the c54eed3-era standard Mongo exclusion of GraphQL-classified records, but did so by switching standard WriteData into graph-only accumulation, later fixed by TT-8884. Hardened with DEFECT-14 and SW-REQ-082; DEFECT-13 now records 0ed84b9 as the origin of the ordinary-record drop. |
| 8173c7e | 2023-05-15 | TT-876 Fix/prometheus cardinality | `missed_before_hardening`: current Prometheus tests covered separator-bearing label values, but no requirement modeled aggregate label tuple-boundary preservation. Hardened with DEFECT-15 and SW-REQ-083; broad unbounded metric cardinality remains KI-backed under `metrics-label-cardinality-unbounded`. |
| 1fd5ba9 | 2023-04-27 | [TT-8793] Fixing Pump 1.8 bugs | `partially_missed_before_hardening`: current SW-REQ-035 already pinned selective per-org collection routing, while SW-REQ-059 broadly pinned mixed aggregate writes but did not prove the second average-update upsert kept the mixed target. Hardened with DEFECT-16 for MongoSelective, DEFECT-17 plus SW-REQ-084 for MongoAggregate mixed average-update routing, SW-REQ-059 output-cardinality evidence, and nonzero request-time assertions in the live Mongo aggregate mixed-collection test. |
| ca22ae4 | 2023-03-16 | TT-8313 Hybrid pump refactor | `partially_missed_before_hardening`: this was not only a refactor; it fixed Hybrid init/connect paths that previously called log.Fatal for recoverable missing-connection-string or MDCB connection failures. Hardened with DEFECT-18 and SW-REQ-085 (`process_exit_on_recoverable` + `errors_propagated`) using existing Hybrid init/connect tests. Residual Hybrid connection leak, retry elapsed-deadline, TLS-skip, SSRF, and process-wide timeout risks remain KI-backed debt. |
| 7fa0754 | 2023-03-06 | [TT-7820] fix aggregate graph pump sharding and errors | `missed_before_hardening`: SW-REQ-043 prose required day-slice routing and selected-table upserts, but its FRETish model did not carry the shard-target invariant and the sharded test only asserted non-empty shards. Hardened with DEFECT-19 and SW-REQ-086 (`routing_target_consistent` + `output_cardinality_bounded`), plus stricter sharded evidence proving representative rows land exactly once in both selected shards while the base table remains absent. The same commit's legacy raw GraphRecord parse-error refactor is now superseded by the current GraphQLStats projection model under SW-REQ-013. |
| 17072c0 | 2023-02-21 | [TT-7977] fix: include RootFields in graph mongo and sql pumps | `partially_missed_before_hardening`: SW-REQ-013 prose and current tests already preserve `GraphQLStats.RootFields`, but the requirement was informal and had no MC/DC-bearing RootFields variable. Hardened with DEFECT-20 and SW-REQ-087 (`structured_projection_preserved`) so RootFields projection now has explicit FRETish variables and witness rows; downstream aggregate/SQL evidence already asserts `rootfields` remains present. |
| e2f277a | 2023-01-12 | TT-7216 Decode Option For Raw Request/Response | `partially_missed_before_hardening`: SYS-REQ-011 already modeled request/response decode toggles and current tests covered nominal decode, but there was no SW-level filterData child and the malformed-base64 test was using deprecated global flags instead of the per-pump toggles. Hardened with DEFECT-21 and SW-REQ-088 (`encoding_aware`), fixed the negative test setup, and kept malformed-base64 silent no-op as KI-backed debt. |
| ca44921 | 2022-11-23 | [TT-5426] Updating timestamp of every record in Demo Mode | `missed_before_hardening`: demo generation was traced to SW-REQ-009, which covers analytics field accessor determinism rather than synthetic demo timestamps, and current tests only checked broad past/future ranges. Hardened with DEFECT-22 and SW-REQ-089 (`temporal_window_inclusive`), moving GenerateDemoData/WriteDemoData trace ownership and adding boundary evidence that records in a generated hour receive matching, spaced TimeStamp/date-part values. |
| 5a25a2e | 2022-11-18 | [TT-5429] Tyk Pump Ignore Fields | `covered_today`: current SYS-REQ-016/SW-REQ-076 model the operator-listed `ignore_fields` contract by JSON tag, including per-pump wiring through `filterData` and `AnalyticsRecord.RemoveIgnoredFields`; SW-REQ-076 has full spec/code MC/DC via `TestIgnoreFieldsFilterData` plus direct `RemoveIgnoredFields` error-path evidence. Cleaned stale test/docs ownership so `TestAnalyticsRecord_RemoveIgnoredFields` points to SW-REQ-076 instead of SW-REQ-009. |
| 8e42170 | 2022-11-03 | [TT-6012] fix edge case where query is to unresolved subgraph schema | `partially_missed_before_hardening`: the historical failure was in the legacy raw GraphQL parser for federation/subgraph `_entities` selections, while current code intentionally projects GraphRecord from structured `GraphQLStats` instead of reparsing raw request/response/schema payloads. Hardened with DEFECT-23 by making SW-REQ-013 explicit about GraphQLStats-authoritative projection and adding unresolved-subgraph legacy payload evidence that structured projection still succeeds. The same commit's empty raw request/response/schema Graph Mongo behavior is already covered today by SW-REQ-037 tests that write GraphQLStats records without legacy raw payload fields. No static signal added because the legacy parser path is no longer present in current production code. |
| d13b62e | 2022-11-02 | [TT-506] Self-Healing when hitting 16mb and configurable aggregation per time | `covered_after_hardening`: current proof already had SW-REQ-058 for configurable Mongo aggregate windows and SW-REQ-062 for max-document-size self-healing, but docs overclaimed the skipped 16 MiB integration test and did not directly witness timestamp reset / same-batch retry. Hardened with DEFECT-24, corrected variable metadata for the actual size-error strings, added bounded timestamp-reset evidence, and added an AST wiring test that verifies `WriteData` classifies through `ShouldSelfHeal(err)` and retries the same `ctx`/`data` batch without generating a huge aggregate. |
| 84f3855 | 2022-11-02 | TT-3067 Add ssl_insecure_skip_verify with Elasticsearch Pump | `covered_after_hardening`: current SW-REQ-068 already models Elasticsearch custom TLS setup through shared `NewTLSConfig`, and SW-REQ-016 covers the shared TLS helper. Hardened with DEFECT-25 by attaching `tls_verification_explicit` directly to SW-REQ-068, annotating the zero-default TLS test for Elasticsearch `SSLInsecureSkipVerify`, and adding ES env evidence for `SSLINSECURESKIPVERIFY`. Strict production certificate validation remains KI-backed debt under `tls-insecure-skip-verify-allowed`; the newly found API-key+TLS transport overwrite is tracked by KI `elasticsearch-api-key-auth-dropped-when-use-ssl`. |
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
