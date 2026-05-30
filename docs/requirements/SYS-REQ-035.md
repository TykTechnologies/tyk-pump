# SYS-REQ-035: Configuration loading is robust to malformed input

## Intent
When the pump encounters a malformed or missing configuration input — config file does not exist, JSON parse error, missing required field, or unknown environment-variable typo — it shall continue without crashing, falling back to defaults for missing values and surfacing an informational log for unknown overrides. This requirement closes the `malformed_input` slot on parent **STK-REQ-003** ("operators can control which analytics are collected … configured via file and environment variables") by making the configuration-loading contract honest about its tolerance for operator mistakes.

## Motivation
The STK-REQ-003 obligation checklist declares three classes of behaviour: `nominal` (the happy path of well-formed config), `boundary` (size limits, count limits) and `malformed_input` (config that is broken in some way). Before this requirement existed, the `malformed_input` slot was unsatisfied: no SYS requirement had `obligation_class: malformed_input` even though the code in `config.go` (and the mapstructure-driven env-override path) does in fact tolerate several malformed-input cases. The audit caught the gap and forced an honest decomposition: rather than re-classifying a sibling SYS req (which would have misrepresented its purpose), Phase 0.7 added SYS-REQ-035 to capture the malformed-input contract explicitly. Two known issues — `mapstructure-decode-silently-drops-unknown-keys` and `env-prefix-const-typo` — sit underneath this requirement and document the parts of the malformed-input contract that are *not* yet observable (silent drops + the historic typo in `ENV_PREVIX`).

## Formalization
```
configuration shall always satisfy config_loading_robust_to_malformed_input
```
Always-pattern: across all startup paths, `LoadConfig` returns without panicking and the resulting `TykPumpConfiguration` is usable, even when one or more of (config-file-missing, JSON parse fails, env-var has unknown key, env-var has wrong type for target field) holds. The truth condition is observable at the boundary: `LoadConfig` returns and downstream code finds the configuration populated with the operator-provided values for the *valid* fields plus defaults for the rest.

## Code references
- `config.go:262 LoadConfig` — the loader entrypoint; resilience to malformed input is layered through these calls.
- `config.go:264-272` — `ioutil.ReadFile` + `json.Unmarshal`; missing file is handled by the `os.IsNotExist` branch, returning the input config unchanged.
- `config.go:282 envconfig.Process(ENV_PREVIX, configStruct)` — the env-var override path, which silently drops unknown keys (tracked by KI `mapstructure-decode-silently-drops-unknown-keys`).
- `config.go:294 shouldOmitConfigFile` — `TYK_PMP_OMITCONFIGFILE` opt-out for env-only deployments.
- The historic typo `ENV_PREVIX` constant (still present for backward compatibility) is tracked by KI `env-prefix-const-typo` — operators using either `TYK_PMP_*` or the typo-form would see different behaviour, but neither panics.

## Evidence
- `config_test.go:109 TestIgnoreConfig` — subtest "Config file does not exist" verifies the missing-file path returns cleanly with the initial config unchanged. Annotated `// SYS-REQ-035:malformed_input:negative` (also covered indirectly by `SW-REQ-002:malformed_input:negative`).
- Two open known issues capture the remaining malformed-input gaps: `mapstructure-decode-silently-drops-unknown-keys` (silent drop on unknown env keys) and `env-prefix-const-typo` (the typo-form coexisting with the canonical form).
- Implementing child: **SW-REQ-002** (the configuration loader) which now satisfies SYS-REQ-035.

## Open questions
- The "informational log for unknown overrides" half of the contract is currently aspirational — `envconfig` silently drops, so the log does not exist. A future enhancement would wrap `envconfig.Process` to enumerate unknown keys and surface them; until then KI `mapstructure-decode-silently-drops-unknown-keys` is the placeholder.
- Missing-required-field behaviour is not crisply defined: the pump runs with whatever the zero value happens to be. A formal "required field" annotation in the config struct would tighten this; not in scope for Phase 0.7.
