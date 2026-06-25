# SW-REQ-024: Prometheus pump — scrape endpoint for analytics metrics

## Intent
The Prometheus pump shall expose analytics as Prometheus metrics over an HTTP
scrape endpoint (default `/metrics` on the configured listen address). The
pump shall register a baseline set of counter and histogram metrics
(`tyk_http_status`, `tyk_http_status_per_path`, `tyk_http_status_per_key`,
`tyk_http_status_per_oauth_client`, `tyk_latency`) and accept operator-defined
custom metrics with configurable labels and bucket sets. Derived from
SYS-REQ-004 (independent per-backend delivery).

## Motivation
Prometheus inverts the usual pump model: instead of pushing each batch to a
remote, the pump runs a local HTTP server (`promhttp.Handler()`) and waits
for Prometheus to scrape. `WriteData` therefore updates in-memory counter
maps and histogram observations rather than performing I/O. This is the
correct shape for Prometheus's pull model, but it means the pump's behaviour
diverges from every other pump in the family: write errors don't really
exist (only metric-registration errors do), and "flushing" means exposing
accumulated counter values via `Expose()`.

The `AggregateObservations` knob trades histogram precision for CPU: when
enabled, the pump averages observed `request_time` values per
label-combination before pushing a single observation. This is documented
inline as experimental but is a useful escape hatch for high-cardinality
deployments.

Path labels are governed by the tracking-path policy split into
**SW-REQ-078** and **SW-REQ-079**: by default raw paths collapse to
`unknown`, while either global `track_all_paths` or per-record `TrackPath`
preserves the raw path for route-level metrics.

Custom metrics let operators define their own counters/histograms with
arbitrary labels from a known whitelist (mapping at
`prometheus.go:442-467`). API-key obfuscation (`ObfuscateAPIKeys` +
`ObfuscateAPIKeysLength`) prevents accidental exposure of raw API keys in
scraped metrics. The MCP-only flag (`MCPOnly`) restricts a metric to MCP
records, enabling MCP-specific dashboards without polluting analytics
metrics.

Failure modes addressed:
- Misconfigured `listen_address`: `Init` returns an error rather than
  silently failing.
- Disabled base metrics: split into **SW-REQ-090**, where `DisabledMetrics`
  skips built-in base metric registration/update/exposition without suppressing
  custom metrics.
- Histogram label schema: split into **SW-REQ-091**, where histogram metrics
  normalize the synthetic `type` label before Prometheus registration and
  observation.
- Custom metric identity: split into **SW-REQ-094**, where valid
  operator-defined metrics initialize as distinct runtime metric instances and
  invalid siblings are skipped without blocking valid metrics.
- Context cancellation mid-batch: `WriteData` checks `ctx.Done()` between
  records.

## Code references
- `pumps/prometheus.go:19-31` — `PrometheusPump` struct (pre-declared
  counter/histogram vecs plus `allMetrics` slice).
- `pumps/prometheus.go:34-52` — `PrometheusConf` (addr, path,
  aggregate_observations, disabled_metrics, track_all_paths,
  custom_metrics).
- `pumps/prometheus.go:61-96` — `PrometheusMetric` (name, help, type, buckets,
  labels, obfuscation, MCP-only).
- `pumps/prometheus.go:137-174` — `CreateBasicMetrics` defines the five base
  metrics.
- `pumps/prometheus.go:187-221` — `Init` starts the listener
  (`http.Handle(p.conf.Path, promhttp.Handler())`).
- `pumps/prometheus.go:246-263` — `InitCustomMetrics` initializes
  operator-defined custom metrics and appends valid metrics to `allMetrics`.
- `pumps/prometheus.go:304-336` — `processMetric` handles enabled, MCP-only,
  counter, and histogram (with special-case `tyk_latency` 3-type expansion).
- `pumps/prometheus.go:339-373` — `WriteData` iterates records and calls
  `Expose()` per metric; also applies the tracking-path gate before metric
  processing.
- `pumps/prometheus.go:378-413` — `InitVec` registers vecs with the
  prometheus registry.
- `pumps/prometheus.go:438-487` — `GetLabelsValues` projects records to
  label values (including MCP labels and `obfuscateAPIKey`).
- `pumps/prometheus.go:560-584` — `Expose` writes accumulated counters /
  aggregated histograms into the prometheus vecs.

## Evidence
- `pumps/prometheus_test.go` covers metric initialisation, label projection,
  obfuscation, MCP handling, and aggregate observations.
- `pumps/prometheus_test.go:TestPrometheusDisablingMetrics` and
  `TestPrometheusDisabledMetricsDoNotDisableCustomMetrics` cover the
  SW-REQ-090 disabled-base-family gate.
- `pumps/prometheus_test.go:TestPrometheusEnsureLabels` covers the SW-REQ-091
  histogram `type` label schema normalization.
- `pumps/prometheus_test.go:TestPrometheusInitCustomMetrics` covers the
  SW-REQ-094 custom metric identity and invalid-sibling behavior.
- `pumps/udp_file_pumps_mcdc_test.go`
  `TestPrometheusPump_WriteData_NoTracking`,
  `TestPrometheusPump_WriteData_TrackedRecord`, and
  `TestPrometheusPump_WriteData_TrackAllPaths` drive the path-label policy at
  the `WriteData` boundary.

## Open questions
- `Init` calls `http.Handle(...)` on the default mux and `http.ListenAndServe`
  in a goroutine; reconfiguring or restarting the pump in-process would leak
  the handler. The requirement doesn't capture the once-per-process
  constraint.
- `log.Fatal` inside the listener goroutine takes down the whole process on
  a bind failure after `Init` has already returned success.
- `getAverageRequestTime` divides by `c.hits` with no zero guard
  (`prometheus.go:582`). Reachable only if `Expose` runs against an empty
  bucket, which the current code prevents — but it's a latent bug.
- `WriteData` returns an error when the context is cancelled mid-batch, but
  prior-iteration mutations to `counterMap`/`histogramMap` persist — partial
  exposition can happen.
- Custom-metric registration uses `prometheus.MustRegister`, which panics
  on duplicate names — Phase A could capture the uniqueness obligation.
