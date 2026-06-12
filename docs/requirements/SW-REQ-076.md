# SW-REQ-076: Operator ignore-fields removal in filterData

## Intent
When the operator configures a non-empty ignore-fields list for a backend,
`main.filterData` shall invoke `AnalyticsRecord.RemoveIgnoredFields` on each
record before dispatch, zeroing every record field whose json tag matches a
listed name so the listed fields are removed from the forwarded record.
Unrecognised field names are logged and leave the record otherwise unchanged.

## Motivation
This requirement is the software decomposition of `SYS-REQ-016`
(`listed_fields_removed`): the system-level guarantee that operator-listed
fields are removed from forwarded records. The operator-listed field-removal
behaviour is implemented by `analytics.RemoveIgnoredFields` applied in
`main.filterData` under the `len(ignoreFields) > 0` gate, distinct from the
Prometheus api-key masking modeled by `SW-REQ-074`. Pinning it at the software
level keeps the field-removal contract explicit and regression-tested in the
`pump-core` component.

## Code references
- `main.go` `filterData` — applies `RemoveIgnoredFields` per record under the
  non-empty ignore-fields gate (`// reqproof:implements SW-REQ-076`).
- `analytics/analytics.go` `AnalyticsRecord.RemoveIgnoredFields` — zeroes the
  fields whose json tag matches a listed name (`// reqproof:implements
  SW-REQ-076`).
- Tests: `main_test.go` `TestIgnoreFieldsFilterData` (`// Verifies:
  SW-REQ-076`); also exercised by the analytics-package
  `TestAnalyticsRecord_RemoveIgnoredFields`.

## Evidence
Verified by `TestIgnoreFieldsFilterData` with all three MC/DC witness rows
covered:
- `// MCDC SW-REQ-076: ignore_fields_configured=F, listed_fields_removed=F => TRUE`
  (no ignore-fields configured — vacuous TRUE)
- `// MCDC SW-REQ-076: ignore_fields_configured=T, listed_fields_removed=F => FALSE`
  (the violation row: the directive was accepted but the listed fields were NOT
  removed)
- `// MCDC SW-REQ-076: ignore_fields_configured=T, listed_fields_removed=T => TRUE`

Each test case configures `ignore_fields_configured=T` via `SetIgnoreFields` and
asserts the forwarded record equals the expected record with the listed fields
zeroed; an "invalid field" sub-case proves that an unrecognised field name does
not silently remove anything. The requirement is assurance level B, approved,
and trace-clean (`approvals_current`, `suspect_clean`).

## Open questions
- If `RemoveIgnoredFields` ever changes its matching from json tag to struct
  field name, this requirement and its tests must be updated together.
