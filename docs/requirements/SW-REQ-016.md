# SW-REQ-016: Common pump base — uniform setters/getters for per-pump configuration

## Intent
The `CommonPumpConfig` struct shall provide a single, uniform mechanism for the
core to configure every pump implementation: filters, write timeout, max-record
size, omit-detailed-recording, ignore-fields, and base64-decoding flags. Because
the core's `filterData` path (`main.go:378`) reaches into every pump through the
`Pump` interface to apply trimming, filtering, omit-detail, and base64 decoding
before each write, the contract behind those calls must be a shared mixin rather
than re-implemented per pump. This requirement is derived from SYS-REQ-004
(per-backend independent delivery): the core must be able to set
backend-specific limits and policy from one place.

## Motivation
Without a shared base, each pump would need to re-declare and wire roughly a
dozen interface methods (`SetFilters`, `GetFilters`, `SetTimeout`, `GetTimeout`,
`SetMaxRecordSize`, `GetMaxRecordSize`, `SetOmitDetailedRecording`,
`SetIgnoreFields`, `SetDecodingRequest/Response`, `SetLogLevel`, `Shutdown`,
`GetEnvPrefix`). Drift between pumps would silently break filter or trim
behaviour for one backend while leaving the others correct — a class of bug
that's extremely hard to detect downstream. Embedding `CommonPumpConfig` makes
the contract trivially uniform; pump-specific overrides (e.g. Influx2's
`Shutdown` flushing the write API) remain explicit.

The base also centralises two cross-cutting concerns shared by the SQL family
(`HandleTableMigration`, `MigrateAllShardedTables`, `OpenGormDB`) and TLS setup
(`NewTLSConfig`), keeping production code DRY without coupling the pump
contract to a particular SQL or TLS library.

## Code references
- `pumps/common.go:18-27` — `CommonPumpConfig` struct (private fields plus
  `OmitDetailedRecording`).
- `pumps/common.go:29-112` — every `SetX`/`GetX` annotated
  `reqproof:implements SW-REQ-016`, satisfying the `Pump` interface in
  `pumps/pump.go:17-39`.
- `pumps/common.go:117-208` — SQL helpers (`HandleTableMigration`,
  `MigrateAllShardedTables`) shared by SW-REQ-019 pumps.
- `pumps/common.go:214-303` — `OpenGormDB`, `NewTLSConfig` shared helpers.
- `main.go:378-432` — `filterData` consumes the getters before each `WriteData`.

## Evidence
- `pumps/common_test.go` exercises filter/getter/setter wiring directly.
- `pumps/pump_test.go` covers `GetPumpByName` and the `.New()` interaction that
  depends on the embedded base.

## Open questions
- The base also exposes SQL-specific helpers (`HandleTableMigration`,
  `OpenGormDB`) that are unrelated to the "uniform setter/getter" intent and
  live there only because `pumps/common.go` is a convenient home. Phase A
  should consider moving these into a `pumps/sqlcommon.go` and giving them
  their own requirement under SW-REQ-019.
- `NewTLSConfig` is a cross-cutting TLS helper not specific to a single
  pump-family — Phase A could split it into its own SW-REQ to make TLS policy
  auditable independently of the common base.
- The `SetLogLevel` setter requires `p.log` to be pre-populated by `Init`; this
  ordering invariant is not captured in the requirement text.
