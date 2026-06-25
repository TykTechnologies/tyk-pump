# SW-REQ-089: Demo analytics timestamps cover the generated hour

## Intent
Demo mode generates synthetic analytics records for requested days and hours.
Each generated record must carry a `TimeStamp` inside the requested synthetic
hour, and the denormalized `Day`, `Month`, `Year`, and `Hour` fields must mirror
that timestamp.

## Motivation
Before TT-5426, demo generation rewrote the date-part fields but left
`TimeStamp` at wall-clock record construction time. Downstream sinks that group,
shard, or query by `TimeStamp` could therefore see demo records clumped at "now"
even while their date-part fields claimed a different synthetic hour.

## Formalization
```
when demo_records_generated analytics_demo shall always satisfy demo_timestamps_cover_generated_hour
```

Variables are declared in `specs/software/variables/analytics-demo.vars.yaml`.

## Code References
- `analytics/demo/demo.go:GenerateDemoData` selects the requested synthetic day
  and hour range.
- `analytics/demo/demo.go:WriteDemoData` assigns `TimeStamp` from the synthetic
  hour and advances records by the per-hour spacing.

## Evidence
- `analytics/demo/demo_test.go:TestWriteDemoDataAssignsSyntheticTimestampsAcrossHour`
  verifies four records in one generated hour are stamped at 15-minute intervals
  and that date-part fields mirror `TimeStamp`.
- `analytics/demo/demo_test.go:TestGenerateDemoData` verifies count and broad
  past/future generation behavior.
