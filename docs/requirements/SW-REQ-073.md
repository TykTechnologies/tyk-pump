# SW-REQ-073: Uptime-aggregate path (decomposed out of SW-REQ-015)

## Intent
The uptime-aggregate path (`analytics/uptime_data.go` `AggregateUptimeData` +
`uptime_aggregate_sql` / `uptime_aggregate_mongo`) shall aggregate uptime
records into per-URL counters (hits, errors, latency) and write the aggregated
rows to the uptime-aggregate backend.

## Motivation
SW-REQ-015 (parent) covers the uptime *data model*. The aggregate path is a
distinct piece of behaviour: it consumes uptime records, groups them by URL,
maintains hit / error / latency counters, and emits aggregated rows. Three KIs
were previously attributed to SW-REQ-015 because no aggregate-side SW req
existed; this requirement is their proper home:
- `uptime-aggregate-erasstr-itoa-always-nonempty` — error-string `strconv.Itoa`
  can produce stable but misleading bucket keys.
- `uptime-aggregate-nil-errormap-on-tcp-then-http` — nil error-map race when a
  URL flips between TCP and HTTP error families inside a single batch.
- `uptime-onconflict-request-time-is-zero-unreachable` — `OnConflict` clause
  whose request-time-is-zero branch is unreachable in production.

## Code references
- `analytics/uptime_data.go` `AggregateUptimeData` — entry point.
- `analytics/uptime_aggregate_sql.go` and `analytics/uptime_aggregate_mongo.go`
  — backend-specific writers.
- Tests: `analytics/uptime_data_test.go`.

## Evidence
This SW req is a contract surface for the uptime-aggregate path. The
`atomicity`, `monotonicity`, and `nominal` obligations are tracked on this
requirement as open KI-backed debt because their failure modes are captured by
the three named KIs; release disposition is `ship_with_known_issue` until those
KIs are remediated.

## Open questions
- Once each KI is resolved, the matching deferral should be removed and new
  test triples `// SW-REQ-073:atomicity:negative`, `// SW-REQ-073:monotonicity:positive`,
  `// SW-REQ-073:nominal:positive` should be annotated against the fixed code paths.
