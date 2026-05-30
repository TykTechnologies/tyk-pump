# SYS-REQ-018: Aggregated counters grouped per org and per API dimension

## Intent
When aggregation is enabled, the pump groups aggregated counters per organisation and per API dimension. This satisfies parent **STK-REQ-001** at the aggregation-shape side: rollups must be queryable along the two primary tenancy dimensions that all Tyk customers report against.

## Motivation
Without per-org grouping, multi-tenant Tyk deployments could not compute per-customer billing or usage. Without per-API grouping, operators could not answer "which endpoint drove yesterday's spike." Capturing dimensional grouping as a distinct SYS req (split from the original SYS-REQ-003 in Phase 0.6) makes the obligation atomic and code-verifiable: the obligation is satisfied iff the aggregate struct carries `OrgID` and `APIID` maps populated by the increment loop.

## Formalization
```
when aggregation_enabled aggregation shall always satisfy records_grouped_by_dimension
```
The input `aggregation_enabled` is true whenever an aggregate pump variant is configured; the output `records_grouped_by_dimension` holds when the per-org map (`map[string]AnalyticsRecordAggregate`) and the per-API counter map (`aggregate.APIID[val]`) are populated for the bucket. Variables: `specs/system/variables/aggregation.vars.yaml`.

## Code references
- `analytics/aggregate.go:722 AggregateData` — `analyticsPerOrg := make(map[string]AnalyticsRecordAggregate)` then `analyticsPerOrg[orgID] = thisAggregate` — the per-org grouping.
- `analytics/aggregate.go:768 incrementAggregate` — receives the per-org `aggregate`, then drives per-dimension counters.
- `analytics/aggregate.go:867-875` — `case "APIID":` block populating `aggregate.APIID[val]` with `Identifier` and `HumanIdentifier`.
- `analytics/aggregate.go:87 AnalyticsRecordAggregate` — struct fields `APIID map[string]*Counter` etc. declare the dimension storage.

## Evidence
- `analytics/aggregate_test.go:282 TestAggregateGraphData_Dimension` — verifies dimensional grouping.
- `analytics/aggregate_test.go:41 TestAggregate_Tags` — covers tag-dimension aggregation (additional dimension beyond org/API).
- Satisfying SW child: **SW-REQ-011** (aggregate counter accumulation).

## Open questions
- Phase 0.6 origin: spun out of SYS-REQ-003 alongside SYS-REQ-019 (counter content) so each obligation is atomic and code-verifiable.
- Records with empty `OrgID` are silently skipped (`analytics/aggregate.go:728-730`); the SYS req does not state behaviour for records lacking the grouping dimension.
- Beyond org and API, the implementation also groups by Versions, OauthIDs, Geo, Tags, APIKeys, Endpoints. The SYS req only names org and API as required; the others are present but not promised here.
