# SW-REQ-094: Prometheus custom metric identity

## Intent
`PrometheusPump.InitCustomMetrics` shall initialize one distinct runtime metric
instance for every valid configured custom metric. The stored metric must
preserve that metric's name, type, labels, aggregate-observation setting, and
initialized Prometheus collector. Invalid custom metrics are skipped, but they
must not prevent valid sibling metrics from being initialized.

## Motivation
TT-6343 fixed a custom metric identity collapse. The historical code iterated
custom metrics with `for _, metric := range ...` and appended `&metric`, so
multiple configured metrics could collapse into one runtime metric identity.
That loses operator-defined metrics even though the appended metric count can
look correct.

## Formalization
```
when valid_custom_metrics_configured pumps_prometheus shall always satisfy custom_metric_instances_preserved
```

Variables are declared in `specs/software/variables/pumps-prometheus.vars.yaml`.

## Code References
- `pumps/prometheus.go:InitCustomMetrics` iterates
  `p.conf.CustomMetrics` by index and appends valid initialized metrics.
- `pumps/prometheus.go:PrometheusMetric.InitVec` initializes the counter or
  histogram vector used by the runtime metric.

## Evidence
- `pumps/prometheus_test.go:TestPrometheusInitCustomMetrics` covers no custom
  metrics, one valid metric, multiple valid metrics, mixed counter/histogram
  metrics, and an invalid metric followed by a valid sibling. The test asserts
  count, retained metric names, labels, metric types, collectors, and distinct
  runtime metric pointers.
