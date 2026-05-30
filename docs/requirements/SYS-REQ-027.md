# SYS-REQ-027: tyk-pump runs as a single-process daemon

## Intent
tyk-pump shall run as a single-process daemon; clustered or sharded multi-process deployments are out of scope. This constraint satisfies parent **STK-REQ-004** by pinning the operational shape the codebase is verified against and that all isolation/coordination assumptions in lower-layer reqs implicitly rest on.

## Motivation
The codebase makes single-process assumptions throughout: the global `Pumps` slice in `main.go`, the singleton `AnalyticsStore` / `UptimeStorage` handles, the absence of any inter-pump coordination primitive, and the way `StartPurgeLoop` runs as a single goroutine driving fan-out via in-process goroutines rather than via a distributed work queue. Capturing this as a SYS-layer constraint matters because operators sometimes attempt to "scale" tyk-pump by running multiple replicas against the same Redis — which would silently duplicate work or, with `GetAndDeleteSet`'s LPOP semantics, race for the same records. Surfacing the constraint explicitly tells operators "don't do that" and tells reviewers that any future change introducing shared mutable state across instances would require a redesign rather than a patch.

## Formalization
```
runtime shall always satisfy single_process_only
```
This is a project-level invariant: the supported deployment topology is exactly one tyk-pump process per gateway analytics stream. There is no temporal qualifier — the invariant holds across all execution. Truth condition: the in-process structures (`Pumps` slice, `AnalyticsStore`, `UptimeStorage`) are the source of truth for "what is being purged" and cannot be coordinated across processes by anything the codebase provides.

## Code references
- `main.go:29-31` — package-level globals `AnalyticsStore`, `UptimeStorage`, `UptimePump` declared as singletons.
- `main.go:192 initialisePumps` — populates the singleton `Pumps` slice from configuration; there is no notion of distributed registry.
- `main.go:260 StartPurgeLoop` — a single goroutine driving the purge tick; not designed for leader election or sharded ranges.
- `main.go:361 writeToPumps` — fan-out via in-process `go execPumpWriting(...)`, not via a work queue.
- `pumps/init.go:6 AvailablePumps map[string]Pump` — process-local registry of pump constructors.

## Evidence
- The runtime model itself is the evidence: every `_test.go` harness in `main_test.go` instantiates a single-process pump under test, and there is no integration test exercising a clustered topology because it is explicitly out of scope.
- Phase B constraint review (`verification.review.comment`): "single-process in main.go runtime."
- Companion docs: SYS-REQ-022 (fan-out to every backend) is itself in-process; clustering would invalidate that.

## Open questions
- Operator-facing follow-up: a startup log line warning "another tyk-pump appears to be active against this Redis" would be a friendly guard, but is not currently emitted. The constraint relies on operator discipline.
- Active-passive failover via HA orchestration (Kubernetes Deployment with `replicas: 1`) is implicitly supported because only one replica is alive at a time; the constraint does not need to be relaxed for that pattern, but it could be clearer that it permits it.
- Scaling guidance for high-throughput deployments is by-design absent at the SYS layer; the relief valve is `PurgeChunk` plus `analytics_config.enable_multiple_analytics_keys` (gateway-side), not horizontal pump scaling.
