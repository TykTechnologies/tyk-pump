# STK-REQ-002: Resilient multi-backend delivery

## Intent
Operators of a Tyk deployment routinely configure several heterogeneous
analytics backends (e.g. SQL for billing, Elasticsearch for search, Splunk
for SIEM) behind a single pump. This stakeholder requirement asserts the
operator-visible property that one unhealthy or slow sink must not take down
delivery to the others, and that transient storage / backend faults must be
retried rather than silently dropped on first failure.

## Motivation
Without backend isolation, a single misbehaving sink (an Elastic cluster in
recovery, a Mongo primary failover, a Splunk HEC outage) would cascade into
loss of analytics for *all* sinks — the pump would block on the slow writer
and Redis would either back up or drop records under its TTL/length policy.
Operators report this class of failure as "we lost an hour of analytics
across the estate because Splunk was slow", and it directly motivates
parallel, per-backend writes with per-backend timeouts.

The retry leg of the requirement embodies the inverse trade-off: not every
error is fatal. Network blips, Redis reconnects, and transient 5xx responses
from HTTP-based sinks should be absorbed by bounded backoff, not surfaced as
record loss. The "bounded" part matters — unbounded retries would themselves
become a back-pressure failure mode, so SYS-REQ-023 caps the retry attempts.

## Code references
Decomposes into the following SYS reqs via its acceptance criteria:
- AC-001 (per-backend isolation): `SYS-REQ-004` (independent writes per
  backend), `SYS-REQ-005` (per-backend timeout).
- AC-002 (retry bounded backoff): `SYS-REQ-006` (exponential backoff retry),
  `SYS-REQ-007` (atomic at-most-once consume from temporal store),
  `SYS-REQ-023` (surface error after max retries).

Implementation entry points:
- Per-backend isolation: `main.go:435` `execPumpWriting` runs each pump in
  its own goroutine with its own `context.WithTimeout`, fed by
  `writeToPumps` at `main.go:361`.
- Storage backoff: `retry/storage-retry.go:10` `GetTemporalStorageExponentialBackoff`,
  used from `storage/temporal_storage.go:328` `ensureConnection`.
- HTTP backoff: `retry/http-retry.go:39` `BackoffHTTPRetry.Send` (5xx, 429,
  and connection errors retried; other errors marked `backoff.Permanent`).

## Evidence
- Per-pump goroutine isolation is exercised indirectly by `main_test.go` and
  by injecting failing dummy pumps (`pumps/dummy.go`).
- Retry policy is exercised by `retry/storage_retry_test.go`,
  `retry/http_retry_test.go`, `retry/http_retry_branches_test.go`.
- Timeout enforcement: `main.go:460-491` covers
  `context.DeadlineExceeded` and logs without affecting other pumps; the
  per-pump timeout setter is `pumps/common.go` (see `SetTimeout`).

## Open questions
- "Neither lost nor duplicated in normal operation" is asserted by the
  stakeholder text but the system does not have an end-to-end test that
  proves dedup across a successful purge — `GetAndDeleteSet` is atomic on
  the Redis side (`LPOP n` + `EXPIRE`) but downstream write failure does not
  re-enqueue. So records can be lost (not duplicated) once a write fails and
  retries exhaust. This conflict between stakeholder text and implementation
  semantics is not flagged in any SYS req.
- "Slow backend never blocks delivery to the others" — true only if the
  operator configures a timeout. If `pmp.GetTimeout() == 0`, the per-pump
  goroutine can run indefinitely; only a warning is logged at `main.go:438`.
