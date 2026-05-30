# SW-REQ-051: HTTP Logz.io pump — disk-buffered SDK enqueue

## Intent
The `LogzioPump` shall, on each purge, marshal each analytics record into a
Logz.io JSON document and enqueue it via the `logzio-go` SDK sender, which
buffers on disk in `QueueDir` and drains every `DrainDuration` to the
configured URL (default `https://listener.logz.io:8071`), bounded by
`DiskThreshold` (1-100%) when `CheckDiskSpace` is true. Derived from
SYS-REQ-004 via Phase A decomposition of SW-REQ-027.

## Motivation
Logz.io has the strongest local buffering semantics of the HTTP-logging
family — the SDK persists events to disk and drains asynchronously, which
makes the pump robust against transient Logz.io outages but also makes
per-record write errors invisible to `WriteData`. Splitting it out of
SW-REQ-027 lets the disk-buffering policy and the SDK's fire-and-enqueue
return contract be explicit.

## Code references
- `pumps/logzio.go:LogzioPump.Init` — config parsing and `NewLogzioClient`
  with `QueueDir`, `DrainDuration`, `DiskThreshold`, `CheckDiskSpace`.
- `pumps/logzio.go:LogzioPump.WriteData` — per-record marshalling +
  `sender.Send`.
- `pumps/logzio.go:NewLogzioClient` — SDK constructor wrapper.

## Evidence
- `pumps/logzio_test.go` (re-annotated `Verifies: SW-REQ-051`).
- Live-Logz.io tests need a real account and are excluded from the local
  audit MC/DC scope (known issue).

## Open questions
- `sender.Send` returns no error at the per-record call site (the SDK is
  fire-and-enqueue) — `WriteData` cannot return per-record errors. Only
  marshalling errors propagate. Honest obligation_class is `nominal`.
- Disk-buffer behaviour means operators can lose buffered events if the
  process is killed mid-drain — worth documenting for ops runbooks.
