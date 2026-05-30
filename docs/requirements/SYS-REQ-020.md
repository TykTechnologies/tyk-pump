# SYS-REQ-020: Every config field overridable via TYK_PMP_* env

## Intent
Every configuration field shall be overridable by an environment variable of the form `TYK_PMP_<FIELD>`, and the effective configuration shall reflect the env override when present. This satisfies parent **STK-REQ-003** by giving operators a uniform, deployable mechanism to tweak any setting without rewriting the JSON config (essential for containerised / Helm deployments).

## Motivation
Without an env-override path, operators would have to template the JSON config per-environment — a fragile pattern compared to `TYK_PMP_*` variables set on the container. Capturing this as its own SYS req (split from SYS-REQ-008 in Phase 0.6) makes the contract symmetric across every field: there are no fields that can only be set via JSON. The reverse — env-only fields — is also avoided, so `pump.conf` remains an authoritative reference.

## Formalization
```
when env_override_present configuration shall always satisfy config_reflects_env
```
The input `env_override_present` is true when at least one `TYK_PMP_<FIELD>` env var is set; the output `config_reflects_env` becomes true after `envconfig.Process(ENV_PREVIX, configStruct)` writes the env value into `TykPumpConfiguration`. Variables: `specs/system/variables/configuration.vars.yaml`.

## Code references
- `config.go:17 const ENV_PREVIX = "TYK_PMP"` — the canonical prefix.
- `config.go:282 envconfig.Process(ENV_PREVIX, configStruct)` — applies env overrides after `json.Unmarshal`.
- `config.go:352 envconfig.Process(PUMPS_ENV_PREFIX+"_"+pmpName, &pmp)` — same mechanism applied per pump in `LoadPumpsByEnv`.
- `config.go:171-175` — comment block documenting the `TYK_PMP_PUMPS_<NAME>_` per-pump prefix.
- `storage/temporal_storage.go:78-85` — separate `envconfig.Process(envRedisPrefix, ...)` plus `envconfig.Process(envTemporalStoragePrefix, ...)` for storage config.

## Evidence
- `config_test.go:56 TestConfigEnv` — sets `TYK_PMP_*` env vars and verifies the resulting configuration.
- `config_test.go:143 TestTykPumpConfiguration_LoadPumpsByEnv` and `:257 TestLoadPumpsByEnv` — exercise per-pump env wiring.
- Satisfying SW child: **SW-REQ-002** (JSON+env loader), **SW-REQ-033** (logger env precedence).

## Open questions
- Phase 0.6 origin: spun out of SYS-REQ-008 ("JSON first") so each precedence side is its own atomic req.
- "Every configuration field" is a strong claim — it holds because `envconfig` is reflection-based and walks the whole `TykPumpConfiguration` struct, but new fields without `mapstructure`/`envconfig` tags could break it without test failure. Worth adding a CI check.
- Env override for nested complex types (`map[string]PumpConfig`) goes through the bespoke `LoadPumpsByEnv` flow (`config.go:300`) rather than `envconfig`; the req does not distinguish these two paths.
