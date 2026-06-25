# SW-REQ-105: Kinesis KMS stream-state reconciliation

## Intent
When `KMSKeyID` is configured, the Kinesis pump must reconcile the stream's
current server-side encryption state against that configured key before Init
can succeed.

The accepted states are precise:

- Already KMS-encrypted with the same non-empty key id: accept and do not call
  `StartStreamEncryption`.
- Already KMS-encrypted with a different non-empty key id: fail Init.
- KMS encryption reported with no key id: do not treat it as verified; call
  `StartStreamEncryption` with the configured key.
- Not encrypted: call `StartStreamEncryption` with the configured key.
- `DescribeStream` errors and non-idempotent start-encryption errors: fail Init.
- `ResourceInUseException` from `StartStreamEncryption`: accept as idempotent
  in-progress encryption.

## Motivation
TT-14473 added Kinesis KMS support. The follow-up `50e5f51` narrowed the
"already encrypted" branch so a KMS state with a missing key id does not look
like a different configured key. This requirement pins that state-machine
behavior directly instead of relying on broad Kinesis pump prose.

## Formalization
```
when kms_key_configured pumps_aws_kinesis shall always satisfy stream_kms_key_state_reconciled
```

Variables are declared in `specs/software/variables/pumps-aws-kinesis.vars.yaml`.

## Code References
- `pumps/kinesis.go:KinesisPump.Init` performs the `DescribeStream` check and
  calls `StartStreamEncryption` when the configured KMS key is not already
  verified on the stream.

## Evidence
- `pumps/kinesis_test.go:TestKinesisPump_DescribeStream_AlreadyEncryptedSameKey`
  covers the same-key no-op path.
- `pumps/kinesis_test.go:TestKinesisPump_DescribeStream_AlreadyEncryptedDifferentKey`
  covers the different-key failure path.
- `pumps/kinesis_test.go:TestKinesisPump_DescribeStream_KMSEncryptedMissingKeyID_StartsEncryption`
  covers the `50e5f51` missing-key-id boundary.
- `pumps/kinesis_test.go:TestKinesisPump_DescribeStream_NotEncrypted_StartEncryptionSuccess`,
  `TestKinesisPump_DescribeStream_NotEncrypted_StartEncryptionResourceInUse`,
  `TestKinesisPump_DescribeStream_NotEncrypted_StartEncryptionGenericError`,
  and `TestKinesisPump_DescribeStream_APIFailure` cover start-encryption and
  describe failure states.
