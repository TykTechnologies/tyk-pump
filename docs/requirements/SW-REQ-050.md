# SW-REQ-050: HTTP Syslog pump — JSON-over-syslog record forwarding

## Intent
The `SyslogPump` shall, on each purge, write one JSON message per analytics
record (with `\n` escaping in `raw_request` and `raw_response`) to a syslog
destination using the operator-configured `Transport` (udp/tcp/tls),
`NetworkAddr`, `LogLevel` (syslog severity 0-7), and `Tag`. Derived from
SYS-REQ-004 via Phase A decomposition of SW-REQ-027.

## Motivation
Syslog remains a popular ingest target for SIEM pipelines that don't speak
HTTP. This pump exists to give operators a low-friction way to forward
analytics to syslog collectors. Splitting it out of SW-REQ-027 lets the
RFC 3164 vs RFC 5424 framing (Go stdlib emits 3164) and the per-record
ctx-cancellation check (unique among HTTP-logging pumps) be explicit.

## Code references
- `pumps/syslog.go:SyslogPump.Init`, `:initWriter`, `:initConfigs` — config
  parsing and writer setup.
- `pumps/syslog.go:SyslogPump.WriteData` — per-record JSON encode + write.
- `pumps/syslog.go:185` — `fmt.Fprintf(s.writer, ...)`'s return is
  intentionally discarded (`_, _ = ...`); per-record write errors are not
  surfaced.
- ctx-aware loop: `select { case <-ctx.Done(): return nil; default: ... }` —
  the only HTTP-logging pump that honours caller `ctx`.

## Evidence
- `pumps/syslog_test.go` (re-annotated `Verifies: SW-REQ-050`).
- Live-syslog tests need a running syslog server and are excluded from the
  local audit MC/DC scope (known issue).

## Open questions
- `WriteData` always returns `nil` (per-record write errors silently
  discarded). Honest obligation_class is `nominal`.
- `initWriter` calls `log.Fatal` on dial failure — same `log.Fatal`
  anti-pattern as graylog/moesif/influx.
- Uses Go stdlib `log/syslog` which emits RFC 3164 (BSD) framing, not
  RFC 5424. Operators expecting RFC 5424 will see parse errors at the
  receiving end.
