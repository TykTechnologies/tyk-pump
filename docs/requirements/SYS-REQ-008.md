# SYS-REQ-008: Configuration loaded from JSON file at startup

## Intent
At startup the pump loads its configuration from a JSON file *before* any environment overrides are applied. This satisfies parent **STK-REQ-003** (operator-configurable behaviour) by establishing JSON as the canonical, version-controllable configuration source — env vars then layer on top for deployment-specific tweaks.

## Motivation
Operators expect a predictable precedence so that `pump.conf` checked into infrastructure repos is the source of truth, and CI/CD env-var overrides are visible exceptions rather than silent shadows. Capturing the "JSON first" obligation prevents future refactors from inverting the order (which would silently break GitOps workflows). Phase 0.6 split the original SYS-REQ-008: the "every field overridable via TYK_PMP_*" obligation moved to companion **SYS-REQ-020**.

## Formalization
```
when json_config_file_present configuration shall always satisfy config_loaded_from_json
```
The input `json_config_file_present` becomes true when the `--conf` path (default `pump.conf`) resolves and `TYK_PMP_OMITCONFIGFILE` is not set; the output `config_loaded_from_json` holds once `json.Unmarshal` has populated `TykPumpConfiguration`. Variables: `specs/system/variables/configuration.vars.yaml`.

## Code references
- `config.go:262 LoadConfig` — entrypoint, called from `main.Init()` at `main.go:75`.
- `config.go:264-272` — `ioutil.ReadFile(*filePath)` + `json.Unmarshal(configuration, &configStruct)`, gated by `shouldOmitConfigFile()`.
- `config.go:294 shouldOmitConfigFile` — checks `TYK_PMP_OMITCONFIGFILE` env to allow env-only setups.
- `config.go:282 envconfig.Process(ENV_PREVIX, configStruct)` — env override runs *after* JSON, encoding the precedence.

## Evidence
- `config_test.go:40 TestLoadExampleConf` — loads the shipped example config.
- `config_test.go:109 TestIgnoreConfig` — verifies `OMITCONFIGFILE` skip path.
- `config_test.go:56 TestConfigEnv` — verifies env overrides apply after JSON.
- Satisfying SW children: **SW-REQ-002** (configuration loader JSON+env), **SW-REQ-033** (logger init env precedence).

## Open questions
- Phase 0.6 split: env-override obligation now lives in **SYS-REQ-020**.
- If `ReadFile` fails, the code logs an error but continues with the zero-value config (`config.go:266`); the SYS req does not specify whether a missing JSON is fatal. Today it is not, which means env-only deployments are implicitly supported even without `OMITCONFIGFILE=true`.
- The req says "loaded from a JSON file" but `LoadConfig` will also tolerate non-JSON (`marshalErr` is logged, not fatal). Strictness of the load is not formalized.
