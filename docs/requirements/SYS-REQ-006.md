# SYS-REQ-006: Retry transient failures with bounded exponential backoff

## Intent
On a transient temporal-store or backend failure, the pump attempts a retry using bounded exponential backoff rather than failing-fast or busy-looping. This error-handling obligation satisfies the parent **STK-REQ-002** by ensuring intermittent network/storage hiccups do not directly translate into dropped analytics records or operator-visible alerts.

## Motivation
Backends and Redis instances momentarily flap during routine operations (rolling restarts, brief network blips). Without retry-with-backoff the pump would amplify these into elevated error rates downstream, and a tight retry loop without backoff would itself become the incident. Codifying bounded exponential backoff at the SYS layer fixes both behaviours in one place. Phase 0.6 split the original SYS-REQ-006: the "what happens once retries are exhausted" obligation moved to companion **SYS-REQ-023**.

## Formalization
```
when transient_failure delivery shall eventually satisfy retry_attempted
```
The input `transient_failure` fires when an operation returns a retryable error (network, 5xx, 429, connection-reset); the output `retry_attempted` becomes true once a backoff-scheduled re-invocation is issued. Variables: `specs/system/variables/delivery.vars.yaml`.

## Code references
- `retry/http-retry.go:34 NewBackoffRetry` and `:39 Send` — the HTTP-side retry: `backoff.RetryNotify(opFn, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), maxRetries), ...)`.
- `retry/http-retry.go:111 isErrorRetryable` — classifies which errors trigger retry vs `backoff.Permanent`.
- `retry/storage-retry.go:10 GetTemporalStorageExponentialBackoff` — multiplier 2, max interval 10s, unbounded max elapsed time.
- `storage/temporal_storage.go:328 ensureConnection` — invokes `backoff.Retry(operation, backoffStrategy)` when the connector is dropped.

## Evidence
- `retry/http_retry_test.go:28 TestBackoffHTTPRetry_Send_Success`, `:44 TestBackoffHTTPRetry_Send_PermanentOn4xx`, `:64 TestIsErrorRetryable` — cover both retryable and permanent classes.
- `retry/http_retry_branches_test.go` — exercises additional branch combinations.
- `retry/storage_retry_test.go` — verifies the backoff parameters.
- `storage/temporal_storage_test.go:161 TestTemporalStorageHandler_ensureConnection` — drop-and-reconnect path.
- Satisfying SW children: **SW-REQ-030** (HTTP retry strategy), **SW-REQ-031** (storage retry strategy), **SW-REQ-007** (temporal storage connect / reconnect).

## Open questions
- Phase 0.6 split: error surfacing on exhaustion is now **SYS-REQ-023**.
- "Bounded" is realised differently per call site: HTTP uses `WithMaxRetries(..., maxRetries)` (a count cap); storage uses `MaxElapsedTime = 0` (no time cap) so retry continues until success — the SYS req does not distinguish these two bounding strategies.
- Only HTTP-shaped backends use the shared `retry` package; non-HTTP backends (SQL, Mongo, Kafka, Influx) each implement their own retry policy or rely on the underlying driver — those policies are not collected under one SYS req.
