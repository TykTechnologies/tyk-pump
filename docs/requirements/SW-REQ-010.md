# SW-REQ-010: Filter with block-list precedence over allow-list

## Intent
Realises parent **SYS-REQ-009**. `AnalyticsFilters.ShouldFilter` evaluates a single record against six possible criteria — three allow lists (`OrgsIDs`, `APIIDs`, `ResponseCodes`) and three block lists (`SkippedOrgsIDs`, `SkippedAPIIDs`, `SkippedResponseCodes`) — using a `switch` whose first three cases are the block lists. If the record matches any populated block list it is filtered out immediately, *before* any allow list is consulted; allow lists then drop records that fail to match a populated allow list. Empty lists are treated as "not configured" and are skipped.

## Motivation
Block-precedence is the obvious operator-friendly behaviour ("I explicitly listed this org as skip — skip it, regardless of what else matches") and a `switch` ordering makes the precedence syntactically visible rather than depending on combined-boolean evaluation. The `len(list) > 0 && …` guard on each branch is what lets operators leave block lists empty in JSON without inadvertently filtering everything out. Trade-off: each list is a slice scanned via `stringInSlice` / `intInSlice` (O(n) per record) — fine for handfuls of IDs, less great for thousands. There is no map-backed fast path.

## Code references
- `analytics/analytics_filters.go:19 ShouldFilter` — the `switch` with block-list cases first (lines 21-26), allow-list cases second (lines 27-32), default `return false`.
- `analytics/analytics_filters.go:38 HasFilter` — short-circuit used by `main.filterData` to skip the whole filter pass when no list is configured.
- `analytics/analytics_filters.go:46 stringInSlice`, `:56 intInSlice` — the linear-scan helpers.
- `main.go:408` — `if filters.ShouldFilter(decoded) { continue }` call site inside `filterData`.

## Evidence
- `analytics/analytics_filters_test.go:12 TestShouldFilter` — tagged `// Verifies: SW-REQ-010`; table-driven coverage including the explicit "block list wins over allow list" case.
- `analytics/analytics_filters_test.go:111 TestHasFilter` — tagged `// Verifies: SW-REQ-010`.
- `analytics/coverage2_test.go:34 TestShouldFilter_SkipAndAllowBranches` — tagged `// SW-REQ-010:boundary:negative`; per-branch coverage of each list type.
- `analytics/aggregate_mcdc_test.go:64 TestHasFilter_EachList` — tagged `// SW-REQ-010:boundary:negative`; MC/DC-style coverage of `HasFilter`.

## Open questions
- The `switch` is "first-matching-case wins" so the precedence between two simultaneously-populated block lists (e.g. block-by-org *and* block-by-response-code) is positional: API-ID block beats Org block beats response-code block. This is observable but not specified in the req description.
- `intInSlice` matches `0` as a valid response code; the AnalyticsRecord field is zero-init `int`. A record that arrives with an unset `ResponseCode` and an allow list of `[200]` will be filtered out — which is correct, but a `skip_response_codes: [0]` configured by an operator who *wanted* to drop "unset" records will also drop legitimate `200=0` corruption, since the type is `int` not `*int`.
