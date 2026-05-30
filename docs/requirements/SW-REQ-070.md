# SW-REQ-070: Elasticsearch — bulk-flush size and interval boundary policy

## Parent
This requirement is a per-significant-behaviour decomposition of the
previous family req SW-REQ-020 (elasticsearch). It carries the
BulkProcessor configuration obligation in isolation.

## Intent
For each ES version operator, when `DisableBulk` is false the
`BulkProcessor` shall be configured with operator-supplied
`BulkConfig.Workers`, `FlushInterval` (seconds), `BulkActions` (record
count, default 1000, `-1` to disable), `BulkSize` (bytes, default 5 MB,
`-1` to disable), and an `After` callback that logs the purged record
count via `printPurgedBulkRecords`. When `DisableBulk` is true, each
record is indexed individually via the per-version `Index().BodyJson
(...).Do(ctx)` call. `Shutdown` flushes the bulk processor when bulk is
enabled. Derived from SYS-REQ-004.

## Motivation
The BulkProcessor is the per-version client's standard high-throughput
write path; the configuration knobs (workers, flush interval, bulk
actions, bulk size) let operators tune the trade-off between write
latency and per-request overhead. The `-1` sentinel on `BulkActions` /
`BulkSize` lets operators disable one bound without disabling the other
(e.g. flush only on the interval). `DisableBulk` exists for operators
who need per-record durability guarantees the BulkProcessor cannot
provide (the processor buffers in memory between flushes).

## Code references
- `pumps/elasticsearch.go:ElasticsearchPump.getOperator` — per-version
  BulkProcessor instantiation with the config knobs above.
- `pumps/elasticsearch.go:Elasticsearch{3,5,6,7}Operator.processData` —
  per-version write path that hands records to the BulkProcessor (or
  to per-record `Index()` when `DisableBulk` is true).
- `pumps/elasticsearch.go:printPurgedBulkRecords` — the `After`
  callback.
- `pumps/elasticsearch.go:ElasticsearchPump.Shutdown` — flushes the
  BulkProcessor.

## Evidence
- The bulk-flush path is exercised end-to-end by the live-ES tests
  (excluded from the local audit MC/DC scope per the known issue);
  there is currently no isolated unit test for the BulkProcessor
  configuration knobs.
- The mapping helper exercised by `pumps/elasticsearch_test.go:TestGetMapping_*`
  (annotated against SW-REQ-068) feeds into the BulkProcessor input
  pipeline.

## Open questions
- No isolated unit test for the BulkProcessor config knobs — operators
  rely on integration tests in production deployments to catch
  regressions in the flush policy.
- The `boundary` obligation is satisfied by the `-1` sentinel and the
  default 1000-record / 5 MiB caps; pathological operator config
  (e.g. `BulkActions: 1` + `Workers: 1000`) is not clamped.
