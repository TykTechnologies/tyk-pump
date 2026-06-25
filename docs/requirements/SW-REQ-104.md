# SW-REQ-104: Prometheus latency metric value projection

## Intent
The Prometheus built-in `tyk_latency` histogram must project each analytics
record into three latency observations for the same API:

- `type="total"` uses `AnalyticsRecord.RequestTime`.
- `type="upstream"` uses `AnalyticsRecord.Latency.Upstream`.
- `type="gateway"` uses `AnalyticsRecord.Latency.Gateway`.

This child requirement sits under SW-REQ-024 so the broad Prometheus scrape
contract does not hide the source field for gateway-only latency.

## Motivation
TT-14871 added gateway-only latency as a first-class metric signal. The field is
useful only if downstream Prometheus consumers can distinguish total request
time, upstream latency, and gateway latency by label and trust that each value
comes from the corresponding analytics record field.

## Formalization
```
when latency_metric_record_present pumps_prometheus shall always satisfy latency_metric_values_projected
```

Variables are declared in `specs/software/variables/pumps-prometheus.vars.yaml`.

## Code References
- `pumps/prometheus.go:processMetric` routes built-in `tyk_latency` histogram
  metrics through the latency-specific observation path.
- `pumps/prometheus.go:observeLatencyMetrics` emits the `total`, `upstream`,
  and `gateway` histogram observations.

## Evidence
- `pumps/prometheus_test.go:TestProcessMetric_HistogramType_LatencyMetric`
  collects the histogram output and asserts the `total`, `upstream`, and
  `gateway` label/value pairs for one analytics record.
