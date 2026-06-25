# SW-REQ-103: REST aggregate HTTP error boundary

Documents: SW-REQ-103

## Contract

REST aggregate calculation treats HTTP status codes greater than or equal to
400 as aggregate errors. Those records increment `Total.ErrorTotal`,
`Total.ErrorMap["<status>"]`, and the matching per-status `Errors["<status>"]`
dimension. Status codes below 400 do not increment aggregate error totals.

This is a child of SW-REQ-011. SW-REQ-011 owns monotonic aggregate counter
accumulation; SW-REQ-103 pins the HTTP status boundary that decides whether the
error counters receive the current record.

## Evidence

- `analytics/aggregate_test.go:TestAggregateData_ResponseCode400CountsAsErrorBoundary`
  feeds 399, 400, 500, and 200 records through `AggregateData`, then asserts
  that 400 and 500 are counted as errors while 399 is not.

## Historical Context

Commit `c02a2cb` fixed the predicate from `>400` to `>=400`. Without this
boundary, HTTP 400 records were present in aggregate hits but missing from
aggregate error totals and per-status error dimensions.
