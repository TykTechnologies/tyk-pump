# SW-REQ-032: Health-check HTTP server with optional pprof

## Intent
Realises parent **SYS-REQ-012**. `server.ServeHealthCheck` resolves endpoint and port defaults, builds a `gorilla/mux` router with one `GET /<healthEndpoint>` route that calls `Healthcheck` — which writes `200 OK` with `Content-Type: application/json` and body `{"status": "ok"}` — and optionally registers `/debug/pprof/profile` and `/debug/pprof/{_:.*}` from `net/http/pprof` when `enableProfiling` is true. Defaults are `healthEndpoint = "health"` and `healthPort = 8083`; the server runs blocking `http.ListenAndServe` on `:<port>`, so the liveness endpoint is reachable by external orchestrators and load balancers instead of being restricted to loopback. It fatals if the port cannot be bound.

## Motivation
A trivial liveness endpoint is the conventional minimum for container orchestrators (Kubernetes `livenessProbe`, ECS health checks) — the response body content is unimportant, the 200 status code is the signal. Binding on all interfaces is part of that contract because probes commonly originate outside the process host or container network namespace; localhost-only listeners pass local smoke tests while failing real monitoring. Keeping the endpoint name and port configurable lets multi-pump deployments avoid port clashes; baking the defaults in means a brand-new operator can launch with no config and still get a working probe. pprof is gated behind a separate config flag (`HTTPProfile`) because exposing profiling endpoints on a process-wide port is a privacy/safety hazard in production. Trade-off: the server only ever serves liveness, never readiness — it returns 200 even if the analytics store is unreachable and the pump is dropping records on every tick.

## Code references
- `server/server.go:18 resolveHealthCheckParams` — endpoint and port defaults.
- `server/server.go:34 buildHealthCheckRouter` — health route plus optional pprof route registration.
- `server/server.go:48 ServeHealthCheck` — starts `http.ListenAndServe` with wildcard-host `:<port>` and calls `log.Fatal` on listener error.
- `server/server.go:64 Healthcheck` — writes the `application/json` response with body `{"status": "ok"}`.
- `main.go:507` — production caller: `go server.ServeHealthCheck(SystemConfig.HealthCheckEndpointName, SystemConfig.HealthCheckEndpointPort, SystemConfig.HTTPProfile)`.

## Evidence
- `server/server_test.go:14 TestHealthcheck_ReportsLiveness` — tagged `// Verifies: SW-REQ-032` and `// SW-REQ-032:nominal:example`; spins up the router and asserts `200 OK` plus the JSON body.
- `server/serve_params_test.go:TestResolveHealthCheckParams_AllBranches` — covers default and configured endpoint/port resolution.
- `server/serve_params_test.go:TestBuildHealthCheckRouter_RegistersExpectedRoutes` and `TestBuildHealthCheckRouter_RegistersPprofWhenEnabled` — cover health route and pprof gating.
- `server/serve_params_test.go:TestServeHealthCheck_BindsExternalInterface` — source-level regression witness for commit 402dab8; it fails if production `ListenAndServe` is restricted to `localhost` or `127.0.0.1`.

## Open questions
- The req text says "respond with a liveness payload" — the payload is `{"status": "ok"}` and is not configurable. A consumer expecting a richer payload (e.g. version, uptime, pump counts) would need a code change. Worth pinning at the SW layer that the body is intentionally minimal.
- A failure in the analytics store, pump shutdown, or backoff retry storm does not affect the liveness response. The "Healthcheck" name is therefore slightly misleading — it reports the HTTP server's own liveness only.
