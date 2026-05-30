# SYS-REQ-002: Analytics record fields preserved on forward

## Intent
When the pump forwards an analytics record to any backend, every field of the record (HTTP request metadata, identity, geo, latency, network, tags, GraphQL stats, MCP stats) is preserved end-to-end. This determinism guarantee satisfies the parent **STK-REQ-001** ("analytics records produced by gateways must be moved to downstream backends" — implicit: without silent field loss).

## Motivation
Downstream consumers (dashboards, billing systems, MDCB) trust the pump as a structurally lossless conduit: any column the gateway produced should be queryable downstream. Capturing this at the SYS layer prevents accidental schema erosion as new fields (e.g. `MCPStats`, `GraphQLStats`) are added — both serializers and the per-pump filter path must keep round-tripping the full record. The `record_forwarded shall always satisfy record_fields_preserved` formulation makes the obligation observable at every dispatch event, not just at end-of-day audits.

## Formalization
```
when record_forwarded model shall always satisfy record_fields_preserved
```
The input `record_forwarded` fires whenever a record leaves `execPumpWriting`; the output `record_fields_preserved` holds when the dispatched record exposes the same field set as the source `AnalyticsRecord`. Variables: `specs/system/variables/model.vars.yaml`.

## Code references
- `analytics/analytics.go:46 AnalyticsRecord` — canonical struct definition (all the fields the obligation covers).
- `analytics/analytics.go:191 GetFieldNames` — declares the deterministic field list used by the SQL/CSV writers.
- `serializer/serializer.go:10 AnalyticsSerializer` interface — `Encode` / `Decode` are the binary serialization roundtrip.
- `serializer/msgp.go` and `serializer/protobuf.go` — the two implementations; both invoked in the purge loop via `AnalyticsSerializers` at `main.go:90`.
- `main.go:310 PreprocessAnalyticsValues` — decode entry point that materializes records before fan-out.

## Evidence
- `serializer/serializer_test.go:18 TestSerializer_Encode`, `:50 TestSerializer_Decode`, `:90 TestSerializer_MCPStats_Roundtrip`, `:133 TestSerializer_NonMCP_NoMCPStats` — round-trip preservation across both serializers.
- `analytics/analytics_test.go` and `analytics/graph_record_test.go`, `analytics/mcp_record_test.go` exercise the field-name and extraction surfaces.
- Satisfying SW children: **SW-REQ-008** (serializer selection / round-trip), **SW-REQ-009** (record model field-name accessors), **SW-REQ-013** (GraphQL extraction), **SW-REQ-014** (MCP extraction).

## Open questions
- "Preserved" is unqualified for transformations operators explicitly opt into: `omit_detailed_recording`, `ignore_fields`, `TrimRawData`, base64 decode all mutate the record in `filterData` (`main.go:378`). These are governed by SYS-REQ-010/011/015/016 — the relationship (this req is the default; the others are sanctioned exceptions) is not encoded in FRETish.
- Per-pump conversions (e.g. SQL columns dropping `RawRequest` into `rawrequest`) are out of scope at the SYS layer; SW-REQ-018..029 each carry their own field-handling fidelity claims.
