# INT-REQ-003: Serializer round-trip fidelity (msgpack / protobuf)

## Intent
The gateway encodes each analytics record before pushing it to the
temporal store; the pump decodes it on consumption. The two sides agree on
which encoding to use via a suffix appended to the analytics key
(empty for msgpack, `_protobuf` for protobuf). This requirement asserts
that whichever encoding is in use, the round-trip preserves every
analytics record field. It satisfies SYS-REQ-002.

## Motivation
This is the underwriting guarantee for STK-REQ-001's "without loss of the
fields operators rely on". If the encoder strips, truncates, or coerces a
field, the operator-visible result is silent data loss with no error path
to detect it. The serializer is a particularly easy place for that to
happen because the gateway-side and pump-side schemas evolve independently
and the protobuf path goes through a separate `*proto.AnalyticsRecord`
type that has to be kept in sync with the canonical `analytics.AnalyticsRecord`.

The key-suffix discriminator is what allows both encodings to coexist on
the same Redis instance during a gradual rollout; the pump consumes both
suffix variants on every purge cycle.

## Code references
Wire-format suffix constants and call sites:
- `serializer/serializer.go:10-14` — `AnalyticsSerializer` interface with
  `Encode`, `Decode`, `GetSuffix`.
- `serializer/serializer.go:16-17` — `MSGP_SERIALIZER = "msgpack"`,
  `PROTOBUF_SERIALIZER = "protobuf"`.
- `serializer/msgp.go:30` — msgpack `GetSuffix()` returns `""`.
- `serializer/protobuf.go:15` — protobuf `GetSuffix()` returns `"_protobuf"`.
- Suffix is appended at `main.go:277`:
  `analyticsKeyName += serializerMethod.GetSuffix()`.
- Both serializers are initialised at `main.go:90`:
  `[]serializer.AnalyticsSerializer{NewAnalyticsSerializer(MSGP_SERIALIZER),
  NewAnalyticsSerializer(PROTOBUF_SERIALIZER)}` — the pump iterates both
  every cycle.
- Round-trip code:
  - msgpack: `serializer/msgp.go:12` `Encode`, `serializer/msgp.go:17`
    `Decode`.
  - protobuf: `serializer/protobuf.go:20` `Encode` (calls
    `TransformSingleRecordToProto`), `serializer/protobuf.go:26` `Decode`
    (calls `TransformSingleProtoToAnalyticsRecord` at line 140).
- Canonical record struct: `analytics/analytics.go:46`
  `AnalyticsRecord` (47 fields including embedded Geo/Network/Latency,
  GraphQL and MCP stats).

## Evidence
- `serializer/serializer_test.go` and
  `serializer/serializer_branches_test.go` exercise both encoders and the
  decode-error branches.

## Open questions
- The protobuf round-trip at `serializer/protobuf.go:169` sets
  `City.Names: nil` on decode, even though the encode side does pass
  `rec.Geo.City.Names`. This is a genuine round-trip loss for the protobuf
  path that contradicts the literal "round-trip every field without loss"
  text of this requirement.
- The protobuf GraphQLStats path flattens `GraphError` to message strings.
  `GraphError.Path` is not represented in `analytics.proto` and is tracked by
  KI `serializer-protobuf-loses-graphql-error-path`.
- The protobuf path also does not round-trip `APIKey` on the decode side
  — only the encode side (`protobuf.go:76`) writes it; the decode tmpRecord
  at `protobuf.go:153` re-reads `rec.APIKey` so this one is OK, but it is
  worth tightening the test coverage to catch the next field that gets
  added to one side and not the other.
- The contract does not specify a schema-version field on either encoder,
  so adding a new field to `AnalyticsRecord` is silently
  forward-incompatible with older pumps that haven't been rebuilt against
  the new proto definitions.
