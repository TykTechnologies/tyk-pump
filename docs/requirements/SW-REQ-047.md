# SW-REQ-047: InfluxDB v2 pump — async WriteAPI batch writes

## Intent
The `Influx2Pump` (Influx v2) shall, on each purge, append one Point per
analytics record to the v2 client's asynchronous `WriteAPI` for the
configured organisation and bucket, using operator-configured `Tags` and
`Fields` and microsecond precision. When `Flush` is true the WriteAPI shall
be drained synchronously before the function returns. On `Init`, the pump
shall check server readiness (`client.Ready`), resolve the organisation by
name via `OrganizationsAPI`, and optionally create the bucket (with
operator-configured retention rules) when `CreateMissingBucket` is enabled.
Derived from SYS-REQ-004 via Phase A decomposition of SW-REQ-022.

## Motivation
InfluxDB v2 is the recommended target for operators using Influx. The v2
client model is fundamentally async — `WriteAPI.WritePoint` returns no
error, instead surfacing failures through a background error channel that
this pump does not drain. The honest obligation is `nominal`: only `Init`
errors propagate; per-record write failures are invisible to `WriteData`.
Splitting v2 out of SW-REQ-022 makes the async-write semantics explicit so
operators evaluating Influx vs Mongo/SQL backends understand the failure-
visibility trade-off.

## Code references
- `pumps/influx2.go:17-21` — `Influx2Pump` struct (persistent client).
- `pumps/influx2.go:47-72` — `Influx2Conf` (bucket, org, token, flush,
  create-missing-bucket).
- `pumps/influx2.go:91-148` — `Init` performs `Ready`, org lookup, and
  optional bucket creation.
- `pumps/influx2.go:151-155` — `Shutdown` flushes the write API.
- `pumps/influx2.go:165-191` — `createBucket` with retention rules.
- `pumps/influx2.go:194-253` — `WriteData` builds points via
  `influxdb2.NewPoint`; writes via `WriteAPI`; optional `Flush`.

## Evidence
- `pumps/influx2_test.go` (re-annotated `Verifies: SW-REQ-047`).
- Live-Influx tests need a running v2 server and are excluded from the
  local audit MC/DC scope (known issue).

## Open questions
- `WriteData` returns errors only from marshalling and the optional
  `Flush`; per-record `WriteAPI` errors are not surfaced. Honest
  obligation_class is `nominal`, not `errors_propagated`.
- TLS configuration is implicit — operators pass an HTTPS URL and rely on
  the underlying client's defaults.
