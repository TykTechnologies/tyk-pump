# SW-REQ-056: AWS Kinesis pump — batched PutRecords with random partition keys

## Intent
The `KinesisPump` shall apply Kinesis environment overrides during
configuration through `TYK_PMP_PUMPS_KINESIS_META_*` by default, or through
the custom `meta_env_prefix` when one is configured. On each purge, it shall
split records into batches of `BatchSize` (default 100), JSON-marshal each
record into a Kinesis
`PutRecordsRequestEntry`
with `PartitionKey` set to a fresh `crypto/rand` integer (for even
MD5-based shard distribution), and submit each batch via `PutRecords` to
the configured `StreamName`. When `KMSKeyID` is configured at Init, the
pump shall verify the stream's current encryption mode/key, enable KMS
server-side encryption if not already enabled with that key, and fail Init
if the stream is already encrypted with a different key. Derived from
SYS-REQ-004 via Phase A decomposition of SW-REQ-028.

## Motivation
Kinesis is the AWS streaming option for analytics pipelines that need
per-shard ordering and high throughput. The random partition key
distributes records evenly across shards (Kinesis maps partition keys via
MD5 to shard hash ranges); operators who need per-org or per-API ordering
must implement their own partitioning downstream. Splitting Kinesis out of
SW-REQ-028 makes the random-partition-key choice and the KMS-key Init
verification explicit, and surfaces the per-batch error swallowing as an
honest `nominal` rather than the family-level `errors_propagated`.

KMS stream-state reconciliation is split into **SW-REQ-105**. In particular,
KMS encryption is considered verified only when the stream reports the same
non-empty key id; a KMS state with a missing key id must be reconciled by
calling `StartStreamEncryption` with the configured key.

The AWS 500-record maximum for one `PutRecords` request is required but not
currently enforced for operator values above 500. That live product gap is
tracked by KnownIssue `kinesis-batch-size-over-aws-putrecords-limit`; the
default `BatchSize` remains 100.

## Code references
- `pumps/kinesis.go:23-49` — `KinesisPump`, `KinesisConf`.
- `pumps/kinesis.go:64-74` — config decode followed by default/custom env
  override application.
- `pumps/kinesis.go:96-135` — `Init` KMS encryption verification path.
- `pumps/kinesis.go:144-221` — `WriteData`; partition-key generation at
  lines 185-194 uses `rand.Int(rand.Reader, big.NewInt(1000000000))`.
- `pumps/kinesis.go:splitIntoBatches` — configured `BatchSize` slicing.

## Evidence
- `pumps/kinesis_test.go` carries code annotations for SW-REQ-056 behavior.
- `pumps/kinesis_test.go:TestKinesisPump_DefaultEnvOverridesConfig` proves
  the `env_override_applied` obligation for `TYK_PMP_PUMPS_KINESIS_META_STREAMNAME`,
  `REGION`, `BATCHSIZE`, and `KMSKEYID` overriding file/config values.
- `pumps/kinesis_test.go:TestKinesisPump_CustomEnvPrefixOverridesConfig`
  proves the `env_override_applied` boundary where a configured
  `meta_env_prefix` is used instead of the default prefix.
- `pumps/kinesis_test.go:TestKinesisPump_StreamName_Required` and the KMS
  `DescribeStream` tests carry exact MC/DC witness rows for the formal
  `kms_key_configured -> stream_encryption_verified` formula.
- `pumps/kinesis_test.go:TestKinesisSplitIntoBatches_AllowsOverAWSLimit`
  reproduces the open KnownIssue where `batch_size > 500` can produce an
  oversized `PutRecords` request.
- `pumps/kinesis_test.go:TestKinesisPump_DescribeStream_KMSEncryptedMissingKeyID_StartsEncryption`
  covers the SW-REQ-105 missing-key-id boundary from TT-14473 follow-up
  `50e5f51`.
- Live-Kinesis tests need real AWS credentials and are excluded from the
  local audit MC/DC scope (known issue).

## Open questions
- `WriteData` always returns `nil` even when `PutRecords` errors (line
  205-207 only logs) — cannot honestly claim `errors_propagated` at the
  WriteData level. Per-record `ErrorCode` responses are debug-logged only and
  tracked by KnownIssue `kinesis-putrecords-per-record-failures-return-nil`.
- Init does propagate errors (KMS verification, stream lookup).
- `crypto/rand` partition key is good for distribution but expensive at
  high throughput (one syscall per record).
