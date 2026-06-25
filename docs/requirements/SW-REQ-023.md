# SW-REQ-023: StatsD and DogStatsD pumps — timing and histogram metrics

## Intent
The StatsD pump shall emit one timing metric per analytics record per
configured timing-field (`request_time`, `latency_total`, `latency_upstream`,
`latency_gateway`) to a StatsD server using `quipo/statsd`; `latency_gateway`
is projected from `AnalyticsRecord.Latency.Gateway`. The DogStatsD
pump shall emit one `request_time` histogram metric per record to a DogStatsD
agent using the official `DataDog/datadog-go` client, with operator-
configurable tags (or a sensible default tag set). Derived from SYS-REQ-004
(independent per-backend delivery).

## Motivation
Per the Phase-0 verification: the original obligation text referenced a single
"per-record metric" guarantee, but the two pumps emit different *kinds* of
metrics. StatsD emits `Timing` (a measurement type designed for latency
distributions, statsd-side aggregation) while DogStatsD emits `Histogram` (a
type with native percentile aggregation in Datadog). They are not
substitutable — operators choose one based on which collector they run.

The StatsD pump's timing-field whitelist (`isTimingField`) prevents typo'd
configs from producing nonsense metrics: only fields that genuinely represent
durations are accepted. Tag-string construction (`metric` = `field + "." +
metricTags`) is dot-separated rather than DogStatsD's `key:value` form because
plain StatsD has no native tagging — operators encode dimensions into the
metric name.

DogStatsD's default tag set (`dogstatsd.go:207-220`) is explicitly bounded
(no path, no api_key) because Datadog meters by metric cardinality; the
inline doc comment warns about the `path` tag in particular. When the
operator overrides with `Tags`, only whitelisted dimension names are honoured;
unknown tag names return an error from `WriteData`.

## Code references
### StatsD
- `pumps/statsd.go:16-19` — `StatsdPump` struct.
- `pumps/statsd.go:25-37` — `StatsdConf` (address, fields, tags,
  separated_method).
- `pumps/statsd.go:76-94` — `connect()` retries indefinitely (no bound).
- `pumps/statsd.go:96-119` — `isTimingField` whitelist; `sendTimingMetric`
  calls `client.Timing(metric, value)`.
- `pumps/statsd.go:122-182` — `WriteData` iterates records and timing fields.

### DogStatsD
- `pumps/dogstatsd.go:26-30` — `DogStatsdPump` struct (persistent client).
- `pumps/dogstatsd.go:33-102` — `DogStatsdConf` (namespace, sample rate,
  buffered, async UDS, tags).
- `pumps/dogstatsd.go:174-186` — `connect` with prefixed namespace and
  `tyk-pump` tag.
- `pumps/dogstatsd.go:189-262` — `WriteData` builds tag list (default or
  operator-supplied) and calls `client.Histogram("request_time", ...)`.
- `pumps/dogstatsd.go:265-270` — `Shutdown` flushes if buffered.

## Evidence
- `pumps/statsd_test.go` — covers `isTimingField`, mapping construction, and
  `latency_gateway` projection.
- `pumps/udp_file_pumps_mcdc_test.go:TestStatsdPump_WriteData_ManyTimingFields`
  proves configured StatsD timing fields emit `request_time`,
  `latency_total`, `latency_upstream`, and `latency_gateway` lines with the
  source record values.
- No `dogstatsd_test.go` (gap).
- End-to-end tests need a running StatsD/DogStatsD endpoint and are excluded
  from the local audit MC/DC scope (recorded as a known issue).

## Open questions
- StatsD emits `Timing`; DogStatsD emits `Histogram`. These are different
  metric kinds with different aggregator semantics — Phase A should split
  into two atomic SW-REQs to make the guarantee per-implementation precise.
- StatsD `connect()` and DogStatsD `connect()` have no retry bound; a
  permanent outage will sleep-loop forever.
- StatsD's hard-coded timing-field whitelist (`isTimingField`) is a hidden
  contract: operators who name a non-timing field as a "field" silently get
  no metric. Phase A could either document or relax this.
- DogStatsD has no dedicated test file at all.
- The unknown-tag error path in DogStatsD's `WriteData` returns from the
  whole batch on the first unknown tag (`return fmt.Errorf(...)`), which is
  a more aggressive failure than the rest of the family.
