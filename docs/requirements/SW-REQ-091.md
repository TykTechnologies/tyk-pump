# SW-REQ-091: Prometheus histogram type label schema

## Intent
When a Prometheus histogram metric is initialized, its label schema must contain
exactly one synthetic `type` label in position 0. Any configured `type` label is
deduplicated and moved to that first position, while all configured non-`type`
labels keep their original relative order. Counter metrics are not modified by
this histogram-only normalization.

## Motivation
Prometheus binds label values positionally. The pump's histogram observation
path prepends a synthetic latency type value such as `total`, `upstream`, or
`gateway`. Before TT-6482, a histogram could be registered without a matching
first `type` label or with duplicate/misordered `type` labels, causing value
misbinding or cardinality errors at observe time.

## Formalization
```
when histogram_metric_configured pumps_prometheus shall always satisfy histogram_type_label_schema_normalized
```

Variables are declared in `specs/software/variables/pumps-prometheus.vars.yaml`.

## Code References
- `pumps/prometheus.go:PrometheusMetric.InitVec` calls `ensureLabels` for
  histogram metrics before registration.
- `pumps/prometheus.go:PrometheusMetric.ensureLabels` removes all configured
  `type` labels, then prepends exactly one `type` label.
- `pumps/prometheus.go:PrometheusMetric.Observe` supplies the synthetic
  histogram type value in the first label-value position.

## Evidence
- `pumps/prometheus_test.go:TestPrometheusInitVec` proves histogram
  initialization applies the exact label normalization before registration,
  including missing and duplicate configured `type` labels, while counter
  labels remain unchanged.
- `pumps/prometheus_test.go:TestPrometheusEnsureLabels` asserts exact
  normalized label slices for missing, existing, middle-position, duplicate,
  empty, and counter schemas.
- `pumps/prometheus_test.go:TestPrometheusHistogramMetric` exercises histogram
  observe/expose behavior after schema normalization.
