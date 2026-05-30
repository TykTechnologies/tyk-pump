# SW-REQ-027: HTTP logging pump family — forward records to remote logging services

## Intent
The HTTP logging pump family (Splunk, Graylog, Logz.io, Syslog, Moesif,
Segment, Resurface) shall forward analytics records to remote logging
services over their respective protocols (HTTPS for Splunk/Moesif/Logz.io,
GELF for Graylog, syslog UDP/TCP/TLS for Syslog, segment.io API for
Segment, Resurface API for Resurface), propagating connection and HTTP write
errors. TLS verification policy is operator-configurable per pump. Derived
from SYS-REQ-004 (independent per-backend delivery).

## Motivation
Per Phase 0 verification: the original obligation referenced
`cert_validation_strict` as a hard guarantee, but `splunk.go:63` exposes
`SSLInsecureSkipVerify` as an operator-controllable knob that bypasses
certificate validation. The corrected obligation is `errors_propagated`
(which is genuinely true across the family) plus a clarification that TLS
verification is operator-configurable per pump.

This family is grouped together because they share the same shape — HTTP-ish
push to a remote analytics/logging endpoint — but they actually differ
substantially per pump:

- **Splunk** uses HEC (`/services/collector/event/1.0`) with a token, supports
  batching with content-length capping, has built-in retry via
  `retry.BackoffHTTPRetry`, and exposes the most TLS knobs
  (cert/key/CA/server-name/skip-verify).
- **Graylog** uses GELF UDP (no TLS, no auth — relies on private network).
- **Logz.io** uses a dedicated SDK with token auth and structured logging.
- **Syslog** uses BSD syslog over UDP/TCP/TLS.
- **Moesif** uses the Moesif SDK with application ID auth.
- **Segment** uses the Segment Go SDK with write key auth.
- **Resurface** uses Resurface's NDJSON API.

The error-propagation guarantee holds because the core's
`execPumpWriting` (`main.go:435`) treats a non-nil `WriteData` return as a
backend failure and logs accordingly; per SYS-REQ-004 a single backend
failure does not block writes to others.

## Code references
- `pumps/splunk.go:36-48` — `SplunkClient` (carries `TLSSkipVerify`);
  `SplunkPump` embedding `CommonPumpConfig`.
- `pumps/splunk.go:63` — `SSLInsecureSkipVerify bool` (the corrected-during-Phase-0
  evidence: TLS verification is operator-configurable).
- `pumps/splunk.go:118-179` — `Init` builds an `http.Client` with optional
  TLS skip-verify; uses `retry.BackoffHTTPRetry`.
- `pumps/splunk.go:181-307` — `WriteData` with optional batching and
  content-length cap.
- `pumps/graylog.go:107` — `GraylogPump.WriteData` — GELF UDP, no TLS knobs.
- `pumps/logzio.go:141` — `LogzioPump.WriteData` — uses Logz.io SDK
  (token-only).
- `pumps/syslog.go:145` — `SyslogPump.WriteData` — supports `udp`, `tcp`,
  `tls` transports.
- `pumps/moesif.go:336` — `MoesifPump.WriteData` — uses Moesif SDK
  (application ID).
- `pumps/segment.go:62-74` — `SegmentPump.WriteData` /
  `WriteDataRecord` — uses Segment Go SDK.
- `pumps/resurface.go:271` — `ResurfacePump.WriteData` — NDJSON API.

## Evidence
- `pumps/splunk_test.go`, `pumps/syslog_test.go`, `pumps/logzio_test.go`,
  `pumps/segment_test.go`, `pumps/resurface_test.go`.
- No `graylog_test.go` or `moesif_test.go` (gap).
- Live-endpoint tests are excluded from the local audit MC/DC scope
  (recorded as a known issue).

## Open questions
- TLS verification is operator-configurable per pump
  (`SSLInsecureSkipVerify` exists in `splunk.go:63`); other pumps in the
  family have varying TLS guarantees (graylog uses GELF UDP with no TLS;
  syslog supports `tls` transport but with its own knobs; logzio/moesif/
  segment rely on their SDK defaults). The family-level requirement hides
  this — Phase A should split per-pump.
- Moesif and Graylog have no test files; segment has only a thin one.
- Splunk has its own retry layer (`retry.BackoffHTTPRetry`); other pumps
  rely on the caller for retry. Phase A should make retry policy explicit
  per-pump.
- The "error propagation" guarantee varies in faithfulness:
  - Splunk batches and returns the last batch's error.
  - Graylog UDP returns errors from `net.Write` but UDP "success" is
    meaningless.
  - Segment uses an async SDK — `WriteData` returns before flush completes.
- Generally: family-level reqs hide per-implementation distinctions, which
  Phase A's per-implementation decomposition addresses. Note this as future
  work.
