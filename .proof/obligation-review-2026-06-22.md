# Obligation Review - 2026-06-22

Reviewer: human:buger via agent-assisted review
Branch: feat/reqproof-coverage
Scope: tyk-pump requirement corpus, Proof catalogue, project-local catalogue,
KnownIssue records, ProblemReport records, accepted risks, code-signal output,
and pump/shared-subsystem behavior surfaces visible in this repository.

## Claim

This pass re-reviewed the current tyk-pump requirements against the available
Proof catalogue and the project-local catalogue. Every high-confidence runtime,
failure, data-integrity, backpressure, retry, cancellation, timeout, schema,
signal, or history-derived gap found during this pass was modeled in one of the
accepted ways:

- real evidence or satisfying child coverage already present;
- new or existing KnownIssue-backed obligation debt;
- reviewed decomposition where the warning was a parent/child scope mismatch;
- explicit active debt left visible by Proof rather than product code changes.

This is not a proof that no bugs can exist. It is a proof-corpus review of the
obligation catalogue and the evidence available from code, specs, KnownIssues,
ProblemReports, accepted risks, code signals, and focused subagent reviews.
Unknown input combinations, future code changes, unsignaled behavior classes,
and bugs outside the current catalogue/code-signal vocabulary remain possible.

## Review Method

Four independent read-only subagents reviewed distinct slices:

- catalogue and taxonomy coverage;
- pump-by-pump runtime/failure behavior;
- shared subsystems such as storage, retry, serializer, fanout, health, metrics;
- defect, KnownIssue, accepted-risk, and history alignment.

The integration pass then updated only proof/spec artifacts. No production code
was changed and no product bug was silently fixed.

## Coverage Themes Reviewed

The pass explicitly checked the following behavior classes across pump families
and shared subsystems:

- backend down, backend slow, timeout/deadline, retry budget;
- error propagation and external-call failure observability;
- output cardinality, duplicate/loss behavior, replay/idempotency;
- buffer/backpressure, OOM/input-size behavior, batch allocation;
- partial write and per-entry backend failure handling;
- panic isolation, process-exit-on-recoverable paths, fanout isolation;
- shutdown, cancellation, resource lifetime, async queue drain behavior;
- malformed input, nil/zero-value input, unknown configuration fields;
- TLS/cert validation, SSRF allowlist, auth/security-relevant surfaces;
- SQL schema migration/version policy and backend-specific consistency;
- timezone/DST/temporal-window behavior;
- code-signal obligations from OpenGrep and project-specific signal rules.

## Material Additions

New active KnownIssues:

- `metrics-label-cardinality-unbounded`
- `instrumentation-statsd-channel-backpressure-blocks-emitters`
- `kafka-writedata-full-batch-allocation-unbounded`

New local catalogue class:

- `schema_version_policy_enforced`

Existing KnownIssues were widened or given evidence where needed, including:

- `storage-retry-maxelapsed-zero-is-unbounded`
- `serializer-protobuf-loses-city-names`
- `sql-default-migration-today-only`
- `elasticsearch-mcp-routing-non-bulk-ignored`
- `logzio-segment-no-shutdown-flush`

Requirement obligations/deferrals were expanded on stakeholder, system,
interface, and software requirements for delivery retention, retry budget,
context cancellation, write-error observability, timezone convention, schema
version policy, metric cardinality, backpressure, Kafka batch memory pressure,
and Elasticsearch MCP non-bulk routing.

## ProblemReport Modeling Decision

ProblemReport / DEFECT records are treated as historical fixed-defect closure
records. Active/unfixed behavior belongs in KnownIssue records.

This pass removed overlap-only active KnownIssue links from fixed DEFECT records
instead of using DEFECT disposition fields to park active debt. As a result,
`problem_reports_reviewed` still reports overlap warnings when a fixed DEFECT
shares a requirement with an unrelated active KnownIssue. This is intentional
project modeling, not hidden product debt.

The reqproof/tool limitation is tracked upstream:

- probelabs/reqproof#319

## Verification

Commands run after the integration pass:

```bash
proof validate
proof workflow check --stage spec --fail-level error
proof workflow check --stage verify --only decomposition_reviewed,mcdc_known_issue_disposition_stale,obligation_evidence_complete,obligation_decomposition_complete,known_issue_complete --fail-level error
PROOF_MONITOR_MIN_FREE_PERCENT=1 PROOF_MONITOR_KILL_FREE_PERCENT=1 PROOF_MONITOR_KILL_SAMPLES=8 PROOF_MONITOR_INTERVAL_SEC=3 bin/run-monitored proof workflow check --stage verify --fail-level error
PROOF_MONITOR_MIN_FREE_PERCENT=1 PROOF_MONITOR_KILL_FREE_PERCENT=1 PROOF_MONITOR_KILL_SAMPLES=8 PROOF_MONITOR_INTERVAL_SEC=3 bin/run-monitored proof audit --fail-level error --max-findings 20
```

Final full audit result:

- errors: 0
- warnings: 3
- build: passed
- configured MC/DC test jobs: passed
- `mcdc-pumps`: completed under memory guard without intervention
- `known_issue_complete`: passed
- `mcdc_known_issue_disposition_stale`: passed
- `obligation_decomposition_complete`: passed
- `obligation_evidence_complete`: passed
- `code_signal_obligations_reviewed`: 0 untriaged findings, remaining rows are
  deferred tracked debt

Remaining warnings:

- `acceptance_criteria_witnessed`: `STK-REQ-002 AC-003` is intentionally
  deferred to `write-failure-after-pop-loses-records`; no truthful acceptance
  test can pass until DLQ, re-enqueue, or equivalent retention exists.
- `code_signal_obligations_reviewed`: 0 untriaged; deferred tracked debt remains
  visible.
- `problem_reports_reviewed`: fixed DEFECT records overlap requirements with
  unrelated active KnownIssues; tracked as reqproof issue #319.

## Residual Risk

This review improves catalogue/obligation modeling but does not guarantee the
absence of all bugs. Additional bug-finding would still benefit from:

- long-running stress/load tests with real backend outages and slow backends;
- fuzzing malformed analytics records, config maps, serializers, and pump inputs;
- race tests across all async pumps and instrumentation paths;
- chaos tests for Redis, SQL, Mongo, Elasticsearch, Kafka, SQS, Kinesis, and HTTP
  sinks;
- memory/heap profiling of large batches and async queues;
- production-like acceptance tests for DLQ/re-enqueue behavior once that feature
  exists.
