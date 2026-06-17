# Deferred Witness Backlog

Tracks every requirement obligation / acceptance criterion that is **deferred**
(no genuine typed test yet) under the hybrid disposition chosen by the owner for
the tyk-pump ReqProof coverage initiative. Each entry here is auditable: the
deferral is recorded either as an `obligation_deferrals` entry on the
requirement (for typed obligation evidence) whose `tracking` points back to this
file, or as a `witness_deferred` block on the stakeholder acceptance criterion.

This is NOT a not-applicable claim. Each item is a real gap to be closed later by
writing the named typed test, at which point the corresponding deferral /
`witness_deferred` should be removed and the test annotated with the proper
triple form `// <REQ>:<obligation>:<evidence>` (or `// <STK-REQ>:<AC>:acceptance`).

Mechanism note (HONEST DEBT, not a hidden pass): obligation-level deferral is now
recorded with the dedicated `proof req edit <REQ> --defer-obligation <class>
--reason "..." --tracking ".proof/deferred-witness-backlog.md"` primitive, which
writes an `obligation_deferrals` entry. Unlike the previously-used
`--suppress-obligation`, a deferral KEEPS the obligation in the required set and
raises a **VISIBLE WARNING** in `obligation_evidence_complete` ("N covered, M
deferred (tracked)") until a real test lands — it never silently drops the
obligation. The deferred counts below are therefore the honest debt ledger to
drive down. (Suppression is reserved strictly for obligations that genuinely do
NOT apply.) The earlier deferral-suppressions that hid these gaps as ✓ covered
have all been converted to `obligation_deferrals` via `--unsuppress-obligation`
+ `--defer-obligation`. These deferrals are owner-attributed
(`owner: human:buger`); see the governance section in the PR for suspect-review /
re-approval.

---

## Section A — Typed obligation evidence deferred (102 items)

Each row: the missing **`nominal`-floor evidence is satisfied separately**; what
is deferred here is the obligation's **typed** evidence class. Deferred via a
tracked `--defer-obligation` (`obligation_deferrals`) entry on the requirement —
a visible WARNING in `obligation_evidence_complete`, not a silent suppression.

| Requirement | Obligation class | Deferred test kind | Reason |
|-------------|------------------|--------------------|--------|
| SW-REQ-001 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-001 | rate_limit_respected | typed unit test for `rate_limit_respected` evidence | DEFERRED pending a typed rate_limit_respected test |
| SW-REQ-001 | secrets_not_logged | typed unit test for `secrets_not_logged` evidence | DEFERRED pending a typed secrets_not_logged test |
| SW-REQ-002 | malformed_input | typed unit test for `malformed_input` evidence | DEFERRED pending a typed malformed_input test |
| SW-REQ-002 | malformed_recovers_or_errors_loudly | typed unit test for `malformed_recovers_or_errors_loudly` evidence | DEFERRED pending a typed malformed_recovers_or_errors_loudly test |
| SW-REQ-002 | polymorphic_type_whitelist | typed unit test for `polymorphic_type_whitelist` evidence | DEFERRED pending a typed polymorphic_type_whitelist test |
| SW-REQ-002 | process_exit_on_recoverable | typed unit test for `process_exit_on_recoverable` evidence | DEFERRED pending a typed process_exit_on_recoverable test |
| SW-REQ-004 | error_handling | typed unit test for `error_handling` evidence | DEFERRED pending a typed error_handling test |
| SW-REQ-004 | resource_lifetime_released | typed unit test for `resource_lifetime_released` evidence | DEFERRED pending a typed resource_lifetime_released test |
| SW-REQ-005 | error_handling | typed unit test for `error_handling` evidence | DEFERRED pending a typed error_handling test |
| SW-REQ-005 | errors_propagated | typed unit test for `errors_propagated` evidence | DEFERRED pending a typed errors_propagated test |
| SW-REQ-006 | error_handling | typed unit test for `error_handling` evidence | DEFERRED pending a typed error_handling test |
| SW-REQ-006 | snapshot_wire_format_compatible | typed unit test for `snapshot_wire_format_compatible` evidence | DEFERRED pending a typed snapshot_wire_format_compatible test |
| SW-REQ-007 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-007 | cancellation_observed | typed unit test for `cancellation_observed` evidence | DEFERRED pending a typed cancellation_observed test |
| SW-REQ-007 | error_handling | typed unit test for `error_handling` evidence | DEFERRED pending a typed error_handling test |
| SW-REQ-007 | resource_lifetime_released | typed unit test for `resource_lifetime_released` evidence | DEFERRED pending a typed resource_lifetime_released test |
| SW-REQ-008 | encoding_safety | typed unit test for `encoding_safety` evidence | DEFERRED pending a typed encoding_safety test |
| SW-REQ-008 | snapshot_wire_format_compatible | typed unit test for `snapshot_wire_format_compatible` evidence | DEFERRED pending a typed snapshot_wire_format_compatible test |
| SW-REQ-009 | hash_format_deterministic | typed unit test for `hash_format_deterministic` evidence | DEFERRED pending a typed hash_format_deterministic test |
| SW-REQ-010 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-011 | encoding_aware | typed unit test for `encoding_aware` evidence | DEFERRED pending a typed encoding_aware test |
| SW-REQ-011 | idempotency | typed unit test for `idempotency` evidence | DEFERRED pending a typed idempotency test |
| SW-REQ-011 | monotonicity | typed unit test for `monotonicity` evidence | DEFERRED pending a typed monotonicity test |
| SW-REQ-016 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-016 | failure_independence_proven | typed unit test for `failure_independence_proven` evidence | DEFERRED pending a typed failure_independence_proven test |
| SW-REQ-016 | polymorphic_type_whitelist | typed unit test for `polymorphic_type_whitelist` evidence | DEFERRED pending a typed polymorphic_type_whitelist test |
| SW-REQ-016 | process_exit_on_recoverable | typed unit test for `process_exit_on_recoverable` evidence | DEFERRED pending a typed process_exit_on_recoverable test |
| SW-REQ-016 | tls_verification_explicit | typed unit test for `tls_verification_explicit` evidence | DEFERRED pending a typed tls_verification_explicit test |
| SW-REQ-021 | cancellation_observed | typed unit test for `cancellation_observed` evidence | DEFERRED pending a typed cancellation_observed test |
| SW-REQ-021 | tls_verification_explicit | typed unit test for `tls_verification_explicit` evidence | DEFERRED pending a typed tls_verification_explicit test |
| SW-REQ-024 | auth_required | typed unit test for `auth_required` evidence | DEFERRED pending a typed auth_required test |
| SW-REQ-025 | resource_lifetime_released | typed unit test for `resource_lifetime_released` evidence | DEFERRED pending a typed resource_lifetime_released test |
| SW-REQ-026 | cancellation_observed | typed unit test for `cancellation_observed` evidence | DEFERRED pending a typed cancellation_observed test |
| SW-REQ-029 | ssrf_protection | typed unit test for `ssrf_protection` evidence | DEFERRED pending a typed ssrf_protection test |
| SW-REQ-029 | tls_verification_explicit | typed unit test for `tls_verification_explicit` evidence | DEFERRED pending a typed tls_verification_explicit test |
| SW-REQ-030 | error_handling | typed unit test for `error_handling` evidence | DEFERRED pending a typed error_handling test |
| SW-REQ-030 | retry_policy_explicit | typed unit test for `retry_policy_explicit` evidence | DEFERRED pending a typed retry_policy_explicit test |
| SW-REQ-031 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-031 | error_handling | typed unit test for `error_handling` evidence | DEFERRED pending a typed error_handling test |
| SW-REQ-031 | retry_policy_explicit | typed unit test for `retry_policy_explicit` evidence | DEFERRED pending a typed retry_policy_explicit test |
| SW-REQ-032 | auth_required | typed unit test for `auth_required` evidence | DEFERRED pending a typed auth_required test |
| SW-REQ-032 | rate_limit_respected | typed unit test for `rate_limit_respected` evidence | DEFERRED pending a typed rate_limit_respected test |
| SW-REQ-034 | query_timeout_bounded | typed unit test for `query_timeout_bounded` evidence | DEFERRED pending a typed query_timeout_bounded test |
| SW-REQ-035 | query_timeout_bounded | typed unit test for `query_timeout_bounded` evidence | DEFERRED pending a typed query_timeout_bounded test |
| SW-REQ-036 | query_timeout_bounded | typed unit test for `query_timeout_bounded` evidence | DEFERRED pending a typed query_timeout_bounded test |
| SW-REQ-039 | edge_case | typed unit test for `edge_case` evidence | DEFERRED pending a typed edge_case test |
| SW-REQ-039 | errors_propagated | typed unit test for `errors_propagated` evidence | DEFERRED pending a typed errors_propagated test |
| SW-REQ-040 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-040 | forward_compatible | typed unit test for `forward_compatible` evidence | DEFERRED pending a typed forward_compatible test |
| SW-REQ-040 | parameterized_only_write | typed unit test for `parameterized_only_write` evidence | DEFERRED pending a typed parameterized_only_write test |
| SW-REQ-040 | reversible_or_documented | typed unit test for `reversible_or_documented` evidence | DEFERRED pending a typed reversible_or_documented test |
| SW-REQ-041 | deadlock_recovery | typed unit test for `deadlock_recovery` evidence | DEFERRED pending a typed deadlock_recovery test |
| SW-REQ-042 | parameterized_only_write | typed unit test for `parameterized_only_write` evidence | DEFERRED pending a typed parameterized_only_write test |
| SW-REQ-043 | parameterized_only_write | typed unit test for `parameterized_only_write` evidence | DEFERRED pending a typed parameterized_only_write test |
| SW-REQ-044 | parameterized_only_write | typed unit test for `parameterized_only_write` evidence | DEFERRED pending a typed parameterized_only_write test |
| SW-REQ-046 | ssrf_protection | typed unit test for `ssrf_protection` evidence | DEFERRED pending a typed ssrf_protection test |
| SW-REQ-047 | ssrf_protection | typed unit test for `ssrf_protection` evidence | DEFERRED pending a typed ssrf_protection test |
| SW-REQ-048 | external_call_failure_observable | typed unit test for `external_call_failure_observable` evidence | DEFERRED pending a typed external_call_failure_observable test |
| SW-REQ-048 | ssrf_protection | typed unit test for `ssrf_protection` evidence | DEFERRED pending a typed ssrf_protection test |
| SW-REQ-049 | panic_free_input_handling | typed unit test for `panic_free_input_handling` evidence | DEFERRED pending a typed panic_free_input_handling test |
| SW-REQ-050 | encoding_safety | typed unit test for `encoding_safety` evidence | DEFERRED pending a typed encoding_safety test |
| SW-REQ-051 | ssrf_protection | typed unit test for `ssrf_protection` evidence | DEFERRED pending a typed ssrf_protection test |
| SW-REQ-052 | redirect_policy_explicit | typed unit test for `redirect_policy_explicit` evidence | DEFERRED pending a typed redirect_policy_explicit test |
| SW-REQ-052 | untrusted_input_bounded | typed unit test for `untrusted_input_bounded` evidence | DEFERRED pending a typed untrusted_input_bounded test |
| SW-REQ-053 | redirect_policy_explicit | typed unit test for `redirect_policy_explicit` evidence | DEFERRED pending a typed redirect_policy_explicit test |
| SW-REQ-054 | resource_lifetime_released | typed unit test for `resource_lifetime_released` evidence | DEFERRED pending a typed resource_lifetime_released test |
| SW-REQ-055 | external_call_failure_observable | typed unit test for `external_call_failure_observable` evidence | DEFERRED pending a typed external_call_failure_observable test |
| SW-REQ-055 | idempotency_key_honored | typed unit test for `idempotency_key_honored` evidence | DEFERRED pending a typed idempotency_key_honored test |
| SW-REQ-056 | idempotency_key_honored | typed unit test for `idempotency_key_honored` evidence | DEFERRED pending a typed idempotency_key_honored test |
| SW-REQ-056 | monotonicity | typed unit test for `monotonicity` evidence | DEFERRED pending a typed monotonicity test |
| SW-REQ-057 | external_call_timeout_bounded | typed unit test for `external_call_timeout_bounded` evidence | DEFERRED pending a typed external_call_timeout_bounded test |
| SW-REQ-058 | determinism | typed unit test for `determinism` evidence | DEFERRED pending a typed determinism test |
| SW-REQ-060 | hash_format_deterministic | typed unit test for `hash_format_deterministic` evidence | DEFERRED pending a typed hash_format_deterministic test |
| SW-REQ-060 | idempotency | typed unit test for `idempotency` evidence | DEFERRED pending a typed idempotency test |
| SW-REQ-060 | monotonicity | typed unit test for `monotonicity` evidence | DEFERRED pending a typed monotonicity test |
| SW-REQ-062 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-062 | error_handling | typed unit test for `error_handling` evidence | DEFERRED pending a typed error_handling test |
| SW-REQ-062 | monotonicity | typed unit test for `monotonicity` evidence | DEFERRED pending a typed monotonicity test |
| SW-REQ-063 | idempotency | typed unit test for `idempotency` evidence | DEFERRED pending a typed idempotency test |
| SW-REQ-064 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-064 | ordering_guarantees_documented | typed unit test for `ordering_guarantees_documented` evidence | DEFERRED pending a typed ordering_guarantees_documented test |
| SW-REQ-064 | temporal_window_inclusive | typed unit test for `temporal_window_inclusive` evidence | DEFERRED pending a typed temporal_window_inclusive test |
| SW-REQ-065 | idempotency | typed unit test for `idempotency` evidence | DEFERRED pending a typed idempotency test |
| SW-REQ-065 | reversible_or_documented | typed unit test for `reversible_or_documented` evidence | DEFERRED pending a typed reversible_or_documented test |
| SW-REQ-066 | idempotency | typed unit test for `idempotency` evidence | DEFERRED pending a typed idempotency test |
| SW-REQ-067 | monotonicity | typed unit test for `monotonicity` evidence | DEFERRED pending a typed monotonicity test |
| SW-REQ-067 | ordering_guarantees_documented | typed unit test for `ordering_guarantees_documented` evidence | DEFERRED pending a typed ordering_guarantees_documented test |
| SW-REQ-068 | ssrf_protection | typed unit test for `ssrf_protection` evidence | DEFERRED pending a typed ssrf_protection test |
| SW-REQ-070 | boundary | typed unit test for `boundary` evidence | DEFERRED pending a typed boundary test |
| SW-REQ-070 | external_call_concurrency_bounded | typed unit test for `external_call_concurrency_bounded` evidence | DEFERRED pending a typed external_call_concurrency_bounded test |
| SW-REQ-071 | panic_free_input_handling | typed unit test for `panic_free_input_handling` evidence | DEFERRED pending a typed panic_free_input_handling test |
| SW-REQ-072 | external_call_timeout_bounded | typed unit test for `external_call_timeout_bounded` evidence | DEFERRED pending a typed external_call_timeout_bounded test |
| SW-REQ-073 | parameterized_only_read | typed unit test for `parameterized_only_read` evidence | DEFERRED pending a typed parameterized_only_read test |
| SW-REQ-074 | secrets_not_logged | typed unit test for `secrets_not_logged` evidence | DEFERRED pending a typed secrets_not_logged test |
| SYS-REQ-004 | external_call_concurrency_bounded | typed unit test for `external_call_concurrency_bounded` evidence | DEFERRED pending a typed external_call_concurrency_bounded test |
| SYS-REQ-004 | failure_independence_proven | typed unit test for `failure_independence_proven` evidence | DEFERRED pending a typed failure_independence_proven test |
| SYS-REQ-005 | external_call_timeout_bounded | typed unit test for `external_call_timeout_bounded` evidence | DEFERRED pending a typed external_call_timeout_bounded test |
| SYS-REQ-008 | malformed_input | typed unit test for `malformed_input` evidence | DEFERRED pending a typed malformed_input test |
| SYS-REQ-012 | rate_limit_respected | typed unit test for `rate_limit_respected` evidence | DEFERRED pending a typed rate_limit_respected test |
| SYS-REQ-016 | secrets_not_logged | typed unit test for `secrets_not_logged` evidence | DEFERRED pending a typed secrets_not_logged test |
| SYS-REQ-035 | malformed_recovers_or_errors_loudly | typed unit test for `malformed_recovers_or_errors_loudly` evidence | DEFERRED pending a typed malformed_recovers_or_errors_loudly test |

---

## Section A2 — Honesty-pass reclassifications (deeper suppression scan, 6 items)

Deeper scan of the remaining `obligation_suppressions` (post-102-conversion) found
six entries whose rationale was DEFERRAL-flavored — the obligation genuinely
applies and a real gap is acknowledged, but the satisfying test/remediation is
postponed — hiding among the genuine not-applicable set. These were converted from
`obligation_suppressions` to `obligation_deferrals` (honest, tracked debt) by
`PROOF_ACTOR=human:buger`. The remaining suppressions on these requirements stay
as genuine not-applicable.

| Requirement | Obligation | Deciding rationale | Tracking |
|-------------|------------|--------------------|----------|
| SW-REQ-001 | fanout_panic_isolated | execPumpWriting inner goroutine has no defer recover(); KI pump-fanout-panic-not-recovered disposition=**fix** (remediation committed, not landed) | KI pump-fanout-panic-not-recovered |
| SW-REQ-001 | failure_independence_proven | independence proof pends the same fix-dispositioned panic-isolation hole | KI pump-fanout-panic-not-recovered |
| SW-REQ-039 | atomicity | MCP-mongo-aggregate $inc atomicity negative test needs a fault-injecting Mongo harness not in CI; future work | this backlog |
| SW-REQ-060 | atomicity | Mongo $inc atomicity negative test needs fault-injection CI cannot provision; future work | this backlog |
| SW-REQ-067 | atomicity | SQL aggregate upsert atomicity negative test needs chaos-injection the Postgres testcontainer lacks; future work | this backlog |
| SW-REQ-061 | denial_of_service_resistant | tag-list fuzz harness "does not exist today"; future work | this backlog |

---

## Section A3 — Maximum-strictness honesty pass: KI-backed real findings converted (151 items)

The owner's maximum-strictness decision: **a suppression remains ONLY if the
obligation GENUINELY DOES NOT APPLY. Tracked-as-a-KI does NOT make a real gap
not-applicable.** A prior pass had left ~135+ obligations backed by
`ship`/`ship_with_known_issue`/accepted-risk KIs (the Phase-G/H/N opengrep
families) as N/A `obligation_suppressions` — but those are REAL untested
obligations: the behavior/test is absent. They have now been converted from
`obligation_suppressions` to visible, tracked `obligation_deferrals` (honest
debt) by `PROOF_ACTOR=human:buger`, each `--tracking` its backing KI.

**151 suppressions converted** (across 54 requirements), via
`req edit --unsuppress-obligation <class> --confirm-unsuppress` →
`--add-obligation <class>` → `--defer-obligation <class> --reason "..."
--tracking "<KI>"`. Each deferral reason is honest and makes **no false "will
fix" claim**: *"real untested obligation under known-issue &lt;KI&gt;; surfaced
as visible tracked debt, not an overclaimed fix"*. Each stays a **WARNING** in
`obligation_evidence_complete` until a real typed test lands.

Converted families (class | #reqs | backing KI | requirements):

| Obligation class | #reqs | Backing KI | Requirements |
|------------------|-------|------------|--------------|
| atomicity | 1 | uptime-aggregate-erasstr-itoa-always-nonempty | SW-REQ-073 |
| cancellation_observed | 4 | mongo-pump-ignores-caller-context | SW-REQ-034, SW-REQ-035, SW-REQ-037, SW-REQ-072 |
| cascade_circuit_breaker | 1 | pump-no-per-pump-circuit-breaker | SW-REQ-001 |
| clock_skew_tolerated | 1 | getanddeleteset-expire-ttl-assumes-clock-sync | SW-REQ-007 |
| concurrent | 1 | instrumentation-goroutines-no-recover-or-shutdown | SW-REQ-005 |
| concurrent | 2 | mongo-pump-ignores-caller-context | SW-REQ-034, SW-REQ-037 |
| concurrent | 1 | pump-fanout-panic-not-recovered | SW-REQ-001 |
| concurrent | 1 | storage-connector-singleton-race | SW-REQ-007 |
| determinism | 3 | elasticsearch-unbounded-reconnect-recursion | SW-REQ-068, SW-REQ-069, SW-REQ-070 |
| determinism | 1 | graylog-nil-client-recursive-writedata-duplicates-data | SW-REQ-049 |
| determinism | 1 | splunk-filtertags-skips-consecutive-matches | SW-REQ-048 |
| error_handling | 1 | mongo-pump-ignores-caller-context | SW-REQ-072 |
| errors_propagated | 27 | pumps-logfatal-on-config-decode | SW-REQ-021, SW-REQ-023, SW-REQ-024, SW-REQ-025, SW-REQ-026, SW-REQ-034, SW-REQ-035, SW-REQ-036, SW-REQ-046, SW-REQ-047, SW-REQ-049, SW-REQ-050, SW-REQ-051, SW-REQ-052, SW-REQ-053, SW-REQ-055, SW-REQ-056, SW-REQ-057, SW-REQ-058, SW-REQ-059, SW-REQ-060, SW-REQ-061, SW-REQ-062, SW-REQ-063, SW-REQ-068, SW-REQ-069, SW-REQ-070 |
| external_call_failure_observable | 1 | hybrid-getdialfn-leaks-conn-on-handshake-fail | SW-REQ-029 |
| external_call_timeout_bounded | 1 | storage-retry-maxelapsed-zero-is-unbounded | SW-REQ-031 |
| input_size_bounded | 1 | kinesis-splitintobatches-zero-infinite-loop | SW-REQ-056 |
| input_size_bounded | 1 | retry-buffers-full-request-body-in-memory | SW-REQ-030 |
| input_size_bounded | 1 | sqs-batchlimit-zero-infinite-loop | SW-REQ-055 |
| malformed_input | 1 | kinesis-splitintobatches-zero-infinite-loop | SW-REQ-056 |
| malformed_input | 1 | sqs-batchlimit-zero-infinite-loop | SW-REQ-055 |
| malformed_recovers_or_errors_loudly | 45 | mapstructure-decode-silently-drops-unknown-keys | SW-REQ-006, SW-REQ-007, SW-REQ-021, SW-REQ-023, SW-REQ-024, SW-REQ-025, SW-REQ-026, SW-REQ-029, SW-REQ-034, SW-REQ-035, SW-REQ-036, SW-REQ-037, SW-REQ-038, SW-REQ-039, SW-REQ-040, SW-REQ-041, SW-REQ-042, SW-REQ-043, SW-REQ-044, SW-REQ-045, SW-REQ-046, SW-REQ-047, SW-REQ-048, SW-REQ-049, SW-REQ-050, SW-REQ-051, SW-REQ-052, SW-REQ-053, SW-REQ-054, SW-REQ-055, SW-REQ-056, SW-REQ-057, SW-REQ-058, SW-REQ-059, SW-REQ-060, SW-REQ-061, SW-REQ-062, SW-REQ-063, SW-REQ-064, SW-REQ-065, SW-REQ-066, SW-REQ-067, SW-REQ-068, SW-REQ-069, SW-REQ-070 |
| monotonicity | 1 | uptime-aggregate-erasstr-itoa-always-nonempty | SW-REQ-073 |
| nil_safety | 4 | mongo-pump-writeuptime-nil-on-bad-msgpack | SW-REQ-034, SW-REQ-035, SW-REQ-040, SW-REQ-055 |
| nil_safety | 1 | preprocess-decode-error-leaves-nil-hole-in-keys | SW-REQ-001 |
| nominal | 1 | uptime-aggregate-erasstr-itoa-always-nonempty | SW-REQ-073 |
| panic_free_input_handling | 1 | csv-writedata-nil-file-handle-panic | SW-REQ-025 |
| panic_free_input_handling | 1 | instrumentation-goroutines-no-recover-or-shutdown | SW-REQ-005 |
| panic_free_input_handling | 9 | no-panic-recovery-in-exec-pump-writing | SW-REQ-024, SW-REQ-037, SW-REQ-040, SW-REQ-041, SW-REQ-045, SW-REQ-064, SW-REQ-065, SW-REQ-066, SW-REQ-067 |
| panic_free_input_handling | 1 | pumps-logfatal-on-config-decode | SW-REQ-034 |
| partial_progress_observable | 1 | pump-no-per-pump-circuit-breaker | SW-REQ-001 |
| per_pump_circuit_breaker | 1 | pump-no-per-pump-circuit-breaker | SW-REQ-001 |
| process_exit_on_recoverable | 19 | pumps-logfatal-on-config-decode | SW-REQ-021, SW-REQ-023, SW-REQ-024, SW-REQ-025, SW-REQ-026, SW-REQ-034, SW-REQ-035, SW-REQ-036, SW-REQ-046, SW-REQ-047, SW-REQ-049, SW-REQ-050, SW-REQ-051, SW-REQ-052, SW-REQ-053, SW-REQ-055, SW-REQ-056, SW-REQ-057, SW-REQ-068 |
| request_timeout_bounded | 1 | prometheus-init-mutates-default-mux | SW-REQ-024 |
| request_timeout_bounded | 3 | pump-no-timeout-can-block-purge-cycle | SW-REQ-068, SW-REQ-069, SW-REQ-070 |
| request_timeout_bounded | 1 | splunk-newsplunkclient-mutates-default-transport | SW-REQ-048 |
| temporal_window_inclusive | 6 | analytics-timestamp-timezone-convention-unpinned | SW-REQ-040, SW-REQ-041, SW-REQ-042, SW-REQ-043, SW-REQ-044, SW-REQ-045 |
| tz_explicit | 3 | analytics-timestamp-timezone-convention-unpinned | SW-REQ-009, SW-REQ-011, SW-REQ-015 |

### What was LEFT as genuine not-applicable (61 suppressions, NOT converted)

Only suppressions where the obligation **genuinely does not apply** remain. These
are category mismatches / default-secure-by-design / ownership-moved, not absent
behavior:

| Genuine-N/A category | Class(es) | # | Requirements |
|----------------------|-----------|---|--------------|
| `concurrent` on disjoint / no-shared-state (gorm/prometheus library-safe handles, channel-guarded single-consumer, disjoint per-batch state) — no race surface, no KI | concurrent | 10 | SW-REQ-024, 038, 040, 041, 045, 054, 064, 065, 066, 067 |
| SQL family Init-level zero-guard already mitigates; opengrep rule is a co-located-guard **style preference** already satisfied | input_size_bounded, malformed_input | 20 | SW-REQ-040, 041, 042, 043, 044, 045, 064, 065, 066, 067 |
| CREATE INDEX DDL on internal sharding identifier (not user data); parameter binding N/A to SQL identifiers | parameterized_only_write | 6 | SW-REQ-041, 045, 064, 065, 066, 067 |
| Phase-A decomposition: obligation moved to per-implementation child (`implemented_by` trace) | errors_propagated, parameterized_only_write | 6 | SW-REQ-018, 019, 020, 022, 027, 028 |
| Default-secure-by-design TLS (`InsecureSkipVerify` defaults false, operator opt-in, #nosec G402) | cert_validation_strict | 5 | SW-REQ-016, 021, 029, 048, 068 |
| `dst_transition_safe` on a `time.Since`/`Sub` **elapsed-duration** (monotonic clock) site, not wall-clock bucketing | dst_transition_safe | 4 | SW-REQ-001, 021, 030, 055 |
| `retry_budget_bounded` already bounded by construction (`backoff.WithMaxRetries`) | retry_budget_bounded | 2 | SW-REQ-029, 030 |
| By-design non-determinism: moesif sampling RNG / demo-only fixture generator | determinism | 2 | SW-REQ-009, 052 |
| Trusted-data recovery loop (catalog carve-out for same-process consumers) | denial_of_service_resistant | 1 | SW-REQ-062 |
| By-design UTF-8 string transport (base64 for binary), trimming contract | binary_data_preserved | 1 | SW-REQ-009 |
| moesif sampling-config HTTP **response** body (upstream API, not attacker input), upstream-bounded | input_size_bounded, malformed_input | 2 | SW-REQ-052 |
| Outbound pump-emitted body, read-error handled, upstream owns shape | malformed_input | 1 | SW-REQ-030 |
| `/health` is liveness-only by-design (KI health-endpoint-is-liveness-only); readiness not gated | external_call_failure_observable | 1 | SW-REQ-032 |

(The earlier Section A2 fix-dispositioned items remain deferred; the genuine-N/A
SW-REQ-001 `dst_transition_safe` example in Section C is part of the 61 above.)

---

## Section B — End-to-end stakeholder acceptance criteria deferred (10 items)

Deferred via `witness_deferred` on each criterion (mechanism mirrors reqproof
#200). Reason: "No end-to-end acceptance test yet; deferred per hybrid disposition".
To close: write a real ACCEPTANCE test on the integrated system and annotate
`// <STK-REQ>:<AC-id>:acceptance`, then remove the `witness_deferred` block.

| Stakeholder req | Acceptance criterion | Deferred test kind | Reason |
|-----------------|----------------------|--------------------|--------|
| STK-REQ-001 | AC-001 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-001 | AC-002 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-002 | AC-001 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-002 | AC-002 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-003 | AC-001 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-003 | AC-002 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-004 | AC-001 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-004 | AC-002 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-005 | AC-001 | e2e acceptance test | No end-to-end acceptance test yet |
| STK-REQ-005 | AC-002 | e2e acceptance test | No end-to-end acceptance test yet |

---

## Section C — Notes

- The `nominal` positive floor (`<REQ>:nominal:nominal`) is **satisfied with real
  happy-path tests**, not deferred — see the 18 witnesses added in this change.
- SW-REQ-012 and SW-REQ-013 retain their pre-existing `:nominal:negative`
  error-path tests AND now carry distinct positive `:nominal:nominal` witnesses;
  neither was flipped.
- True not-applicable suppressions (genuine out-of-contract obligations, e.g.
  SW-REQ-001 dst_transition_safe on a monotonic-duration log site) are recorded
  directly on the requirements with a not-applicable rationale and are NOT listed
  here — this backlog is only for honestly-deferred items pending a future test.
