# SW-REQ-026: stdout and dummy pumps — emit-to-stdout and discard for debugging

## Intent
The stdout pump shall emit each analytics record to process stdout (as
either a logrus-formatted text line or a JSON object, controlled by
`Format`). The dummy pump shall accept records and discard them, logging
only a count. Together they serve debugging (stdout) and test-harness
(dummy) workflows where no real backend is available or desired. Derived
from SYS-REQ-004 (independent per-backend delivery).

## Motivation
Both pumps exist because the rest of the pump family is heavy: every other
backend requires either an external service or filesystem-level state.
During development, smoke-testing, and integration tests, operators need a
sink that always succeeds and emits something they can inspect (`stdout`)
or that proves the gateway-pump pipeline is alive without producing output
that needs cleanup (`dummy`).

The stdout pump supports JSON output with operator-controlled root field
name (`LogFieldName`, default `tyk-analytics-record`). JSON mode runs a
payload transform (`transformHTTPPayload`) that splits headers from body
using the standard HTTP separator `\r\n\r\n`, compacts JSON bodies, and
collapses whitespace — making the captured raw_request/raw_response
readable as single-line log entries. The `UseLegacyPayloadFormat` knob
skips this transform for back-compat with downstream log parsers that
expect the raw escaped string.

The dummy pump's `Init` and `WriteData` are five-line no-ops. It exists
purely as a registry placeholder so configs referencing `"type": "dummy"`
resolve at startup.

## Code references
### stdout
- `pumps/stdout.go:21-24` — `StdOutPump` struct.
- `pumps/stdout.go:27-38` — `StdOutConf` (format, log_field_name,
  use_legacy_payload_format).
- `pumps/stdout.go:84-118` — `WriteData`: JSON path calls
  `logrus.JSONFormatter` and writes via `fmt.Print(string(data))`; text
  path uses `s.log.WithField(...).Info()`.
- `pumps/stdout.go:124-145` — `transformHTTPPayload` splits HTTP headers
  from body at `\r\n\r\n` and JSON-compacts the body when valid.
- `pumps/stdout.go:150-166` — `removeWhitespaces` collapses
  `\r`/`\t`/`\n`.

### dummy
- `pumps/dummy.go:7-9` — `DummyPump` struct (embeds `CommonPumpConfig`,
  no fields).
- `pumps/dummy.go:15-22` — `New`/`GetName`.
- `pumps/dummy.go:25-37` — `Init`/`WriteData`: log only, no I/O.

## Evidence
- `pumps/stdout_test.go` covers `transformHTTPPayload`, JSON output, and
  field-name override.
- No dedicated `dummy_test.go` — the pump's lack of behaviour makes a
  unit test largely vacuous. Indirectly exercised by the registry tests
  in `pumps/pump_test.go` (`AvailablePumps["dummy"]`).

## Open questions
- The text-mode path uses `s.log.WithField(...).Info()`, which goes
  through logrus's configured output (typically stderr), not `fmt.Print`.
  The requirement says "emit to stdout" but only the JSON path is
  strictly stdout. Phase A should clarify.
- `transformHTTPPayload` assumes the payload is HTTP-shaped; for non-HTTP
  raw_request strings (e.g. MCP JSON-RPC) the transform is a near-no-op
  but the requirement doesn't bound the input format.
- The dummy pump has no test file at all (gap).
- The `decoded.RawRequest = transformHTTPPayload(...)` mutation inside
  `WriteData` mutates the caller's record. The core's `filterData` runs
  before `WriteData`, but Phase A could capture the no-shared-state
  obligation explicitly.
