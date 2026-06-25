# SW-REQ-017: Pump registry — name lookup and per-pump instance construction

## Intent
The pump registry shall expose a map of available pump prototypes keyed by
name (`pumps.AvailablePumps`) and a lookup function (`GetPumpByName`) that
returns the prototype; the core then calls the prototype's `Pump.New()` to
obtain a fresh, isolated instance for each configured pump entry. This
two-stage pattern (prototype lookup + `.New()` factory) keeps the registry a
compile-time-known set while letting the runtime instantiate as many copies as
the user configures. Derived from SYS-REQ-001 (per-purge-cycle ingestion).

## Motivation
The core needs to (a) validate at startup that a user-specified pump name is
known, (b) produce a fresh instance per configuration entry so two configs of
the same pump (e.g. two MongoDB targets) don't share state, and (c) keep the
list of supported pumps explicit and reviewable. Returning the prototype
directly would couple instance lifetime to package-init lifetime; returning a
new struct each call would forbid pumps from carrying initialization-only state
on the prototype. The `prototype + .New()` split addresses both: registration
is cheap (just a struct literal) and instantiation is explicit.

The failure modes this catches:
- Typos in pump names fail fast at config-load time with a clear error
  (`name + " Not found"`).
- Per-pump state (clients, mutexes, log entries) never leaks between
  configurations because `.New()` always returns a freshly-zeroed struct.

## Code references
- `pumps/init.go:6` — `var AvailablePumps map[string]Pump` (package-level
  registry).
- `pumps/init.go:9-46` — `init()` registers all 30+ pump prototypes by name.
- `pumps/pump.go:17-39` — `Pump` interface, including `New() Pump` factory
  method.
- `pumps/pump.go:48-55` — `GetPumpByName` performs a case-insensitive lookup
  and returns the prototype or an error.
- `main.go:201-208` — `pmpType, _ := pumps.GetPumpByName(...)` followed by
  `thisPmp := pmpType.New()` is the actual usage pattern.

## Evidence
- `pumps/pump_test.go` exercises `GetPumpByName` for known and unknown names.
- `pumps/pump_test.go:37 TestGetPumpByName_SQSSupported` pins `sqs`/`SQS`
  as a supported registry key and verifies `.New()` returns a fresh `SQSPump`.
- Indirectly: every pump-specific `*_test.go` that calls `pump.New()` confirms
  the factory returns a usable, independent instance.

## Open questions
- The registry is mutated only at `init()` time; there is no public
  Register/Deregister API. Phase A could decide whether to allow plugin-style
  registration or document the closed-set choice explicitly.
- `GetPumpByName` lowercases the input but the map is populated with
  lowercase-only keys, so the lowercasing is defensive. The exact
  case-sensitivity contract is not in the requirement text.
- The requirement doesn't capture the prototype-must-not-hold-runtime-state
  invariant that `.New()` exists to enforce — Phase A should add an explicit
  obligation.
