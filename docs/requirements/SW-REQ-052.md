# SW-REQ-052: HTTP Moesif pump — masking, sampling, and SDK enqueue

## Intent
The `MoesifPump` shall, on each purge, parse each analytics record's
base64-encoded raw request/response, apply operator-configured header/body
masking and capture toggles, identify the user via the configured
`UserIDHeader` (or fall back to record `Alias` / `OauthID` / parsed
Authorization header), apply per-user / per-company sample-rate filtering
fetched from the Moesif app-config endpoint, and enqueue qualifying events
into the Moesif SDK with a weight equal to `floor(100 / samplingPercentage)`.
The app-config sampling rules shall be refreshed once per minute when the
SDK's `X-Moesif-Config-Etag` differs from the cached `eTag`. Derived from
SYS-REQ-004 via Phase A decomposition of SW-REQ-027.

## Motivation
Moesif has the most behaviourally complex pump in the HTTP-logging family:
header/body masking, per-user/per-company sampling, weighted events to
preserve aggregate accuracy, and an etag-driven config refresh. Splitting
it out of SW-REQ-027 makes those unique behaviours auditable and surfaces
the `untrusted_input_bounded` obligation (the masking/parsing path operates
on operator-untrusted headers and bodies).

## Code references
- `pumps/moesif.go:MoesifPump.WriteData` — the orchestrator.
- `pumps/moesif.go:MoesifPump.parseConfiguration` — app-config refresh.
- `pumps/moesif.go:MoesifPump.getSamplingPercentage` — per-user/per-company
  sample-rate resolution.
- `pumps/moesif.go:decodeRawData`, `:maskRawBody`, `:maskData` —
  masking pipeline.
- `pumps/moesif.go:fetchIDFromHeader`, `:parseAuthorizationHeader`,
  `:buildURI` — user identification.

## Evidence
- No dedicated `moesif_test.go` exists today (coverage gap).
- The previous family req SW-REQ-027 is retained as a `[SUPERSEDED by
  Phase A decomposition: ...]` anchor.

## Open questions
- Multiple `p.log.Fatal(err)` calls on base64-decode failures (lines 349,
  357, 380, 387) — same `log.Fatal` anti-pattern as graylog/syslog.
  Honest obligation_class is `nominal`, not `errors_propagated`.
- `QueueEvent` error is logged but not returned.
- Per-record `rand.Seed(time.Now().UnixNano())` at lines 472-473 is both a
  determinism quirk and a perf regression worth flagging.
- No test file — gap.
