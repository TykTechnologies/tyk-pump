# SW-REQ-046: InfluxDB v1 pump — line-protocol batch writes

## Intent
The `InfluxPump` (Influx v1) shall, on each purge, open a fresh HTTP client
to the configured Influx v1 address, build an InfluxDB 1.x line-protocol
`BatchPoints` with operator-configured `Tags` and `Fields` selected from each
analytics record (timestamps recorded at `time.Now()`, microsecond
precision), and write the batch via the v1 client. Derived from SYS-REQ-004
via Phase A decomposition of SW-REQ-022.

## Motivation
InfluxDB v1 remains in operator inventories despite v2 being the recommended
target. This pump exists for backward compatibility. Splitting it out of
the influx family makes the v1-specific defects (silent write-error
discard, unbounded reconnect recursion) explicit so they are not masked by
the family-level claim of error propagation.

## Code references
- `pumps/influx.go:15-18` — `InfluxPump` struct.
- `pumps/influx.go:27-46` — `InfluxConf`.
- `pumps/influx.go:85-99` — `connect()` (unbounded recursion on failure with
  a 5-second sleep — a stack-overflow risk under sustained outage).
- `pumps/influx.go:101-173` — `WriteData` builds a `BatchPoints`; the return
  value of `c.Write(bp)` is *discarded* at line 169 — write errors are
  silently swallowed.

## Evidence
- No dedicated `influx_test.go` exists for v1 (coverage gap).
- The previous family req SW-REQ-022 is retained as a `[SUPERSEDED by
  Phase A decomposition: ...]` anchor.

## Open questions
- `WriteData` always returns `nil` because `c.Write(bp)` is discarded
  (line 169). Honest obligation_class is `nominal`, not `errors_propagated`.
- `connect()` recursion has no bound; under sustained Influx outage this
  can blow the stack. Tracked as a follow-up known-issue candidate.
- No `influx_test.go` — the v1 pump is currently untested beyond
  compilation. Should be flagged independently of the req decomposition.
