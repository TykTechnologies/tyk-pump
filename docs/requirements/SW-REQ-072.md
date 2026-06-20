# SW-REQ-072: Per-pump write timeout and context propagation (design point under SYS-REQ-005)

## Intent
Each pump goroutine spawned by `execPumpWriting` (`main.go`) shall receive a
`context.Context` bounded by the per-pump configured timeout (`PumpConfig.Timeout`).
Every pump's `WriteData` implementation shall honour ctx cancellation: when
`ctx.Done()` fires, the pump shall stop the in-flight write and return `ctx.Err()`
so the core can detect over-time writes and surface them per SYS-REQ-005.

## Motivation
SYS-REQ-005 (parent) and STK-REQ-002 require per-backend isolation: a slow or
hung backend must not block other backends or the purge cycle. The
implementation contract today is split between the caller (`main.execPumpWriting`
wraps each pump in a `context.WithTimeout(ctx, pmp.GetTimeout()*time.Second)`)
and the callee (each pump's `WriteData(ctx, []interface{}) error`). SW-REQ-072
names the joint design point so the affixed Known Issues have a SW-level home:
- `mongo-pump-ignores-caller-context` — mongo family ignores the caller ctx and
  uses a backend default.
- `elasticsearch-unbounded-reconnect-recursion` — connect-failure recursion is
  not bounded by ctx.
- `pump-no-timeout-can-block-purge-cycle` — `PumpConfig.Timeout = 0` disables
  enforcement entirely.

## Code references
- Caller: `main.go` `execPumpWriting` — wraps the per-pump call in
  `context.WithTimeout` and a goroutine, then awaits the result via a channel.
- Pump contract: `pumps/pump.go` `Pump.WriteData(context.Context, []interface{}) error`.
- Affected callees (see KIs above):
  - `pumps/mongo.go`, `pumps/mongo_selective.go`, `pumps/mongo_aggregate.go` (caller ctx ignored).
  - `pumps/elasticsearch.go` (unbounded reconnect recursion).
  - All pumps when `PumpConfig.Timeout == 0` (no enforcement).

## Evidence
This SW req is a contract-only design point. The `cancellation_observed` and
`error_handling` obligations are SUPPRESSED on this requirement (rationale:
the actual cancellation behaviour is broken today and is captured by the three
named KIs). Release disposition is `ship_with_known_issue` until the KIs are
remediated.

## Open questions
- The remediation plan is to refactor each affected pump to honour caller ctx
  (mongo: pass `ctx` to `Database.Insert`/`Aggregate`; elasticsearch: bound
  reconnect via ctx; treat `Timeout == 0` as a hard policy error or apply a
  sane default).
- Once the KIs are resolved, the deferrals on SW-REQ-072 should be removed
  and tests annotated `// SW-REQ-072:cancellation_observed:negative` and
  `// SW-REQ-072:error_handling:negative` against the new behaviour.
