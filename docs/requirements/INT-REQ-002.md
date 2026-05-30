# INT-REQ-002: Gateway/pump uptime-key publish-consume contract

## Intent
This is the cross-component wire contract for the uptime/host-checker
stream between the gateway and the pump: the gateway publishes uptime
report data under a single, well-known key, and the pump pops it off and
forwards it to the configured uptime backend. It satisfies SYS-REQ-014 and
parallels INT-REQ-001 for the request-analytics stream.

## Motivation
The uptime stream is logically and storage-wise separate from the
request-analytics stream — it has its own producer (the gateway's
host-checker), its own backend (SQL or Mongo uptime tables), and its own
storage prefix (`host-checker:` rather than `analytics-`). The contract
needs to be called out separately so that schema changes on one side do
not silently break the other.

Unlike the request-analytics path, the uptime path is single-key and
unsharded: there is no `_0..9` suffix, no serializer-suffix discriminator.
That asymmetry is exactly what makes pinning the contract in a separate
INT req worthwhile — readers should not assume by analogy with INT-REQ-001.

## Code references
- `storage/store.go:19` `UptimeAnalytics_KEYNAME = "tyk-uptime-analytics"` —
  the wire-format key constant.
- `main.go:142` — the uptime storage handler is constructed by cloning the
  analytics storage config and swapping `KeyPrefix = "host-checker:"`. The
  effective Redis key becomes `host-checker:tyk-uptime-analytics`.
- `main.go:294` — `UptimeStorage.GetAndDeleteSet(storage.UptimeAnalytics_KEYNAME,
  chunkSize, expire)` — the same `GetAndDeleteSet` contract as INT-REQ-005,
  but on the uptime store.
- `main.go:300` — `UptimePump.WriteUptimeData(UptimeValues)` — forward call.
- `pumps/pump.go:41-45` — `UptimePump` interface declaration: `GetName()`,
  `Init(interface{}) error`, `WriteUptimeData(data []interface{})`.

## Evidence
- `storage/temporal_storage_test.go` covers `GetAndDeleteSet` against any
  key, including uptime.
- `pumps/sql_test.go` and `pumps/mongo_test.go` cover the SQL/Mongo
  uptime-write paths (`IsUptime: true` instances).

## Open questions
- `UptimePump.WriteUptimeData` returns no error. A backend write failure
  is silently dropped at `main.go:300`; the contract is "fire and forget",
  unlike the regular `Pump.WriteData` contract in INT-REQ-004 which
  propagates errors. This should arguably be a separate INT req or at
  least an explicit asymmetry note.
- The contract does not pin the payload schema on the uptime key (the
  gateway-side `UptimeReportData` type). If the gateway changes that
  struct, the pump's Mongo/SQL marshalling can break silently.
