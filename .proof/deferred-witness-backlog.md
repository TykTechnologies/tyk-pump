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
