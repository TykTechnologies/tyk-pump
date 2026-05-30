# SW-REQ-002: Config loader (file then TYK_PMP_* env overrides)

## Intent
Implements the configuration-loading half of parent **SYS-REQ-008**. `LoadConfig` reads the JSON config file from `-c/--conf` (default `pump.conf`) into a `TykPumpConfiguration` struct, then runs `envconfig.Process("TYK_PMP", configStruct)` so that any `TYK_PMP_*` environment variable overrides the corresponding struct field. Pump-specific overrides come from a second pass (`LoadPumpsByEnv`) that scans `TYK_PMP_PUMPS_<NAME>_*` env vars and merges them onto the per-pump `PumpConfig`.

## Motivation
The file-then-env order is a deliberate twelve-factor lean: operators ship a baseline `pump.conf` for shape/defaults and inject deployment-specific secrets (Redis password, Mongo URL, StatsD host) via env, without needing a per-environment file. Capturing this at the SW layer pins down two non-obvious behaviours: pump names are folded to upper-case before merging (so `csv` in JSON and `TYK_PMP_PUMPS_CSV_*` end up on the same key), and `TYK_PMP_OMITCONFIGFILE=true` skips the JSON read entirely so a pure-env deployment doesn't need any file on disk. Trade-off: there is no schema validation pass — typos in JSON keys are silently ignored by `encoding/json`, and env-driven misconfigurations only surface when the affected pump fails to init.

## Code references
- `config.go:262 LoadConfig` — `ioutil.ReadFile` + `json.Unmarshal` (conditional on `shouldOmitConfigFile`), uppercase pump-name normalisation at lines 275-280, then `envconfig.Process(ENV_PREVIX, configStruct)` at line 282.
- `config.go:294 shouldOmitConfigFile` — reads `TYK_PMP_OMITCONFIGFILE`.
- `config.go:300 LoadPumpsByEnv` — scans `os.Environ()` for `TYK_PMP_PUMPS_*` prefixes, dedups pump names, then `envconfig.Process(PUMPS_ENV_PREFIX+"_"+pmpName, &pmp)` per pump (line 352).
- `config.go:17 ENV_PREVIX = "TYK_PMP"` — note the typo in the constant name (preserved for backwards compat).
- `main.go:75 LoadConfig(conf, &SystemConfig)` — the only caller in production.

## Evidence
- `config_test.go:40 TestLoadExampleConf` — `// Verifies: SW-REQ-002`; loads `pump.example.conf` and asserts the struct.
- `config_test.go:56 TestConfigEnv` — exercises the env-override pass with a synthetic `TYK_PMP_*` set.
- `config_test.go:109 TestIgnoreConfig` — `TYK_PMP_OMITCONFIGFILE=true` path.
- `config_test.go:12 TestToUpperPumps` — pump-name case folding.
- `config_test.go:143 TestTykPumpConfiguration_LoadPumpsByEnv`, `config_test.go:257 TestLoadPumpsByEnv` — the pump-by-env merge.
- `main_test.go:408 TestDeprecationWarnings` — `// Verifies: SW-REQ-002`; deprecated-field warnings.

## Open questions
- File-read errors are logged at `Error` (not fatal) — a typo in `-c path` produces a warning, an empty struct, and a downstream "no pumps configured" fatal far from the root cause. The req description says "read the JSON config file" but does not require the file to exist or be valid.
- `envconfig` ignores `json:` tags entirely; the env-var name is derived from the Go field name. This means a JSON-friendly rename of a struct field is a silent breaking change for env-driven deployments.
