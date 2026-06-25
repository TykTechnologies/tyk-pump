# SW-REQ-079: Prometheus tracked path labels are preserved

## Intent
When `PrometheusConf.TrackAllPaths` is true, or an individual analytics record
sets `TrackPath`, the Prometheus pump shall preserve the record's raw request
path in the emitted `path` metric label.

## Motivation
The path-label policy has two sides: untracked paths collapse to `unknown`, but
explicitly tracked paths must remain visible for route-level dashboards. This
requirement captures the open-gate side of TT-9873 (`69f5f4a`) so a future
regression cannot satisfy the cardinality limit by collapsing paths that the
operator or API explicitly opted into tracking.

## Code references
- `pumps/prometheus.go` `PrometheusPump.WriteData` leaves `record.Path`
  unchanged when `TrackAllPaths || record.TrackPath` is true.
- `pumps/udp_file_pumps_mcdc_test.go`
  `TestPrometheusPump_WriteData_TrackedRecord` verifies the per-record gate.
- `pumps/udp_file_pumps_mcdc_test.go`
  `TestPrometheusPump_WriteData_TrackAllPaths` verifies the global gate.

## Evidence
- `// SW-REQ-079:metric_path_label_tracking_policy:nominal`
  on both boundary tests.
- MC/DC rows for
  `track_all_paths_enabled | record_track_path_enabled -> path_label_preserved`
  are recorded in `pumps/udp_file_pumps_mcdc_test.go`.

## Open questions
- Preserving raw paths is an intentional operator/API choice, but it can still
  create high-cardinality metrics. That residual production-risk budget remains
  tracked under `metrics-label-cardinality-unbounded`.
