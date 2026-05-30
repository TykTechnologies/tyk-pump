# SW-REQ-028: AWS pump family — SQS, Kinesis, Timestream record forwarding

## Intent
The AWS pump family shall forward analytics records to AWS services:
- **SQS** sends batched messages with a configured static `MessageGroupId`
  (when set) and ULID `Id` per message, supporting FIFO queues with optional
  ID-based deduplication.
- **Kinesis** writes record entries with cryptographically-random
  `PartitionKey`s (generated via `crypto/rand`) so messages distribute
  evenly across shards via Kinesis's md5-of-partition-key shard mapping.
- **Timestream** writes time-series records via `WriteRecords` to a
  configured database/table, projecting analytics fields into dimensions
  and measures.

Derived from SYS-REQ-004 (independent per-backend delivery).

## Motivation
Per Phase 0 verification: the original requirement claimed Kinesis
partitioned "by organisation"; inspection of `kinesis.go:185-194` showed
the actual implementation generates a random `big.Int` for each record and
uses its string form as the partition key. The corrected description
reflects the actual code: even shard distribution is the goal, not
per-org affinity. (The trade-off is that consumers cannot rely on
per-org ordering — but tyk-pump records are timestamp-ordered globally
anyway.)

The three pumps share an SDK (`aws-sdk-go-v2`) and a credentials/region
model but otherwise have nothing in common: SQS is queue semantics with
optional FIFO ordering; Kinesis is streaming with shard-level ordering;
Timestream is a purpose-built time-series database with dimension/measure
projection. Their `WriteData` methods are correspondingly different:

- SQS batches with `SendMessageBatch` (chunked by `AWSSQSBatchLimit`).
- Kinesis batches with `PutRecords` (default batch 100, max 500 per AWS
  limits) and inspects per-record error codes in the response.
- Timestream batches with `WriteRecords` (max 100 per AWS limits).

Failure modes addressed:
- Lost AWS credentials: each pump's `Init` calls
  `config.LoadDefaultConfig` which fails fast if the credential chain is
  exhausted.
- Per-record failures in Kinesis: `output.Records[].ErrorCode` is logged
  per failed record.
- KMS encryption on Kinesis streams (`KMSKeyID`): `Init` checks current
  encryption state and either enables or no-ops, handling
  `ResourceInUseException` gracefully.

## Code references
- `pumps/sqs.go:29-78` — `SQSPump`, `SQSConf` (queue name, region,
  credentials, message group ID, batch limit, dedup).
- `pumps/sqs.go:131-175` — `WriteData` builds `SendMessageBatchRequestEntry`
  with ULID IDs and optional `MessageGroupId`. Note line 151 vs 159: the
  same `if s.SQSConf.AWSMessageGroupID != ""` block runs twice (apparent
  copy-paste, but harmless — sets the same value twice).
- `pumps/sqs.go:178-197` — `write` chunks by `AWSSQSBatchLimit`.
- `pumps/kinesis.go:23-49` — `KinesisPump`, `KinesisConf` (stream, region,
  batch size, KMS key).
- `pumps/kinesis.go:96-135` — `Init` enables KMS server-side encryption
  when `KMSKeyID` is set, handles existing-encryption cases.
- `pumps/kinesis.go:144-221` — `WriteData`; partition key generation at
  lines 185-194 uses `rand.Int(rand.Reader, big.NewInt(1000000000))` for
  even shard distribution (Phase 0 verified).
- `pumps/timestream.go:27-29` — `TimestreamWriteRecordsAPI` interface
  (for testability).
- `pumps/timestream.go:46-76` — `TimestreamPumpConf` (region, table,
  database, dimensions, measures, write_rate_limit, name_mappings).
- `pumps/timestream.go:122` — `WriteData` calls `WriteRecords`.

## Evidence
- `pumps/sqs_test.go`, `pumps/kinesis_test.go`, `pumps/timestream_test.go`
  cover each pump's mapping and batching logic with mocked AWS clients.
- Live-AWS tests need real credentials and are excluded from the local
  audit MC/DC scope (recorded as a known issue).

## Open questions
- Each AWS service has distinct partitioning semantics (Kinesis random,
  SQS static-group, Timestream none). Phase A should split per-service
  with the actual partitioning policy as an explicit obligation.
- SQS `WriteData` sets `MessageGroupId` twice in a row when configured
  (lines 151-153 and 159-161) — harmless duplication, but worth a cleanup
  pass.
- Kinesis errors are logged from `output.Records[].ErrorCode` but
  `WriteData` always returns `nil` — partial-batch failure is not
  propagated to the core.
- Kinesis partition key uses `crypto/rand` (good for distribution, bad
  for performance under high load — every record requires a syscall).
  No requirement captures this trade-off.
- Timestream's dimension/measure projection silently drops fields not in
  its mapping; the requirement doesn't list the supported field set.
- KMS-key handling in Kinesis is a real obligation but lives in `Init`,
  not `WriteData` — Phase A should capture it as a separate startup
  obligation.
