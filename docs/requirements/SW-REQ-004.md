# SW-REQ-004: Graceful shutdown handler

## Intent
Realises the orderly-stop side of parent **SYS-REQ-001**. `checkShutdown` is polled once per purge cycle by `StartPurgeLoop`; on `ctx.Done()` it iterates every entry in the `Pumps` slice, calls `pmp.Shutdown()`, logs success/failure per pump, signals `wg.Done()` so `main` can unblock, and returns `true` to tell the purge loop to exit. The cancel signal is wired in `main` to `SIGINT` / `SIGTERM` via `signal.Notify` on a buffered channel.

## Motivation
A non-blocking `select { case <-ctx.Done(): ...; default: }` check inside the purge tick keeps the steady-state path branch-free while still guaranteeing that the next tick after termination is the last one. Shutdown is best-effort: an error from `pmp.Shutdown()` is logged but never propagated, so a hung Mongo pump cannot block the rest of the pumps from closing. Trade-off: there is no per-pump shutdown timeout — if a pump's `Shutdown` blocks indefinitely, the whole `wg.Wait()` in `main` blocks, and the process only exits when the supervisor SIGKILLs it.

## Code references
- `main.go:335 checkShutdown` — non-blocking `select` on `ctx.Done()`, loop over `Pumps` calling `Shutdown`, `wg.Done()` and return `true` on cancel.
- `main.go:303` — call site inside `StartPurgeLoop` (one check per tick, after both analytics and uptime data have been processed).
- `main.go:533-542` — main wires the `WaitGroup`, `context.WithCancel`, `signal.Notify(termChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)`, then `cancel()` + `wg.Wait()`.

## Evidence
- `main_test.go:257 TestShutdown` — tagged `// Verifies: SW-REQ-004` and `// SW-REQ-004:error_handling:negative`. Builds a `MockedPump`, spawns a goroutine that polls `checkShutdown`, fires a `SIGINT` into the term chan, asserts `mockedPump.TurnedOff == true`.

## Open questions
- `wg.Done()` is called unconditionally inside the cancel branch even if `Shutdown` returned an error — the contract is "all shutdowns attempted" not "all shutdowns succeeded". A pump that panics in `Shutdown` (rather than returning an error) will crash the process.
- The check runs *after* the per-tick analytics drain, so a cancel that arrives early in a tick still results in one more full purge before exit. Acceptable for the SYS req (records consumed eventually) but worth noting for very long purge intervals.
- The uptime pump (`UptimePump`) is *not* iterated by `checkShutdown` — only the main `Pumps` slice is. An uptime SQL/Mongo connection leak on shutdown is possible. The req text says "shut down all pumps" — strictly speaking the uptime pump is not shut down.
