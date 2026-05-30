# SW-REQ-054: HTTP Resurface pump — async worker with channel buffering

## Intent
The `ResurfacePump` shall, on each purge, hand the data batch to an
internal goroutine via a buffered channel (capacity 5); the worker shall
parse each record's base64-encoded raw HTTP request and response,
reconstruct `http.Request` / `http.Response` instances (including request
URL, headers, custom `tyk-API-*` headers, optional chunked-trailer
parsing), and submit them via `logger.SendHttpMessage`. `WriteData` shall
return `ctx.Err()` on context cancellation and shall not block when the
worker queue is full while the pump is enabled. Derived from SYS-REQ-004
via Phase A decomposition of SW-REQ-027.

## Motivation
Resurface is the only HTTP-logging pump with a real async-worker / channel
design and ctx-aware send. It exists because Resurface's API expects HTTP
message replay (request and response), which is expensive to reconstruct
per-record; the worker pattern amortises the cost across the pump's
runtime. Splitting it out of SW-REQ-027 makes the worker shape and the
ctx-aware return contract explicit.

## Code references
- `pumps/resurface.go:ResurfacePump.WriteData` — the public entry; returns
  `ctx.Err()` on cancellation.
- `pumps/resurface.go:ResurfacePump.writeData` — the internal worker
  function.
- `pumps/resurface.go:ResurfacePump.initWorker` — sets up the buffered
  channel.
- `pumps/resurface.go:mapRawData`, `:parseHeaders` — record → http.Request /
  http.Response reconstruction.
- `pumps/resurface.go:Flush`, `:Shutdown` — lifecycle hooks.

## Evidence
- `pumps/resurface_test.go` (re-annotated `Verifies: SW-REQ-054`).
- Live-Resurface tests need a real endpoint and are excluded from the local
  audit MC/DC scope (known issue).

## Open questions
- The worker swallows per-record errors (`continue`); only ctx errors
  surface at the WriteData level. Honest obligation_class is `nominal`
  (could be promoted to `errors_propagated` only for ctx errors).
- Channel capacity of 5 is hard-coded; under sustained load this can
  apply backpressure to the pump loop.
