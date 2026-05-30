# SYS-REQ-012: HTTP health-check endpoint reports liveness

## Intent
The pump exposes an HTTP endpoint that returns process liveness suitable for Kubernetes / load-balancer probes. This satisfies parent **STK-REQ-004** (observability) by giving operators the standard mechanism to determine whether the pump process is running and able to accept work, decoupled from any backend's health.

## Motivation
Without a dedicated health endpoint, operators have to infer pump liveness from log scraping or from backend write success — both indirect. A dedicated, no-state HTTP 200 response makes the pump first-class in any orchestration stack. Capturing this at the SYS layer also fixes the contract for the response shape (`{"status": "ok"}`) so probes can rely on it.

## Formalization
```
when health_probe_received observability shall immediately satisfy liveness_reported
```
The input `health_probe_received` becomes true on receipt of a `GET` to the configured endpoint; the output `liveness_reported` becomes true once a 200 with the JSON liveness body has been written. Variables: `specs/system/variables/observability.vars.yaml`.

## Code references
- `server/server.go:19 ServeHealthCheck` — mounts `r.HandleFunc("/"+healthEndpoint, Healthcheck).Methods("GET")`; defaults `healthEndpoint = "health"`, `healthPort = 8083`.
- `server/server.go:49 Healthcheck` — writes `{"status": "ok"}` with `Content-Type: application/json` and `http.StatusOK`.
- `main.go:501 go server.ServeHealthCheck(SystemConfig.HealthCheckEndpointName, SystemConfig.HealthCheckEndpointPort, SystemConfig.HTTPProfile)` — started from `main()` before purge loop.
- `server/server.go:32-35` — optional `/debug/pprof/...` profiling endpoints when `HTTPProfile` is true.

## Evidence
- `server/server_test.go:14 TestHealthcheck_ReportsLiveness` — directly exercises the handler.
- Satisfying SW child: **SW-REQ-032** (health-check server contract).

## Open questions
- "Liveness" in the implementation is purely "process is up and the HTTP server is accepting" — it does not test that the purge loop is making progress, nor that backends are reachable. The SYS req uses the word "liveness" without distinguishing it from readiness; if operators expect readiness semantics this is a contract gap.
- The endpoint and port are configurable but the response *body* is hard-coded; the req does not specify the payload format.
