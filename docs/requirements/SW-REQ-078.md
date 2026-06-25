# SW-REQ-078: Prometheus untracked path labels collapse to unknown

## Intent
When `PrometheusConf.TrackAllPaths` is false and an analytics record does not
set `TrackPath`, the Prometheus pump shall emit the bounded sentinel
`unknown` into any `path` metric label instead of emitting the raw request
path.

## Motivation
TT-9873 (`69f5f4a`) fixed the tracking-path policy so untracked request paths
do not create one metric series per raw URL. This requirement captures the
closed-gate side of that fix as a first-class contract rather than relying on
the broad SW-REQ-024 metric-exposition requirement.

## Code references
- `pumps/prometheus.go` `PrometheusPump.WriteData` rewrites
  `record.Path` to `prometheusUnknownPath` when neither tracking gate is open.
- `pumps/udp_file_pumps_mcdc_test.go`
  `TestPrometheusPump_WriteData_NoTracking` drives `WriteData` and asserts the
  exported counter contains `path="unknown"` and not the raw path.

## Evidence
- `// SW-REQ-078:metric_path_label_tracking_policy:negative`
  on `TestPrometheusPump_WriteData_NoTracking`.
- MC/DC rows for
  `!track_all_paths_enabled & !record_track_path_enabled -> path_label_unknown`
  are recorded in `pumps/udp_file_pumps_mcdc_test.go`.

## Open questions
- This requirement bounds only the path label when tracking is disabled.
  Other metric labels and explicit path-tracking modes remain covered by
  `metrics-label-cardinality-unbounded` until a broader label-cardinality
  budget is implemented.
