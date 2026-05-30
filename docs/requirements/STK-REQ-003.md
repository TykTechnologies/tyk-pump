# STK-REQ-003: Operator control over what analytics leave the system

## Intent
Operators have privacy, regulatory, and cost obligations that mean not every
record produced by the gateway should be forwarded, and not every byte of a
record should leave the perimeter. This stakeholder requirement captures the
need for fine-grained, per-backend operator control: allow/block lists on
organisation, API and response code; omission of raw request/response
payloads; and bounded record size. Configuration is sourced from a file plus
environment variables so it can be operationalised in containerised
deployments.

## Motivation
The pump is frequently deployed in environments where the gateway-side data
is rich enough to include PII (raw HTTP bodies, IP addresses), and where the
downstream backends are subject to different governance (a SIEM may be
allowed to see everything; a marketing analytics DB may not). Without the
controls in this requirement, operators would be forced to either deploy
multiple gateways or sanitise records out-of-band — both expensive and
error-prone. The same controls double as cost containment: trimming raw
payloads and dropping unwanted orgs/APIs keeps downstream storage bills
predictable.

The env-var-overlay angle exists because the pump is overwhelmingly run in
container orchestrators where secrets and per-environment overrides are
injected at deploy time. A file-only configuration would force config
rebuilds for every secret rotation.

## Code references
Decomposes into the following SYS reqs via its acceptance criteria:
- AC-001 (allow/block per backend): `SYS-REQ-008` (config from JSON +
  env-var overrides), `SYS-REQ-009` (per-backend filter precedence:
  block-list wins, allow-list as gate), `SYS-REQ-020` (every field
  overridable via `TYK_PMP_<FIELD>`).
- AC-002 (omit payloads, cap size): `SYS-REQ-010` (truncate over max size),
  `SYS-REQ-011` (base64-decode if enabled — note: relevant to payload
  handling), `SYS-REQ-015` (omit-detailed-recording drops raw payloads),
  `SYS-REQ-016` (drop operator-listed ignore_fields).

Implementation pointers:
- Filter struct & semantics: `analytics/analytics_filters.go:3`
  `AnalyticsFilters`, `analytics/analytics_filters.go:19` `ShouldFilter`.
- Trim and omit: `analytics/analytics.go:296` `TrimRawData`, and the
  `RawRequest=""`/`RawResponse=""` clearing at `main.go:397`.
- Per-pump config plumbing: `pumps/common.go` (`SetFilters`, `SetMaxRecordSize`,
  `SetOmitDetailedRecording`, `SetIgnoreFields`).
- Config loading: `config.go:262` `LoadConfig` (JSON), `config.go:282`
  `envconfig.Process(ENV_PREVIX, ...)` for env-var overlay.

## Evidence
- `analytics/analytics_filters_test.go` covers allow/block list logic
  including precedence.
- Trim and omit coverage in `analytics/coverage2_test.go` and the SW-REQ-009 /
  SW-REQ-010 marked tests.
- Env-var overlay coverage: `config_test.go` exercises `LoadConfig` plus
  env overrides.

## Open questions
- "Per-backend" allow/block lists are realised by `pmp.SetFilters(pmp.Filters)`
  in `main.go:208` — but the global `SystemConfig.MaxRecordSize` at
  `main.go:404` overrides only when the pump-level value is zero. There is
  no per-backend allow-list for fields (only ignore-fields, which is a
  block-list); operators wanting "only forward these fields" must invert
  manually.
- The deprecated global `raw_request_decoded` / `raw_response_decoded`
  settings (`config.go:255`) are documented as having no effect, but the
  warning is emitted only at startup (`main.go:56`); operators who set them
  may still expect global behaviour. No SYS req covers the deprecation
  pathway.
