# SYS-REQ-031: Gateway uses msgpack or protobuf with suffix-based selection (environmental assumption)

## Intent
The Tyk gateway encodes analytics records using either msgpack (at the bare key) or protobuf (at the `_protobuf`-suffixed key); no other serializers are produced. This is an environmental assumption that satisfies parent **STK-REQ-001** by surfacing the gateway-side encoding contract tyk-pump consumes but does not control.

## Motivation
tyk-pump dispatches decoding via a serializer table keyed off the analytics-key suffix: bare keys go through `MsgpSerializer`, `_protobuf`-suffixed keys go through `ProtobufSerializer`. If the gateway emitted a third encoding (e.g. JSON, Avro) at a different suffix, tyk-pump would silently never select it; if it stopped emitting one of the two, the corresponding `GetAndDeleteSet` call would just always return empty. Capturing the encoding-and-suffix convention as a SYS-layer assumption pairs with SYS-REQ-030 (key naming) to fully describe the gateway producer contract, and makes any future "let's add an Arrow serializer" gateway proposal recognise that it requires a coordinated tyk-pump change.

## Formalization
```
gateway_integration shall always satisfy gateway_uses_msgpack_or_protobuf
```
This is an environmental input invariant: whenever a Tyk gateway publishes records consumed by this tyk-pump, the encoding is one of the two and the suffix convention holds. There is no trigger — the assumption must hold for any record the pump successfully decodes. Truth condition is owned by the gateway's analytics serializer factory.

## Code references
- `serializer/serializer.go:16-17` — `MSGP_SERIALIZER = "msgpack"` and `PROTOBUF_SERIALIZER = "protobuf"` — the two values the codebase understands.
- `serializer/serializer.go:20 NewAnalyticsSerializer` — factory that returns the matching implementation; defaults to msgpack for unknown values.
- `serializer/msgp.go:30 GetSuffix() string` — msgpack suffix (empty string, the "bare" key).
- `serializer/protobuf.go:16 return "_protobuf"` — the protobuf suffix the gateway must emit.
- `main.go:276-277 for _, serializerMethod := range AnalyticsSerializers { analyticsKeyName += serializerMethod.GetSuffix() }` — the dispatch loop that turns the suffix table into per-encoding key reads.

## Evidence
- External-owner review: `assumption.external_owner: team:tyk-gateway`, status `open`, reviewed via `proof req assumptions review` in Phase B (`verification.review.comment`: "external-owner reviewed (Redis protocol / MongoDB / Tyk gateway / MaxMind)"). Next review date: `2026-11-30`.
- `serializer/serializer_test.go:21-29` round-trips both encodings, asserting they encode/decode the same `AnalyticsRecord` shape that the gateway emits.
- Related KI: `.proof/known-issues/serializer-protobuf-loses-city-names.yaml` — documents a *known* divergence in the protobuf encoding (city names dropped). The assumption is that the gateway *uses* protobuf at the `_protobuf` suffix, not that the protobuf schema is loss-free.

## Open questions
- The assumption is strict ("msgpack or protobuf"); a hypothetical third encoding would silently fall through to the default `MsgpSerializer` in `NewAnalyticsSerializer`, producing garbage decodes. There is no startup check that the registered serializers match what the gateway is producing.
- No direct KI on this assumption, but the linked serializer-protobuf-loses-city-names KI is a reminder that *encoding parity* between the two formats is an additional, separate concern not covered here.
- The suffix convention (`""` for msgpack, `"_protobuf"` for protobuf) is encoded on both sides — gateway and pump — but the two are not generated from a shared schema. Operator-facing follow-up: a shared constants module would prevent drift if the gateway team ever renamed the suffix.
