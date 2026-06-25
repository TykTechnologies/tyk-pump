# SW-REQ-090: Prometheus disabled base metric families

## Intent
When `disabled_metrics` names a built-in Prometheus base metric family exactly,
the Prometheus pump must omit that family from base metric registration and from
the later `WriteData` update/exposition loop. The controlled built-in family
set is:

- `tyk_http_status`
- `tyk_http_status_per_path`
- `tyk_http_status_per_key`
- `tyk_http_status_per_oauth_client`
- `tyk_latency`

`disabled_metrics` does not suppress operator-defined custom metrics.

## Motivation
Before TT-6799, the Prometheus pump initialized every built-in metric family on
startup. Operators could not remove high-cardinality or unwanted base families
from the scrape surface. The fix added an exact-name filter before base metric
registration, so disabled built-ins are not present in `allMetrics` and cannot
be updated or exposed by `WriteData`.

## Formalization
```
when base_metric_disabled pumps_prometheus shall always satisfy base_metric_family_absent
```

Variables are declared in `specs/software/variables/pumps-prometheus.vars.yaml`.

## Code References
- `pumps/prometheus.go:initBaseMetrics` builds the `disabled_metrics` exact-name
  set and skips matching built-in base metric families before `InitVec`.
- `pumps/prometheus.go:WriteData` updates and exposes only families retained in
  `p.allMetrics`.
- `pumps/prometheus.go:InitCustomMetrics` appends custom metrics after base
  metric filtering, so the built-in disable gate does not suppress custom
  metrics.

## Evidence
- `pumps/prometheus_test.go:TestPrometheusDisablingMetrics` proves every
  named built-in family in the controlled set is absent from `allMetrics` and
  leaves its family name available for registration in an isolated Prometheus
  registry when disabled by exact name, and that an unknown disabled name does
  not suppress built-in families.
- `pumps/prometheus_test.go:TestPrometheusDisabledMetricsDoNotDisableCustomMetrics`
  proves a custom metric named in `disabled_metrics` is still initialized.
