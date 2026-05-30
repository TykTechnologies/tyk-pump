# SYS-REQ-009: Per-backend filtering with block-takes-precedence

## Intent
Per backend, the pump excludes a record before forwarding when (a) it matches an operator-configured block list on `org_id`, `api_id`, or `response_code`, OR (b) at least one non-empty allow list is configured and the record does not satisfy it; block matches take precedence over allow matches. This satisfies parent **STK-REQ-003** (operator-configurable behaviour) by giving operators the standard allow/deny tooling without surprising precedence.

## Motivation
Filtering is the primary mechanism operators use to scope what each backend sees (e.g. send only org X to billing, exclude internal API IDs from external logging). The boundary obligation captured here pins the precedence rule — block always wins — so future maintainers cannot reorder the cases and silently change which records reach which sinks. The 6-case `switch` in `ShouldFilter` is the single source of truth.

## Formalization
```
when (record_matches_block_filter | record_outside_allow_list) filtering shall always satisfy record_excluded
```
The disjoint input encodes the two exclusion paths; the output `record_excluded` becomes true when `ShouldFilter` returns `true` and `filterData` therefore skips dispatching that record to the given pump. Variables: `specs/system/variables/filtering.vars.yaml`.

## Code references
- `analytics/analytics_filters.go:3 AnalyticsFilters` — declares `OrgsIDs`, `APIIDs`, `ResponseCodes` (allow) and `SkippedOrgsIDs`, `SkippedAPIIDs`, `SkippedResponseCodes` (block).
- `analytics/analytics_filters.go:19 ShouldFilter` — the precedence-bearing `switch`: block cases (`SkippedAPIIDs`, `SkippedOrgsIDs`, `SkippedResponseCodes`) are evaluated *first*; allow cases come after.
- `analytics/analytics_filters.go:38 HasFilter` — early-exit predicate.
- `main.go:408 if filters.ShouldFilter(decoded) { continue }` — call site in the per-pump `filterData`.
- `pumps/common.go:30 SetFilters` / `:35 GetFilters` — wiring into each pump's filter slot.

## Evidence
- `analytics/analytics_filters_test.go:12 TestShouldFilter` — covers every block/allow combination (large table-driven test).
- `analytics/analytics_filters_test.go:111 TestHasFilter`.
- `main_test.go:168 TestWriteDataWithFilters` — end-to-end fan-out with mixed allow/block per pump.
- Satisfying SW child: **SW-REQ-010** (AnalyticsFilters semantics).

## Open questions
- The FRETish disjunction `(block | outside_allow)` covers the externally visible cases, but does not say the block clauses are *checked first*. The precedence is correct in code (block cases listed first in the `switch`) but the SYS req relies on prose ("block-list matches take precedence") rather than encoding ordering in FRETish.
- Filtering applies to non-aggregated dispatch via `filterData`; aggregate pumps pre-filter inside their own `WriteData` (e.g. `pumps/mongo_aggregate.go:313`). The SYS req does not call out that the same `AnalyticsFilters` struct governs both paths.
