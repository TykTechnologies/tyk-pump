# SYS-REQ-023: Retry exhaustion surfaces error to caller

## Intent
When the configured maximum retry attempts have been exhausted for a transient failure, the pump surfaces the error to its caller without further retries. This satisfies parent **STK-REQ-002** by closing the loop on SYS-REQ-006: retries are bounded *and* termination is observable upward, so operators see a real error instead of silent indefinite retrying.

## Motivation
Indefinite retry loops are a known operational hazard — they mask outages and accumulate goroutines. By contract, after `maxRetries` attempts the HTTP retry strategy returns the underlying error so calling code can decide what to do (log it, fall back, fail the batch). Capturing this as its own SYS req (split from SYS-REQ-006 in Phase 0.6) makes the termination side atomic and code-verifiable: it is satisfied iff `backoff.RetryNotify` is allowed to return naturally.

## Formalization
```
when retry_attempts_exhausted delivery shall always satisfy error_surfaced_to_caller
```
The input `retry_attempts_exhausted` becomes true once `backoff.WithMaxRetries(..., maxRetries)` has been consumed; the output `error_surfaced_to_caller` becomes true when `BackoffHTTPRetry.Send` returns the propagated error to its caller. Variables: `specs/system/variables/delivery.vars.yaml`.

## Code references
- `retry/http-retry.go:96-98` — `return backoff.RetryNotify(opFn, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), s.maxRetries), func(err error, t time.Duration) { ... })` — the explicit max-retries wrap plus the propagated return.
- `retry/http-retry.go:34 NewBackoffRetry(errMsg string, maxRetries uint64, ...)` — `maxRetries` arrives from the calling pump's configuration.
- `retry/http-retry.go:93 return backoff.Permanent(err)` — non-retryable errors bypass the loop and return immediately (related but distinct surfacing path).

## Evidence
- `retry/http_retry_test.go:44 TestBackoffHTTPRetry_Send_PermanentOn4xx` — verifies the permanent-error surfacing path.
- `retry/http_retry_test.go:64 TestIsErrorRetryable` — covers the retryable classification.
- `retry/http_retry_branches_test.go` — additional branch coverage including retry exhaustion shapes.
- Satisfying SW child: **SW-REQ-030** (HTTP retry strategy).

## Open questions
- Phase 0.6 origin: spun out of SYS-REQ-006 to make the termination side atomic and code-verifiable.
- The storage-side `GetTemporalStorageExponentialBackoff` sets `MaxElapsedTime = 0` — meaning retries never naturally exhaust on time. The SYS req says "configured maximum retry attempts have been exhausted"; for storage the relevant cap is attempts-via-`backoff.Retry` (`storage/temporal_storage.go:347`), and exhaustion behaviour is identical (return error), but the SYS req's wording is HTTP-centric.
- Non-HTTP backends (SQL, Mongo, Kafka, Influx) implement their own retry — each must independently satisfy this surfacing obligation; SW-REQ-018..029 collectively carry that.
