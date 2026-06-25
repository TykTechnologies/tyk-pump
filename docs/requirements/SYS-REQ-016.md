# SYS-REQ-016: Operator-listed fields removed before forwarding

## Intent
The pump removes operator-listed fields (identified by their JSON tag) from each analytics record before forwarding to a backend. This satisfies parent **STK-REQ-003** (operator-configurable behaviour) as a fine-grained privacy / shape-control mechanism for backends that should never see specific fields (e.g. drop `api_key` before sending to a third-party log aggregator).

## Motivation
Whereas `omit_detailed_recording` is a coarse "drop the raw bodies" switch, `ignore_fields` lets operators surgically remove any JSON-tagged field of `AnalyticsRecord`. Codifying this at the SYS layer makes it a first-class privacy lever and locks in the contract that the match is by JSON tag — not by Go field name — which matters for downstream documentation and for refactors that might rename Go fields without updating tags.

## Formalization
```
when ignore_fields_configured privacy shall always satisfy listed_fields_removed
```
The input `ignore_fields_configured` is true when `len(pump.GetIgnoreFields()) > 0`; the output `listed_fields_removed` becomes true once `RemoveIgnoredFields(ignoreFields)` has zero'd each matching field. Variables: `specs/system/variables/privacy.vars.yaml`.

## Code references
- `main.go:414-416` — `if len(ignoreFields) > 0 { decoded.RemoveIgnoredFields(ignoreFields) }` inside `filterData`.
- `analytics/analytics.go:420 RemoveIgnoredFields` — iterates `structs.Fields(a)`, matches `field.Tag("json")` against the ignore list, calls `field.Zero()` on hit; logs an error when an ignored tag is not found.
- `pumps/common.go:85 SetIgnoreFields` / `:90 GetIgnoreFields`.
- `main.go:212 thisPmp.SetIgnoreFields(pmp.IgnoreFields)` — wired from JSON config per pump.

## Evidence
- `main_test.go:TestIgnoreFieldsFilterData` — exercises per-pump ignore lists.
- `analytics/analytics_test.go:TestAnalyticsRecord_RemoveIgnoredFields` — verifies direct JSON-tag field zeroing.
- Satisfying SW child: **SW-REQ-076** (filterData + RemoveIgnoredFields behavior).
- Supporting wiring: **SW-REQ-016** covers the common-pump `SetIgnoreFields` / `GetIgnoreFields` storage surface.

## Open questions
- Match is by JSON tag, so a field with no `json:` tag cannot be ignored. The SYS req doesn't constrain matching semantics.
- Removal is implemented as "set to zero value", not "delete". For a backend that introspects record schemas, zero values are indistinguishable from absent fields — the SYS req says "removed" which is ambiguous. In practice this is fine for JSON pumps (zero values still serialize) and SQL pumps (zero column values) but operators expecting structural absence may be surprised.
- The match scope is the top-level `AnalyticsRecord` only; nested structures (`Geo.Country.ISOCode`, `Network.BytesIn`) are not addressable by the ignore list because `RemoveIgnoredFields` does not recurse.
