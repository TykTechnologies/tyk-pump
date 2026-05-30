# SYS-REQ-024: Build requires Go toolchain 1.25 or later

## Intent
The tyk-pump Go module shall be built with Go toolchain version 1.25 or later. This is a project-level constraint surfaced as a system requirement so the trust boundary between the codebase and its build environment is explicit. It satisfies parent **STK-REQ-001** by capturing the toolchain version the ingestion path is verified against.

## Motivation
Capturing the toolchain floor as a SYS-layer constraint matters because every other requirement in this spec assumes Go 1.25 language and runtime semantics — generics, error wrapping, structured concurrency primitives, and the GC behaviour the goroutine fan-out in `writeToPumps` is sized against. Building with an older toolchain would silently invalidate that verification: the binary would either fail to compile or behave under different runtime characteristics than the ones the test suite exercises. Promoting the constraint to a first-class req lets operators see the toolchain dependency without reading `go.mod`, and lets reviewers gate the next minor bump explicitly rather than as a tooling drive-by.

## Formalization
```
build shall always satisfy go_125_or_later
```
This is a project-level invariant: across every build of tyk-pump, the toolchain producing the artifact must be Go 1.25 or newer. There is no temporal trigger — the invariant must hold on every CI run and every developer build. The truth condition is verified by the `go` directive at the top of `go.mod` together with the toolchain check the Go command performs at compile time (a Go 1.24 toolchain will refuse to build a module declaring `go 1.25.0`).

## Code references
- `go.mod:3 go 1.25.0` — the canonical declaration of the toolchain floor; the Go command enforces this directive at build time.
- `Makefile:1-2 build-fips` — uses the system `go` binary, inheriting the toolchain version pin from `go.mod`.
- `.github/workflows/*.yml` — CI workflows install a matching Go version (the `setup-go` step is where the pin is enforced for hosted builds).

## Evidence
- `go.mod:3` itself is the build artifact that satisfies this constraint; the Go toolchain rejects mismatched builds.
- `make build-fips` followed by `go tool nm tyk-pump | grep -i boring` (the `validate-fips` target) demonstrates that the constraint is exercised end-to-end on the FIPS path as well.
- Phase B constraint review captured in `verification.review.comment`: "Verified against go.mod / storage code / Makefile / runtime model."

## Open questions
- The constraint is one-sided: there is no upper bound. A future Go 1.26 release that changed iterator semantics or `range func` behaviour would build cleanly even if it broke the requirement-level guarantees — a CI matrix run pinned to 1.25.x would catch that drift, but is not currently mandated by this req.
- Operator-facing follow-up: the Tyk distro Dockerfiles and Helm charts should be cross-checked to ensure the runtime image's libc/glibc is compatible with binaries produced by a 1.25 toolchain (especially relevant on Alpine for musl-vs-glibc surprises).
