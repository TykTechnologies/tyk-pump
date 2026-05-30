# SYS-REQ-022: Consumed records dispatched to every backend pump

## Intent
When the pump has consumed an analytics record from the temporal store, it dispatches the record to every configured backend pump on that same purge cycle. This satisfies parent **STK-REQ-001** at the fan-out side: the value of running multiple pumps is that each backend sees every record, not a stochastic subset.

## Motivation
Operators configure multiple pumps explicitly to fan-out the same data (raw to SQL, aggregated to Mongo, alerting to Splunk). A bug that silently dropped records from one branch of the fan-out would be invisible to the gateway and only surface as missing rows in downstream reports days later. Capturing fan-out as its own SYS req (split from SYS-REQ-001 in Phase 0.6) makes the "every pump sees every record" promise atomic and code-verifiable.

## Formalization
```
when record_available_for_dispatch delivery shall always satisfy record_dispatched_to_all_backends
```
The input `record_available_for_dispatch` becomes true when a record has been decoded in `PreprocessAnalyticsValues`; the output `record_dispatched_to_all_backends` holds when one goroutine per configured pump has been launched against the same `keys` slice. Variables: `specs/system/variables/delivery.vars.yaml`.

## Code references
- `main.go:361 writeToPumps` — `wg.Add(len(Pumps)); for _, pmp := range Pumps { go execPumpWriting(&wg, pmp, &keys, ...) }` — one goroutine per backend, all sharing the same `keys`.
- `main.go:435 execPumpWriting` — per-pump goroutine; receives the same record set.
- `main.go:331 writeToPumps(keys, job, startTime, int(secInterval))` — single call site, from `PreprocessAnalyticsValues`.
- `main.go:192 initialisePumps` — populates the `Pumps` slice that `writeToPumps` iterates.

## Evidence
- `main_test.go:168 TestWriteDataWithFilters` — exercises multi-pump fan-out with shared input records.
- Satisfying SW child: **SW-REQ-017** (pump registry / `pumps.AvailablePumps`).

## Open questions
- Phase 0.6 origin: spun out of SYS-REQ-001 so consumption and fan-out are atomic.
- "Dispatched" is true even when a backend's `WriteData` fails or times out — `execPumpWriting` still gets *called* per pump. The req says "dispatched", not "successfully written"; SYS-REQ-004 covers the isolation guarantee on top.
- The `keys` slice is shared by pointer; each pump's `filterData` makes a *new* slice via `filteredKeys := make([]interface{}, len(keys)); copy(filteredKeys, keys)` (`main.go:389-390`) so per-pump mutations do not leak — this is correct but unstated at SYS level.
