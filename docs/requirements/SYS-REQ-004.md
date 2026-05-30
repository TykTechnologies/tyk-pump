# SYS-REQ-004: Per-backend isolation on write

## Intent
When the pump writes a batch of records, each configured backend pump is written to independently — a failure, panic, or timeout on one backend must not prevent or roll back writes to the others. This satisfies the parent **STK-REQ-002** ("delivery to downstream sinks must be resilient") by encoding the basic isolation property that operators rely on when they configure heterogeneous backends (e.g. Splunk + Prometheus + SQL).

## Motivation
Most tyk-pump deployments multiplex 2-5 backends with very different reliability characteristics (a remote logging SaaS may be slow; local SQL is fast). Without per-backend isolation, the slowest or flakiest sink would degrade every other sink, defeating the whole point of running multiple pumps. Capturing this at the SYS layer prevents future maintainers from converting the fan-out into a sequential pipeline for "simplicity" and silently regressing operator behaviour.

## Formalization
```
when a_backend_failed delivery shall always satisfy other_backends_written
```
The input `a_backend_failed` becomes true whenever any single pump returns an error or its context is cancelled in `execPumpWriting`; the output `other_backends_written` holds when the remaining pumps still complete their writes for the same batch. Variables: `specs/system/variables/delivery.vars.yaml`.

## Code references
- `main.go:361 writeToPumps` — `for _, pmp := range Pumps { go execPumpWriting(...) }`: one goroutine per backend, joined via `sync.WaitGroup`.
- `main.go:435 execPumpWriting` — isolates failure: errors are logged (`log.Warning("Error Writing to: ...")`) but never propagated to siblings.
- `main.go:473-491` — `select` on `ch <- err` vs `ctx.Done()` ensures a single pump's timeout/cancel never blocks the WaitGroup.
- `pumps/common.go:18 CommonPumpConfig` — each pump owns its own filter/timeout/state, so there is no shared mutable surface that one failing pump could corrupt.

## Evidence
- `main_test.go:168 TestWriteDataWithFilters` exercises multi-pump fan-out with per-pump filter state.
- SW-layer reqs that satisfy this: **SW-REQ-016** (common-pump base accessors), and every backend SW req (**SW-REQ-018..029**) which inherits the contract through the `Pump` interface.

## Open questions
- The FRETish uses "always satisfy" but the actual guarantee is "best-effort within `purge_delay`": the parent loop blocks on `wg.Wait()` (`main.go:369`), so a slow backend without `Timeout` configured can still delay the *next* purge tick (warned about at `main.go:436-446`). The SYS req does not state this latency interaction.
- Panics inside a backend's `WriteData` are not currently caught with `recover()` in `execPumpWriting`; a panicking pump would crash the whole process, violating this req. Worth flagging for security/robustness review.
