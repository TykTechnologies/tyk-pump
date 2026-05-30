# SW-REQ-053: HTTP Segment pump — Segment Track event forwarding

## Intent
The `SegmentPump` shall, on each purge, marshal each analytics record to
JSON and submit it as a Segment `Track` event with `Event="Hit"` and
`AnonymousId` set to the record's `APIKey`, via the
`segmentio/analytics-go` SDK using the configured `segment_write_key`.
Derived from SYS-REQ-004 via Phase A decomposition of SW-REQ-027.

## Motivation
Segment is a customer-data pipeline that operators may use to route
analytics into downstream tools (CRMs, marketing automation, data
warehouses). This pump is the smallest of the HTTP-logging family —
fire-and-forget into the Segment SDK with no batching or retry. Splitting
it out of SW-REQ-027 makes the no-retry / no-batch shape explicit so
reviewers don't assume the family-level claims of error propagation or
batching apply.

## Code references
- `pumps/segment.go:SegmentPump.WriteData` — per-record `Track` submission.
- `pumps/segment.go:WriteDataRecord` — per-record helper.
- `pumps/segment.go:ToJSONMap` — record → flat map conversion.

## Evidence
- `pumps/segment_test.go` (re-annotated `Verifies: SW-REQ-053`).
- Live-Segment tests need a real write key and are excluded from the local
  audit MC/DC scope (known issue).

## Open questions
- SDK `Track` error is logged but not returned; per-record marshalling
  error is logged but `WriteDataRecord` always returns nil; `WriteData`
  always returns nil. Honest obligation_class is `nominal`.
- No batching — every record is one SDK call. Could be a perf issue at
  high throughput; operators rely on the SDK's internal flushing.
