# STK-REQ-004: Pump health and throughput observability

## Intent
SRE/platform operators need to know when the pump itself — distinct from the
gateway, distinct from any one backend — is unhealthy or falling behind.
This stakeholder requirement captures two operator-facing signals: an HTTP
liveness endpoint suitable for orchestrator and load-balancer probes, and
optional purge-timing/record-count metrics emitted to a metrics sink when
instrumentation is enabled.

## Motivation
Pump outages are silent failure modes. When the pump stops draining the
analytics list, the gateway-side Redis grows; eventually records age out and
are lost. Dashboards downstream simply show "no traffic". Operators need an
out-of-band signal that says "the pump process is alive" (so Kubernetes can
restart it) and a richer signal that says "the pump is alive but its
purge loop is taking longer than the purge interval" (so SREs can alert on
degradation before data is lost).

Metrics are opt-in (gated on `TYK_INSTRUMENTATION=1`) because the StatsD
sink is a hard dependency operators may not run. The health endpoint is
always on, because orchestrator probes are universal.

## Code references
Decomposes into the following SYS reqs via its acceptance criteria:
- AC-001 (HTTP liveness endpoint): `SYS-REQ-012`.
- AC-002 (purge timing / record-count metrics when enabled):
  `SYS-REQ-013`, `SYS-REQ-017` (metrics-sink failure must not propagate to
  records being forwarded).

Implementation pointers:
- Health endpoint: `server/server.go:19` `ServeHealthCheck` (defaults to
  port 8083, path `/health`); handler `server/server.go:49` `Healthcheck`
  responds `{"status":"ok"}` with HTTP 200.
- Wiring: `main.go:501` `go server.ServeHealthCheck(...)`.
- Instrumentation init: `instrumentation_helpers.go:16` `SetupInstrumentation`,
  gated on `os.Getenv("TYK_INSTRUMENTATION") == "1"`.
- Per-purge job: `main.go:264` `instrument.NewJob("PumpRecordsPurge")`,
  emitting `purge_time_all` (`main.go:291`) and per-pump
  `purge_time_<name>` (`main.go:493`); per-record event at `main.go:328`
  `job.Event("record")`.

## Evidence
- `server/server_test.go` exercises the health-check handler.
- `instrumentation_test.go` exercises StatsD sink wiring.
- Per-pump purge timing is asserted indirectly by the `gocraft/health`
  job nesting in `main_test.go`.

## Open questions
- The health endpoint reports only process liveness — there is no readiness
  signal that the pump can actually reach Redis or any configured backend.
  Operators using it as a readiness probe will get false positives during
  startup, when `setupAnalyticsStore` is still failing-fast.
- `SYS-REQ-017` requires metrics-sink failures not to propagate, but the
  StatsD sink path in `instrumentation_statsd_sink.go` swallows errors at
  the sink level; there is no documented invariant that a metrics-sink
  exception cannot escape `job.Timing(...)`. The promise is implicit in the
  `gocraft/health` API contract, which the spec does not pin.
