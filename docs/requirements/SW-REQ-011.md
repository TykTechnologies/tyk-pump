# SW-REQ-011: Monotonic aggregate counter accumulation

## Intent
Realises parent **SYS-REQ-003**. `analytics.incrementOrSetUnit(b, c)` is the workhorse used by every per-dimension aggregator (`AggregateData`, `AggregateGraphData`, `AggregateMCPData`, `AggregateUptimeData`): it takes a "base" counter `b` for the current record and either initialises a freshly-allocated counter from `b` (when the dimension's existing counter `c` is nil) or accumulates each scalar field of `b` into the existing `c` via `+=`. As more records are folded in, `Hits`, `Success`, `ErrorTotal`, `TotalRequestTime`, `TotalLatency`, `TotalUpstreamLatency`, and each per-status-code entry of `ErrorMap` only ever grow.

## Motivation
Aggregate counters drive dashboard charts and downstream billing; a non-monotonic counter is a usability bug ("the count went down after a refresh?") and a billing bug. Centralising accumulation in a single helper means every caller — APIID, APIKey, Geo, Tags, Endpoints, OauthIDs, Versions, Errors, MCP Methods/Primitives/Names — gets identical semantics for free, and an audit can be confined to one function. Trade-off: `RequestTime` is recomputed as `TotalRequestTime / Hits` after each accumulation (not just at flush) so reads see a running average; `Min*Latency` is *not* monotonic by design (latency mins shrink as smaller values arrive), and the helper guards `MinLatency` updates with a `base.ErrorTotal == 0` check so error-paths can't poison min latency.

## Code references
- `analytics/aggregate.go:992 incrementOrSetUnit` — the centre of the monotonicity guarantee. Lines 1002-1009 are the `+=` accumulations; lines 1011-1027 are the per-counter latency min/max updates.
- `analytics/aggregate.go:768 incrementAggregate` — per-dimension iterator that calls `incrementOrSetUnit` for every label (APIID at line 870, Errors at 884, Versions at 893, APIKeys at 902, OauthIDs at 926, Geo at 946, Tags at 958, Endpoints at 973, ApiEndpoint at 979).
- `analytics/aggregate.go:722 AggregateData` — top-level entry that loops records and calls `incrementAggregate`.
- `analytics/aggregate.go:191 Code.ProcessStatusCodes` — folds the per-status `ErrorMap` into the `Code` struct totals.
- `analytics/uptime_data.go:225 IncrementOrSetUnit` (closure inside `AggregateUptimeData`) — the parallel implementation for uptime data, structurally identical.

## Related requirements
- **SW-REQ-093** owns REST aggregate partitioning: `AggregateData` excludes
  GraphQL- and MCP-classified records from REST aggregates before the
  monotonic counter logic covered here is applied.

## Evidence
- `analytics/aggregate_test.go:25 TestCode_ProcessStatusCodes` — tagged `// Verifies: SW-REQ-011`; the per-status-code summation.
- `analytics/aggregate_test.go:41 TestAggregate_Tags`, `:96 TestAggregateGraphData`, `:282 TestAggregateGraphData_Dimension`, `:352 TestAggregateData_SkipGraphRecords` — multi-record accumulation checks.
- `analytics/aggregate_test.go:415 TestSetAggregateTimestamp`, `:456 TestAggregatedRecord_TableName`, `:488 TestAggregatedRecord_GetObjectID` — surrounding aggregate-API coverage.
- `analytics/aggregate_mcdc_test.go:30 TestAggregateData_MCDCBranches` — tagged `// SW-REQ-011:monotonicity:negative`; MC/DC coverage of the branches inside `incrementOrSetUnit` and `incrementAggregate`.

## Open questions
- `MinLatency` / `MinUpstreamLatency` are *not* monotonic — they shrink as the aggregate observes faster requests. The req description says "never decrease" but the only fields where that strictly holds are `Hits`, `Success`, `ErrorTotal`, `TotalRequestTime`, `TotalLatency`, `TotalUpstreamLatency`, and the per-status `ErrorMap` entries. The min-latency fields are intentionally non-monotonic but the wording does not say so.
- `c.RequestTime = c.TotalRequestTime / float64(c.Hits)` divides by `c.Hits`; if a future caller ever sets `Hits = 0` while having a non-zero total, this becomes `+Inf` rather than `0`. No guard.
