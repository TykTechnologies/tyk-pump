# SW-REQ-049: HTTP Graylog pump — GELF UDP message forwarding

## Intent
The `GraylogPump` shall, on each purge, encode each analytics record as a
GELF JSON message containing only the operator-configured `Tags` field set,
and send it to the configured Graylog host:port via the `gelf` client
(typically UDP). Derived from SYS-REQ-004 via Phase A decomposition of
SW-REQ-027.

## Motivation
Graylog is the cheapest HTTP-logging integration: GELF over UDP has no TLS,
no acknowledgements, and no retry. This pump exists for operators who want
to fire-and-forget analytics to a Graylog instance on a trusted private
network. Splitting it out of SW-REQ-027 lets the no-acks / no-TLS
properties be explicit — the family-level error_propagation claim was
misleading because UDP `Write` "success" is operationally meaningless.

## Code references
- `pumps/graylog.go:GraylogPump.Init` — config parsing and `connect()`.
- `pumps/graylog.go:GraylogPump.WriteData` — GELF encode + UDP send.
- `pumps/graylog.go:GraylogPump.connect` — opens the UDP socket.

## Evidence
- No dedicated `graylog_test.go` exists today (coverage gap).
- The previous family req SW-REQ-027 is retained as a `[SUPERSEDED by
  Phase A decomposition: ...]` anchor.

## Open questions
- The pump calls `p.log.Fatal(err)` on base64-decode and JSON-marshal
  errors, which terminates the *entire* Pump process. Honest
  obligation_class is `nominal` (errors propagate by killing the
  process, which is a defect rather than a contract).
- If the client is nil, `WriteData` recurses into `connect()` then
  re-enters `WriteData(ctx, data)` unbounded — a latent stack-overflow.
  Tracked alongside other recursive-reconnect issues.
- No test file — gap.
