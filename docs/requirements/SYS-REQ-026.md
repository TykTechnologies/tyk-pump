# SYS-REQ-026: FIPS-compliant build available via `make build-fips`

## Intent
A FIPS-compliant binary build of tyk-pump shall be available via `make build-fips`, producing a binary linked against boringcrypto (via `GOEXPERIMENT=boringcrypto`). This satisfies parent **STK-REQ-002** by giving regulated-environment customers an audit-defensible build path without forking the codebase.

## Motivation
Tyk customers in US Federal, healthcare, and financial-services environments are subject to FIPS-140 cryptography validation requirements. Without a FIPS build target, those operators would have to maintain a downstream fork — fragmenting the codebase and slowing security-patch rollout. Capturing the availability of the FIPS build as a SYS-layer constraint surfaces the obligation: any refactor of the build system or crypto imports must keep `make build-fips` working. It also pins the *how* (the boringcrypto GOEXPERIMENT) rather than just the *what*, so reviewers know which Go cryptographic backend is being relied on.

## Formalization
```
build shall always satisfy fips_build_available
```
This is a project-level invariant: at every commit on `master` (and every tagged release), the `build-fips` Makefile target must succeed and produce a binary that links the boringcrypto symbols. There is no temporal qualifier — the constraint must hold on every build, not just on demand. The truth condition is verified by the Makefile target itself plus the `validate-fips` companion target that greps `go tool nm` output for `boring` symbols.

## Code references
- `Makefile:1-2 build-fips` — `GOEXPERIMENT=boringcrypto go build -tags=boringcrypto`.
- `Makefile:10-11 validate-fips` — `go tool nm tyk-pump | grep -i boring`, the self-check that boringcrypto symbols are actually linked.
- `Makefile:7-8 run-fips` — convenience target that runs the FIPS binary, demonstrating it is end-to-end functional, not just compilable.
- Any `crypto/*` imports in the codebase (e.g. TLS code in pumps that talk to backend HTTPS endpoints) implicitly route through boringcrypto when this target is used.

## Evidence
- `make build-fips && make validate-fips` is the build-artifact-level evidence; success means boringcrypto symbols are linked.
- CI workflows under `.github/workflows/` should invoke the FIPS target periodically; the constraint review (`verification.review.comment`) confirmed the target is wired and verified.
- Phase B review (`verification.review.comment`): "FIPS in Makefile."
- Release distribution of FIPS-labelled packages/images is covered separately
  by **SYS-REQ-036**.

## Open questions
- The constraint asserts the build is *available*, not that every release ships
  a FIPS artifact. The release-artifact variant mapping is pinned by
  **SYS-REQ-036**.
- `GOEXPERIMENT=boringcrypto` is itself an experimental Go feature; if upstream Go promotes or removes the experiment, the Makefile target needs updating. The req does not bind itself to the *current* Go FIPS plumbing — only to "a FIPS build is available."
- No automated test asserts FIPS-mode behaviour against the network-using pumps (HTTPS backends); the constraint only covers buildability.
