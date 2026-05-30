# SYS-REQ-019: Per-unit counters track hits, successes, errors, latency

## Intent
When records have been aggregated within the configured window, the pump maintains hits, successes, errors and latency counters per aggregated unit. This satisfies parent **STK-REQ-001** by guaranteeing that every aggregated row downstream has the four core metrics dashboards depend on — not just hit counts.

## Motivation
Aggregated analytics with only hit counts are useless for SRE work: operators need latency distributions and error rates side-by-side. Capturing the four-counter set (split from SYS-REQ-003 in Phase 0.6) makes it concrete which metrics the SQL/Mongo aggregate schemas must carry, and protects them against accidental schema-drift in `Counter`.

## Formalization
```
when records_aggregated aggregation shall always satisfy hits_errors_latency_counted
```
The input `records_aggregated` is true once at least one record has been processed by `incrementAggregate` within the current window; the output `hits_errors_latency_counted` becomes true when the per-unit `Counter` has its Hits/Success/ErrorTotal/Latency fields advanced. Variables: `specs/system/variables/aggregation.vars.yaml`.

## Code references
- `analytics/aggregate.go:38 Counter` — declares `Hits`, `Success`, `ErrorTotal` (`gorm:"column:error"`), `TotalRequestTime`, `RequestTime`, plus latency family (`MaxLatency`, `MinLatency`, `TotalLatency`, `Latency`).
- `analytics/aggregate.go:800-815` — counter initialization with `Hits: 1`, `RequestTime`, `TotalRequestTime`, latency min/max/total values.
- `analytics/aggregate.go:816-832` — totals updates: `aggregate.Total.Hits++`, `aggregate.Total.Success++`, `aggregate.Total.ErrorTotal++`, `aggregate.Total.ErrorMap[...]++`.
- `analytics/aggregate.go:992 incrementOrSetUnit` — per-dimension counter merge that propagates hits/success/error/latency into each dimension bucket.

## Evidence
- `analytics/aggregate_test.go:25 TestCode_ProcessStatusCodes` — verifies the response-code bucket counters.
- `analytics/aggregate_test.go:41 TestAggregate_Tags` — exercises hit/success counting on tagged records.
- `analytics/aggregate_test.go:822 TestAnalyticsRecordAggregate_AsChange` and `:1105 TestAnalyticsRecordAggregate_AsTimeUpdate` — counter accumulation in the DB-write paths.
- Satisfying SW child: **SW-REQ-011** (aggregate counter accumulation; "running totals never go backwards").

## Open questions
- Phase 0.6 origin: spun out of SYS-REQ-003 with SYS-REQ-018 (dimensions).
- The Counter struct also tracks `OpenConnections`, `ClosedConnections`, `BytesIn`, `BytesOut`, `UpstreamLatency`; the SYS req only names the four primary families. The other counters are implementation surplus, not an obligation.
- Min-latency has special "do not update on error" handling (`analytics/aggregate.go:850-857`) which is correct but unstated at the SYS level.
