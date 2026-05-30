# SW-REQ-048: HTTP Splunk pump — HEC event POST with TLS and retry

## Intent
The `SplunkPump` shall POST each analytics record (or a batch up to
`BatchMaxContentLength` bytes when `EnableBatch` is true) as a Splunk HEC
event JSON to `CollectorURL` with `Authorization: Splunk <token>`. TLS
configuration (server-name, CA, client cert/key, and the operator-controlled
`SSLInsecureSkipVerify` override) shall be applied via `NewTLSConfig`.
Transient send failures shall be retried up to `MaxRetries` via
`retry.BackoffHTTPRetry`. Send errors shall be returned to the caller. When
`ObfuscateAPIKeys` is true, API keys are masked with `****` plus the
trailing `ObfuscateAPIKeysLength` characters. Derived from SYS-REQ-004 via
Phase A decomposition of SW-REQ-027.

## Motivation
Splunk is the most heavyweight HTTP-logging pump: it batches with a
content-length cap, has its own retry layer, exposes the most TLS knobs
(client cert/key, CA, server name, skip-verify), and supports API-key
obfuscation. Splitting it out of the HTTP-logging family makes those
unique behaviours explicit so reviewers do not conflate Splunk with the
simpler family members. The `cert_validation_strict` obligation appears on
the checklist because `SSLInsecureSkipVerify` is operator-configurable —
the obligation is satisfied by the operator's choice (and the documentation
that surrounds that choice), not by the code statically refusing
relaxation.

## Code references
- `pumps/splunk.go:36-48` — `SplunkClient` (carries `TLSSkipVerify`).
- `pumps/splunk.go:63` — `SSLInsecureSkipVerify bool` (the operator
  override).
- `pumps/splunk.go:118-179` — `Init` builds an `http.Client` with optional
  TLS skip-verify; uses `retry.BackoffHTTPRetry`.
- `pumps/splunk.go:181-307` — `WriteData` with optional batching and
  content-length cap.
- `pumps/splunk.go:newSplunkClient`, `:FilterTags` — helper paths.

## Evidence
- `pumps/splunk_test.go` (re-annotated `Verifies: SW-REQ-048`).
- Live-Splunk tests need a running HEC endpoint and are excluded from the
  local audit MC/DC scope (known issue).

## Open questions
- `SSLInsecureSkipVerify` exists as an operator override; the corrected
  Phase 0 evidence reflects that TLS verification is operator-configurable
  per pump. Documentation should make the security implication clear.
- The retry layer uses `retry.BackoffHTTPRetry` which has its own
  retry-after-aware behaviour (SW-REQ-030/031); operators rely on its
  defaults.
