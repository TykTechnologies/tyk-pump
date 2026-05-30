# SW-REQ-033: Logger init from TYK_LOGLEVEL plus fixed text formatter

## Intent
Realises parent **SYS-REQ-008** in the logger package. `logger.init()` runs at package import time, reads `TYK_LOGLEVEL` from the environment, maps it through `level(...)` to a `logrus.Level` (`error`, `warn`, `debug`, anything-else → `info`), and installs a fixed `*logrus.TextFormatter` (`TimestampFormat = "Jan 02 15:04:05"`, `FullTimestamp = true`, `DisableColors = true`). The singleton is exposed via `GetLogger()` and consumed by every other package via `var log = logger.GetLogger()`. JSON-configured overrides for `log_level` / `log_format` are applied later by `main.Init` (not by this package).

## Trade-off / motivation note
Initialising at *package init* time (not at `main` startup) means any package-level `var log = logger.GetLogger(); log.Info(...)` initialiser sees a valid logger before `main` runs — important for `init()` chains and for tests, which never call `main.Init`. The trade-off is that the only configuration channel available at this stage is the process environment: a JSON config-file value cannot influence the logger until `main` has read the file and explicitly applied it. The formatter is *fully* hard-coded (no env knob for the timestamp format or for colour); switching to JSON output requires a JSON config override in `main.Init`, which replaces `log.Formatter` wholesale.

## Code references
- `logger/init.go:13 init` — runs at import; sets `log.Level = level(os.Getenv("TYK_LOGLEVEL"))` and `log.Formatter = formatter()`.
- `logger/init.go:19 level` — lower-cases the input and matches `"error"`/`"warn"`/`"debug"`; everything else (including empty) falls through to `InfoLevel`.
- `logger/init.go:33 formatter` — returns the `*logrus.TextFormatter` with the hard-coded settings.
- `logger/init.go:42 GetLogger` — returns the package-level singleton.
- `main.go:79-81` — JSON-driven format override: `if SystemConfig.LogFormat == "json" { log.Formatter = &logrus.JSONFormatter{} }`.
- `main.go:97-113` — JSON-driven level override: only applied when `TYK_LOGLEVEL` is *unset* (env wins over file). Switch over `info`/`error`/`warn`/`debug`, with `log.Fatalf` on unknown values.
- `main.go:116-118` — `--debug` CLI flag forces `DebugLevel`, overriding both env and file.

## Evidence
- `logger/level_test.go:11 TestLevel_AllBranches` — tagged `// SW-REQ-033:nominal:negative`; table-driven coverage of each `level()` branch including the default `info` fallthrough.
- `logger/level_test.go:31 TestFormatter_FixedShape` — tagged `// Verifies: SW-REQ-033`; asserts the formatter's timestamp format, `FullTimestamp`, and `DisableColors` are exactly the documented values.
- `logger/level_test.go:45 TestGetLogger_ReturnsSingleton` — asserts `GetLogger()` returns the same `*logrus.Logger` instance across calls.
- `logger/init_test.go:14 TestFormatterWithForcedPrefixFileOutput`, `:56 Test_GetLogger` — surrounding coverage.

## Open questions
- The JSON-config-driven log-level and log-format overrides happen in `main.go:79-81` and `main.go:97-113`, *not* in `logger/init.go`. The req description correctly notes this split, but the satisfaction story is bipartite: any verification scoped to the `logger` package alone misses the JSON overrides. Tests in the `main` package (e.g. `main_test.go:TestDeprecationWarnings`) do not directly exercise the level-override switch.
- `TYK_LOGLEVEL` takes precedence over `log_level` in JSON config (the JSON path is guarded by `if os.Getenv("TYK_LOGLEVEL") == ""`). This is the opposite convention from most `TYK_PMP_*` env vars, which use `envconfig.Process` to override JSON. Worth documenting because it surprises operators who expect uniform precedence.
- The `--debug` CLI flag overrides both env and JSON (and is applied after both). There is no way to override `--debug` back down to a lower level once set.
- `formatter()` is unexported but returns a pointer to a fresh struct each call — calling it twice produces two distinct formatter instances, which matters only for a future change that swaps formatters under a lock.
