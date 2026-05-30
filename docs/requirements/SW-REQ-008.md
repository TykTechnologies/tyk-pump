# SW-REQ-008: Analytics serializer (msgpack / protobuf) with key-suffix dispatch

## Intent
Realises the serialization contract of parent **SYS-REQ-002**. The `serializer` package exposes an `AnalyticsSerializer` interface with `Encode`, `Decode`, and `GetSuffix` methods, and a factory `NewAnalyticsSerializer(serializerType)` that returns either `*MsgpSerializer` (default; empty suffix) or `*ProtobufSerializer` (suffix `"_protobuf"`). `main.StartPurgeLoop` constructs both at startup and walks them in order, appending each `GetSuffix()` to the base analytics key — so the gateway can write msgpack records to `tyk-system-analytics` and protobuf records to `tyk-system-analytics_protobuf` and the pump consumes both lists in the same purge cycle.

## Motivation
Two on-wire formats are needed because the gateway can be configured to emit either (msgpack for legacy, protobuf for the newer transport). Keying the dispatch off the *Redis-list key suffix* rather than off a per-record header lets the pump remain stateless: it does not have to peek at any record bytes to decide which decoder to use. Trade-off: each purge cycle does N (= number of serializers) extra `GetAndDeleteSet` calls per shard, most of which return empty lists; cheap, but visible in the Redis command-rate metric.

## Code references
- `serializer/serializer.go:10 AnalyticsSerializer` — interface.
- `serializer/serializer.go:20 NewAnalyticsSerializer` — `switch serializerType` factory, defaults to msgpack.
- `serializer/msgp.go:12 Encode` / `:17 Decode` / `:30 GetSuffix` — msgpack impl, empty suffix.
- `serializer/protobuf.go:15 GetSuffix` — returns `"_protobuf"`.
- `serializer/protobuf.go:20 Encode`, `:26 Decode`, `:36 TransformSingleRecordToProto`, `:140 TransformSingleProtoToAnalyticsRecord` — full bidirectional mapping including GraphQL and MCP sub-records.
- `main.go:90` — `AnalyticsSerializers = []AnalyticsSerializer{NewAnalyticsSerializer(MSGP_SERIALIZER), NewAnalyticsSerializer(PROTOBUF_SERIALIZER)}`.
- `main.go:277` — `analyticsKeyName += serializerMethod.GetSuffix()` inside the per-shard loop.

## Evidence
- `serializer/serializer_test.go:18 TestSerializer_Encode`, `:50 TestSerializer_Decode`, `:90 TestSerializer_MCPStats_Roundtrip`, `:133 TestSerializer_NonMCP_NoMCPStats`, `:153 TestSerializer_GetSuffix` — all tagged `// Verifies: SW-REQ-008`; round-trip and suffix dispatch coverage.
- `serializer/serializer_branches_test.go:13 TestSerializer_RichRecordRoundtrip` — tagged `// SW-REQ-008:encoding_safety:negative`; exercises GraphQL-stats + Geo fields through msgpack and protobuf.
- `serializer/serializer_branches_test.go:50 TestSerializer_Decode_Malformed` — tagged `// SW-REQ-008:encoding_safety:negative`; both decoders return an error on garbage input rather than panicking.
- `serializer/serializer_branches_test.go:63 TestNewAnalyticsSerializer_DefaultsToMsgp` — verifies the factory default branch.

## Open questions
- The protobuf path goes through a hand-written translator (`TransformSingleRecordToProto` / `TransformSingleProtoToAnalyticsRecord`) — any new field added to `AnalyticsRecord` requires three coordinated edits (the struct, the proto, both translators). There is no compile-time guard; missing-field bugs only surface in roundtrip tests.
- `MsgpSerializer.Decode` accepts both `string` and `[]byte` via a type switch, but `ProtobufSerializer.Decode` asserts `analyticsData.([]byte)` unconditionally — a `string` input panics. The two serializers are not interface-compatible on the input type, which is undocumented.
