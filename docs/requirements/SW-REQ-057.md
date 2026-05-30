# SW-REQ-057: AWS Timestream pump — multimeasure record forwarding

## Intent
The `TimestreamPump` shall, on each purge, split records into batches of
`timestreamMaxRecordsCount=100` (the AWS hard limit), convert each record
into a multimeasure `types.Record` with operator-configured `Dimensions`
and `Measures` (with optional `NameMappings` rename, optional zero-value
emission, and optional `RateLimit` measure inclusion), and submit each
batch via `WriteRecords` to the configured `DatabaseName`/`TableName`. The
first batch error shall be returned to the caller;
`RejectedRecordsException` shall be logged with the first rejection
reason. Derived from SYS-REQ-004 via Phase A decomposition of SW-REQ-028.

## Motivation
Timestream is AWS's purpose-built time-series store; this pump exists for
operators who want analytics in a service optimised for time-window
queries. The pump propagates write errors honestly (unlike the kinesis
sibling) and applies the AWS 100-record `WriteRecords` cap as a hard
boundary. Splitting it out of SW-REQ-028 lets the `boundary` obligation
on the AWS cap and the per-record rename / zero-value / rate-limit
operator toggles be auditable.

## Code references
- `pumps/timestream.go:27-29` — `TimestreamWriteRecordsAPI` interface (for
  testability).
- `pumps/timestream.go:46-76` — `TimestreamPumpConf` (region, table,
  database, dimensions, measures, write_rate_limit, name_mappings).
- `pumps/timestream.go:122` — `WriteData`'s `WriteRecords` call.
- `pumps/timestream.go:TimestreamPump.BuildTimestreamInputIterator` —
  batch iteration.
- `pumps/timestream.go:MapAnalyticRecord2TimestreamMultimeasureRecord`,
  `:GetAnalyticsRecordDimensions`, `:GetAnalyticsRecordMeasures` — field
  projection helpers.

## Evidence
- `pumps/timestream_test.go` (re-annotated `Verifies: SW-REQ-057`).
- Live-Timestream tests need real AWS credentials and are excluded from
  the local audit MC/DC scope (known issue).

## Open questions
- The `boundary` obligation lives on the AWS-hard-limit batch-size cap
  (100); operators cannot exceed it.
- Field projection silently drops fields not in the operator-configured
  dimension/measure mapping; reviewers should document the supported
  field set.
