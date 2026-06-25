# SW-REQ-055: AWS SQS pump — batched SendMessage with optional FIFO

## Intent
The `SQSPump` shall, on each purge, JSON-marshal each analytics record into
a `SendMessageBatchRequestEntry` (Id generated as a ULID), optionally
attaching `AWSMessageGroupID`, `MessageDeduplicationId` (when
`AWSMessageIDDeduplicationEnabled` is true), and `DelaySeconds`. The pump
shall then submit batches of `AWSSQSBatchLimit` entries via
`SendMessageBatch` on the resolved queue URL. The first batch error shall
be returned to the caller. Derived from SYS-REQ-004 via Phase A
decomposition of SW-REQ-028.

## Motivation
SQS is the AWS queue option for analytics pipelines that want at-least-once
delivery and decoupled consumers (e.g. Lambda processors). The pump
supports FIFO queues via `MessageGroupId` and per-message dedup. Splitting
it out of SW-REQ-028 makes the per-batch error-propagation guarantee
(which actually holds, unlike the kinesis sibling) explicit, and surfaces
the AWS 10-record hard cap on `SendMessageBatch` as a separate boundary
concern.

## Code references
- `pumps/sqs.go:29-78` — `SQSPump`, `SQSConf` (queue, region, credentials,
  message-group ID, batch limit, dedup toggle).
- `pumps/sqs.go:SQSPump.WriteData` — orchestrator.
- `pumps/sqs.go:131-175` — entry builder (ULID Id; optional
  `MessageGroupId`).
- `pumps/sqs.go:178-197` — `write` chunks by `AWSSQSBatchLimit`.
- `pumps/sqs.go:SQSPump.NewSQSPublisher` — SDK constructor.

## Evidence
- `pumps/sqs_test.go` (re-annotated `Verifies: SW-REQ-055`).
- Live-SQS tests need real AWS credentials and are excluded from the local
  audit MC/DC scope (known issue).

## Open questions
- The code has a duplicate `MessageGroupId` assignment (lines 151 and 159)
  — harmless but worth a cleanup pass.
- `AWSSQSBatchLimit` is operator-configurable; AWS hard-caps
  `SendMessageBatch` at 10 entries and will reject larger batches. Worth a
  separate `boundary` follow-up to clamp operator input.
- Mixed malformed input currently leaves zero-value entries in the preallocated
  SQS batch; this is tracked by KI
  `sqs-malformed-record-sends-empty-entry`.
