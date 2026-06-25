# SW-REQ-050: HTTP Syslog pump — single-line syslog record forwarding

## Intent
The `SyslogPump` shall, on each purge, write one legacy `map[...]` syslog
message per analytics record. LF characters in `raw_request` and
`raw_response` are escaped as backslash-n sequences so one analytics record
remains one syslog entry, while preserving the backward-compatible output
shape. Delivery uses the operator-configured `Transport` (udp/tcp/tls),
`NetworkAddr`, `LogLevel` (syslog severity 0-7), and `Tag`. When per-pump
`omit_detailed_recording` is enabled, syslog output must reflect the core
privacy transform and must not contain the original raw request/response
payload bytes. Derived from SYS-REQ-004 via Phase A decomposition of
SW-REQ-027, and linked to SYS-REQ-015 for the backend-output privacy boundary.

## Motivation
Syslog remains a popular ingest target for SIEM pipelines that don't speak
HTTP. This pump exists to give operators a low-friction way to forward
analytics to syslog collectors. Splitting it out of SW-REQ-027 lets the
RFC 3164 vs RFC 5424 framing (Go stdlib emits 3164) and the per-record
ctx-cancellation check (unique among HTTP-logging pumps) be explicit.

## Code references
- `pumps/syslog.go:SyslogPump.Init`, `:initWriter`, `:initConfigs` — config
  parsing and writer setup.
- `pumps/syslog.go:SyslogPump.WriteData` — per-record legacy `map[...]`
  write with LF escaping in `raw_request` and `raw_response`.
- `main.go:filterData` plus `SyslogPump`'s shared `CommonPumpConfig` surface
  provide the per-pump `omit_detailed_recording` privacy transform before
  `WriteData` formats backend output.
- `pumps/syslog.go:185` — `fmt.Fprintf(s.writer, ...)`'s return is
  intentionally discarded (`_, _ = ...`); per-record write errors are not
  surfaced.
- ctx-aware loop: `select { case <-ctx.Done(): return nil; default: ... }` —
  the only HTTP-logging pump that honours caller `ctx`.

## Evidence
- `pumps/syslog_test.go` (re-annotated `Verifies: SW-REQ-050`).
- `TestSyslogPump_WriteData_WithMultilineHTTP` covers DEFECT-2 / TT-15532 by
  asserting a multiline raw request/response emits exactly one syslog line,
  contains escaped `\n`, and still uses the legacy `map[...]` format.
- `TestSyslogPump_WriteData_ManyRecords` covers the one-output-per-record
  cardinality contract.
- `main_test.go:TestSyslogPump_OmitDetailedRecordingRedactsForwardedPayloads`
  covers DEFECT-35 / commit `34e1a2c` syslog privacy inheritance by asserting
  a real syslog UDP message omits original raw request/response bytes after
  `filterData` redaction.
- Live-syslog tests need a running syslog server and are excluded from the
  local audit MC/DC scope (known issue).

## Formalization note
The FRETish slice for this requirement is intentionally transport-focused
(`transport_tcp -> tcp_writer_used`). The multiline fragmentation guarantee is
covered through `encoding_safety` and `output_cardinality_bounded` obligations
and their annotated tests rather than by expanding the transport predicate.

## Open questions
- `WriteData` always returns `nil` (per-record write errors silently
  discarded). This remains tracked by the write-error KnownIssue family.
- `initWriter` calls `log.Fatal` on dial failure — same `log.Fatal`
  anti-pattern as graylog/moesif/influx.
- Uses Go stdlib `log/syslog` which emits RFC 3164 (BSD) framing, not
  RFC 5424. Operators expecting RFC 5424 will see parse errors at the
  receiving end.
