# SW-REQ-009: AnalyticsRecord deterministic field-name accessors

## Intent
Realises the deterministic-projection part of parent **SYS-REQ-002**. `AnalyticsRecord.GetFieldNames` returns the canonical ordered list of field labels for an analytics record, composed by hand from a top-level `[]string` plus the field-name slices contributed by the nested `Geo`, `Network`, and `Latency` sub-structs. The matching `GetLineValues` accessors return values in the same order. The record carries helpers (`IsGraphRecord`, `IsMCPRecord`, `RemoveIgnoredFields`, `TrimRawData`, `SetExpiry`) that pumps use without ever ranging over a Go `map`.

## Motivation
The CSV pump and several others need a stable column order across processes; Go's `map` iteration order is randomized per process, so any field projection driven by `structs.Map(...)` ranging would shuffle columns from run to run. Pinning the order in code (static `[]string`) is the simplest correctness fix; pinning it at the *SW component* layer locks future contributors out of "clean refactors" that swap to reflection. Trade-off: adding a field is now a three-touch change (struct field, `GetFieldNames` slice, `GetLineValues` slice) and the build does not catch a missed update — only the CSV roundtrip tests do.

## Code references
- `analytics/analytics.go:191 (*AnalyticsRecord).GetFieldNames` — hand-rolled `fields := []string{"Method", "Host", …}`, then `append(fields, Geo.GetFieldNames()...)`, etc.; trailing `Tags`/`Alias`/`TrackPath`/`ExpireAt`/`ApiSchema` appended last.
- `analytics/analytics.go:160 NetworkStats.GetFieldNames`, `:170 Latency.GetFieldNames`, `:179 GeoData.GetFieldNames` — sub-struct field lists.
- `analytics/analytics.go:268 (*AnalyticsRecord).GetLineValues` — value projector in matching order.
- `analytics/analytics.go:104 IsMCPRecord`, `:415 IsGraphRecord` — boolean discriminators on the embedded stats sub-structs.

## Evidence
- `analytics/analytics_test.go:111 TestAnalyticsRecord_GetFieldNames` — tagged `// Verifies: SW-REQ-009`; asserts the exact slice content + ordering.
- `analytics/analytics_test.go:163 TestAnalyticsRecord_GetLineValues`, `:188 TestLatency_GetFieldNames`, `:203 TestLatency_GetLineValues` — companion ordering tests.
- `analytics/analytics_test.go:15 TestAnalyticsRecord_IsGraphRecord`, `:100 TestAnalyticsRecord_Base` — accessor coverage.
- Field-removal behavior is owned by **SW-REQ-076**, including `analytics/analytics_test.go:TestAnalyticsRecord_RemoveIgnoredFields`.

## Open questions
- `Latency.GetFieldNames` returns three entries including `Latency.Gateway` but `NetworkStats.GetLineValues` only emits four values (OpenConnections, ClosedConnection, BytesIn, BytesOut). The asymmetry is intentional (`Latency` has a gateway field, others do not) but any future addition to `NetworkStats` requires touching both lists in tandem.
- There is no test that asserts `GetFieldNames().length == GetLineValues().length` for `AnalyticsRecord` itself — if the two slices drift, the failure mode is CSV columns shifted by one, which is silent until a downstream reader complains.
