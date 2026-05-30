# STK-REQ-005: API uptime data forwarding

## Intent
Per-API availability dashboards (a first-class Tyk Dashboard feature)
depend on uptime/host-checker data being drained from the gateway-side
temporal store and forwarded to a backend the dashboard can read. This
stakeholder requirement asserts that the pump forwards that uptime stream
by default, and that operators who do not need it can disable the uptime
purger via configuration.

## Motivation
Without uptime forwarding, the Dashboard's "API health" widgets go blank —
even if request analytics continue to flow. The two streams are logically
separate (uptime is produced by the gateway host-checker, not by request
processing) and they live under different Redis keys with different key
prefixes. Conflating them in the pump would force operators to either keep
both or lose both. Splitting them lets operators who do not use uptime
dashboards (or who use an external uptime tool) opt out and save the
write load on their backend.

The "unless the operator disables" carve-out is a real production case:
many self-hosted deployments use Prometheus blackbox probes or similar
external checkers and disable the in-gateway uptime story entirely.

## Code references
Decomposes into the following SYS reqs via its acceptance criteria:
- AC-001 (forward uptime to uptime backend): `SYS-REQ-014` (consume uptime
  from temporal store), `SYS-REQ-021` (forward consumed uptime to configured
  uptime backend).
- AC-002 (disable via config): `SYS-REQ-014` (the same SYS req covers the
  opt-out gate).

Implementation pointers:
- Uptime store key: `storage/store.go:19`
  `UptimeAnalytics_KEYNAME = "tyk-uptime-analytics"`. Note the storage key
  prefix is swapped to `host-checker:` for the uptime client
  (`main.go:142`).
- Uptime drain: `main.go:294`
  `UptimeStorage.GetAndDeleteSet(storage.UptimeAnalytics_KEYNAME, ...)`,
  guarded by `if !SystemConfig.DontPurgeUptimeData` at `main.go:293`.
- Uptime pump init: `main.go:239` `initialiseUptimePump`, selecting between
  `pumps.SQLPump{IsUptime:true}` and `pumps.MongoPump{IsUptime:true}` based
  on `SystemConfig.UptimePumpConfig.UptimeType`.
- Forward call: `main.go:300` `UptimePump.WriteUptimeData(UptimeValues)`.

## Evidence
- `main_test.go` covers `initialiseUptimePump` selection logic.
- `pumps/sql_test.go` and `pumps/mongo_test.go` cover SQL/Mongo uptime
  write paths via the `IsUptime` flag.

## Open questions
- The `UptimePump` interface (`pumps/pump.go:41`) has `WriteUptimeData(data
  []interface{})` with **no error return** — meaning failures in the uptime
  backend are silently dropped at the call site (`main.go:300`). This
  diverges from the regular `Pump.WriteData` contract which returns an
  error, and is not covered by any SYS req.
- Only SQL and Mongo are wired as uptime backends (`main.go:244`). The
  stakeholder text says "configured uptime backend" without enumerating;
  if operators expect e.g. Prometheus/Elastic as an uptime sink, the
  current implementation will silently fall through to Mongo (default
  case at `main.go:249`).
