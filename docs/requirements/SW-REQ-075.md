# SW-REQ-075: Per-backend fan-out independence in writeToPumps

## Intent
`main.writeToPumps` shall write to each configured backend in its own
`execPumpWriting` goroutine and wait for all of them, so that a failure, error,
or timeout contained within one pump's goroutine (logged as a Warning, not
propagated) does not prevent the records being written to the remaining
backends.

## Motivation
This requirement is the software decomposition of `SYS-REQ-004`
(`other_backends_written`): the system-level guarantee that one failing backend
does not block the others. The per-backend fan-out independence lives in
`main.writeToPumps` / `execPumpWriting` (the `pump-core` component), distinct
from the per-pump write-timeout abort modeled by `SW-REQ-072`. Pinning it at the
software level keeps the fan-out contract — one goroutine per pump, each
containing its own error/timeout, logged not propagated — explicit and
regression-tested.

## Code references
- `main.go` `writeToPumps` / `execPumpWriting` — per-backend fan-out (one
  `execPumpWriting` goroutine per pump, per-goroutine error/timeout containment)
  (`// reqproof:implements SW-REQ-075`).
- Tests: `main_test.go` `TestWriteDataWithFilters` (`// Verifies: SW-REQ-075`).

## Evidence
Verified by `TestWriteDataWithFilters` with all three MC/DC witness rows
covered:
- `// MCDC SW-REQ-075: a_backend_failed=F, other_backends_written=F => TRUE`
  (no backend failed — vacuous TRUE)
- `// MCDC SW-REQ-075: a_backend_failed=T, other_backends_written=F => FALSE`
  (the violation row: a backend failed and the others were NOT written)
- `// MCDC SW-REQ-075: a_backend_failed=T, other_backends_written=T => TRUE`

The five-pump fan-out with per-pump filters forces at least one backend to
legitimately reject a record while the others still write, witnessing
`a_backend_failed=T` with `other_backends_written=T`. The requirement is
assurance level B, approved, and trace-clean (`approvals_current`,
`suspect_clean`).

## Related requirements
`SW-REQ-095` covers the narrower TT-5776 data-isolation invariant inside this
fan-out path: per-backend `filterData` transforms must not mutate the shared
decoded dispatch batch seen by sibling pumps.

## Open questions
- If `writeToPumps` ever changes from a per-pump goroutine fan-out (e.g. a
  shared error channel that aborts the whole write on the first failure), this
  requirement and its MC/DC witnesses must be updated together.
