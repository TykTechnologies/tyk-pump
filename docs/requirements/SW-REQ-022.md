# SW-REQ-022: InfluxDB pump family — v1 line-protocol and v2 API writers

## Intent
The InfluxDB pump family shall write analytics points to InfluxDB v1
(`influx` pump, line protocol via `influxdata/influxdb/client/v2`) and
InfluxDB v2 (`influx2` pump, v2 HTTP API via `influxdata/influxdb-client-go/v2`).
Each pump exposes operator-configurable tag and field selection over a fixed
mapping derived from the analytics record. Derived from SYS-REQ-004
(independent per-backend delivery).

## Motivation
InfluxDB v1 and v2 are different products with different wire protocols,
auth models, organisation concepts (v2 introduces orgs/buckets, v1 has
databases), and client libraries. Lumping them under one logical "InfluxDB"
choice for operators is convenient but the implementations share almost
nothing — they're two separate pumps registered under two distinct names
(`influx` and `influx2` in `pumps/init.go:19-20`).

Both pumps share a configurable tag/field projection: the operator lists
which analytics-record fields become Influx tags (indexed, low-cardinality)
versus fields (measurement values). The fixed mapping (method, path,
response_code, …) means the operator can't surface arbitrary headers or
extensions, but it keeps the tag space bounded — important for Influx
performance, which degrades with high-cardinality tags.

Failure modes addressed:
- v1 transient outage: `connect()` recurses with a 5-second sleep on failure
  (no bound).
- v2 startup verification: `Init` calls `client.Ready()` and the
  `OrganizationsAPI` lookup, failing fast if the server isn't reachable or
  org is wrong.
- v2 missing bucket: `CreateMissingBucket` opt-in calls the buckets API to
  create on demand.

## Code references
### InfluxDB v1
- `pumps/influx.go:15-18` — `InfluxPump` struct.
- `pumps/influx.go:27-46` — `InfluxConf` (database, addr, fields, tags).
- `pumps/influx.go:85-99` — `connect()` (unbounded recursion on failure).
- `pumps/influx.go:101-173` — `WriteData` builds a `BatchPoints`, projects
  configured fields/tags, writes via line-protocol client.

### InfluxDB v2
- `pumps/influx2.go:17-21` — `Influx2Pump` struct (holds a persistent client).
- `pumps/influx2.go:47-72` — `Influx2Conf` (bucket, org, token, flush,
  create-missing-bucket).
- `pumps/influx2.go:91-148` — `Init` performs `Ready`, org lookup, optional
  bucket creation.
- `pumps/influx2.go:151-155` — `Shutdown` flushes the write API.
- `pumps/influx2.go:165-191` — `createBucket` with retention rules.
- `pumps/influx2.go:194-253` — `WriteData` builds points via
  `influxdb2.NewPoint`, writes via `WriteAPI`, optional `Flush`.

## Evidence
- `pumps/influx2_test.go` exercises v2 pump configuration and write logic.
- No dedicated `influx_test.go` for v1 (gap).
- End-to-end tests require a running Influx instance and are excluded from
  the local audit MC/DC scope (recorded as a known issue).

## Open questions
- v1 and v2 are different protocols, different clients, different auth, and
  different lifecycle (v1 reconnects per write, v2 holds a persistent
  client). Lumping them in one req hides the per-version differences. Phase A
  should split into SW-REQ-022a (v1) and SW-REQ-022b (v2).
- v1 has no test file at all — `influx2_test.go` exists but not
  `influx_test.go`. Worth flagging as a coverage gap independent of req
  decomposition.
- v1 `connect()` recurses without a maximum retry count; on a permanent
  outage this could blow the stack rather than fail Init.
- v1 `WriteData` calls `c.Write(bp)` ignoring the returned error — silent
  data loss.
- v2 `WriteData` does not return errors from `WriteAPI.WritePoint` (async by
  design); only `Shutdown` surfaces drain failures.
- Neither pump exposes TLS knobs separately — operators pass an HTTPS URL
  and rely on the underlying client's defaults.
