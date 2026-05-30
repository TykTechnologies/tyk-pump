# INT-REQ-004: Pump interface contract

## Intent
Every backend pump (Mongo, SQL, Elasticsearch, Kafka, Splunk, Prometheus,
...) must implement a single Go interface so the pump core can drive any
backend uniformly: load configuration, set per-backend behaviours
(filters, timeout, omit/trim/decoding flags), write batches with cancellable
context, and shut down gracefully. Errors from `WriteData` must propagate
to the core so per-backend timeouts and warnings can be surfaced. This
requirement satisfies SYS-REQ-004.

## Motivation
The interface is what makes the per-backend isolation property
(STK-REQ-002) implementable: the core only needs to know that every pump
honours `WriteData(ctx, []interface{}) error`, so it can wrap each call
with `context.WithTimeout` and a goroutine, and the per-backend error
signal is uniform. Without the interface, the core would have to type-switch
on every supported backend — and operators couldn't add a new backend
without touching the core.

The interface also pins the configuration surface: every pump exposes
setters for the operator-visible knobs (filters, ignore-fields,
decode-request/response), so `initialisePumps` in `main.go` can apply
operator config without knowing the backend-specific shape.

## Code references
The complete interface declaration:
- `pumps/pump.go:17-39` `type Pump interface { ... }` with 18 methods:
  - Identity & lifecycle: `GetName()`, `New()`, `Init(interface{}) error`,
    `Shutdown() error`.
  - Write path: `WriteData(context.Context, []interface{}) error`.
  - Per-backend knobs: `SetFilters`/`GetFilters`, `SetTimeout`/`GetTimeout`,
    `SetOmitDetailedRecording`/`GetOmitDetailedRecording`,
    `SetMaxRecordSize`/`GetMaxRecordSize`,
    `SetIgnoreFields`/`GetIgnoreFields`,
    `SetDecodingResponse`/`GetDecodedResponse`,
    `SetDecodingRequest`/`GetDecodedRequest`.
  - Logging: `SetLogLevel(logrus.Level)`.
  - Env-var prefix: `GetEnvPrefix()`.
- `pumps/pump.go:14-15` env-var prefix constants:
  `PUMPS_ENV_PREFIX = "TYK_PMP_PUMPS"`, `PUMPS_ENV_META_PREFIX = "_META"`.
- Implementations are driven from `main.go:192-225` `initialisePumps`,
  which calls every setter and then `Init(pmp.Meta)`.
- `WriteData` error propagation: `main.go:470` `ch <- pmp.WriteData(ctx,
  filteredKeys)` and the surfacing branch at `main.go:474-479`.
- Shared default implementation of most getters/setters:
  `pumps/common.go` `CommonPumpConfig`.

## Evidence
- `pumps/pump_test.go` exercises `GetPumpByName` against the
  `AvailablePumps` registry.
- Every concrete pump has a `*_test.go` covering `Init` + `WriteData`,
  including `pumps/mongo_test.go`, `pumps/sql_test.go`,
  `pumps/elasticsearch_test.go`, `pumps/kafka_test.go`, etc.
- The interface is satisfied by ~25 concrete pumps registered in
  `pumps/init.go` `AvailablePumps`.

## Open questions
- The interface has no `Ping`/`Healthcheck` method. The core cannot tell
  whether a pump's downstream is reachable until it tries to write; this
  feeds back into STK-REQ-004's readiness gap.
- Some pumps (notably the SQL pump's `SetDecodingRequest`/`SetDecodingResponse`
  at `pumps/sql.go:201` and `:208`) silently no-op with a warning instead
  of returning an error. The interface contract doesn't have a "feature
  unsupported" signal, so operator config can be ignored without a hard
  failure.
