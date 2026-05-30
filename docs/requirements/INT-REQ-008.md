# INT-REQ-008: Pump meta-config interpretation and rejection contract

## Intent
Each entry in the operator's `pumps:` config block has a `type` and a
`meta` map; the `meta` map is opaque to the core and interpreted
according to the declared `type` by the corresponding pump's `Init`
method. This requirement asserts two things: (1) the core dispatches the
`meta` map to the correct pump type for interpretation, and (2) malformed
pump configuration is rejected loudly — surfaced as an error to the
operator rather than silently ignored. It satisfies SYS-REQ-008.

## Motivation
The polymorphic-meta pattern is what lets the pump's config file describe
~25 different backends without each one bloating the top-level schema.
But it pushes validation to the per-pump `Init`, where two failure modes
have historically caused operator pain: (a) silent acceptance of typos
(operator writes `csv_dir` as `csv_path` and the pump initialises with a
zero-value path); and (b) silent skipping of misconfigured pumps so the
running pump set differs from the configured pump set without a clear
error.

"Loud" rejection means the misconfiguration must reach the operator —
either as an error log with the pump skipped from the running set, or
(for the catastrophic case where no pumps are left) as a process-exit
fatal.

## Code references
- Config struct: `config.go:103-105`
  `Meta map[string]interface{} \`json:"meta"\`` — the per-pump opaque
  payload.
- Dispatch in core: `main.go:201` `pmpType, err :=
  pumps.GetPumpByName(pumpTypeName)`; if the type is unknown the pump is
  skipped with an error log at `main.go:203-205`.
- Per-pump `Init(meta)`: `main.go:215` `initErr := thisPmp.Init(pmp.Meta)`;
  on error the pump is skipped with an error log at `main.go:217`. If
  zero pumps remain, `main.go:227` `log.Fatal("No pumps configured")`
  exits the process.
- Per-pump `Init` typically does `mapstructure.Decode(conf, &c.SomeConf)`
  (e.g. `pumps/sql.go:223`, returning the decode error), then merges
  env-var overrides via `processPumpEnvVars` at `pumps/pump.go:58`.
- Env-var dispatch convention:
  - `pumps/pump.go:14` `PUMPS_ENV_PREFIX = "TYK_PMP_PUMPS"`.
  - `pumps/pump.go:15` `PUMPS_ENV_META_PREFIX = "_META"`.
  - Each pump's meta env prefix is computed at `config.go:363`
    `pmp.Meta["meta_env_prefix"] = PUMPS_ENV_PREFIX + "_" + pmpName +
    PUMPS_ENV_META_PREFIX`.
- Pump-name resolution: `pumps/pump.go:48` `GetPumpByName` looks up
  `pumps.AvailablePumps[strings.ToLower(name)]`, returning an explicit
  "Not found" error.

## Evidence
- `config_test.go` covers `LoadConfig` including pump-name normalisation
  to uppercase (`main.go:277-280`) and env-var overlays.
- `pumps/pump_test.go` covers `GetPumpByName` happy and not-found paths.
- Most per-pump test suites (`pumps/csv_test.go`, `pumps/kafka_test.go`,
  `pumps/elasticsearch_test.go`, ...) include an `Init`-with-bad-meta
  case that asserts the returned error.

## Open questions
- "Loud" is implemented as a logged error and a skipped pump
  (`main.go:217`). For an operator who configures three pumps and
  fat-fingers one, the process *will* keep running with two pumps — and
  the only signal is a single log line at startup. The contract does
  not require a non-zero exit, a metric, or a health-check fail.
- `mapstructure.Decode` is lenient by default: extra/unknown keys in the
  meta map are *not* rejected, so `csv_pathh` is silently dropped. The
  contract says "reject malformed" but the implementation accepts
  typos. Adding `mapstructure.DecoderConfig{ErrorUnused: true}` would
  realise the spec but would be a breaking change for existing operator
  configs.
- `meta_env_prefix` is itself injected into the meta map at
  `config.go:363`, which means pumps see a key they did not declare —
  another reason `ErrorUnused: true` cannot be turned on without
  reworking the env overlay.
