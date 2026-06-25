# SW-REQ-083: Prometheus label tuple boundaries

Documents: SW-REQ-083

The Prometheus pump aggregates counters and, when configured, histograms in
memory before exposing them to the Prometheus client. Some aggregate maps use a
string key derived from label values joined with the internal separator `--`.

## Contract

When a metric label value itself contains `--`, aggregation must preserve the
original label tuple separately from the internal map key. Expose-time code must
replay the stored label values as originally observed instead of splitting the
joined key and accidentally turning one label value into multiple labels.

## Evidence

- `pumps/prometheus_test.go:TestPrometheusCounterMetric` includes counter
  records with `APIID: "api--3"` and asserts the stored label tuple is
  `[]string{"500", "api--3"}` / `[]string{"200", "api--3"}`.
- `pumps/prometheus_test.go:TestPrometheusHistogramMetric` includes an
  aggregated histogram record with `APIID: "api--3"` and asserts the stored
  label tuple is `[]string{"total", "api--3", "GET", "health"}`.

## Known Issues

This requirement is not a full label-cardinality budget. Broader Prometheus,
StatsD, and DogStatsD high-cardinality label/tag risk remains tracked under
`metrics-label-cardinality-unbounded`.
