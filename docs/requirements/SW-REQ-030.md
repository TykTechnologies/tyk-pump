# SW-REQ-030: HTTP retry with bounded exponential backoff

## Intent
Realises the HTTP-call resilience side of parent **SYS-REQ-006**. `retry.BackoffHTTPRetry.Send` buffers the request body, then drives `httpclient.Do(req)` inside `backoff.RetryNotify` with a `WithMaxRetries(NewExponentialBackOff(), maxRetries)` strategy. Each attempt re-creates `req.Body` from the buffered bytes (avoiding the "ContentLength=X with Body length Y" trap on retry) and reads/discards the response body so the HTTP client can reuse the connection. Retries fire on transient errors (`isErrorRetryable`), 5xx responses, and 429; 4xx responses other than 429 are wrapped in `backoff.Permanent` so they exit the loop immediately.

## Motivation
Idempotent backend write retries are the standard way to ride out brief network blips and overload signals without losing data. The "buffer body once, replay on each attempt" pattern is what makes `Send` safe to call on requests with bodies; the alternative (caller-supplied `io.Reader`) is footgun-prone because most readers are single-pass. The error classification at `isErrorRetryable` distinguishes "connection-level transient" (retry) from "application-level error like auth fail" (permanent) — important so that a misconfigured token doesn't burn through `maxRetries` attempts.

## Code references
- `retry/http-retry.go:18 BackoffHTTPRetry` — the struct carrying logger, client, error message, max-retry count.
- `retry/http-retry.go:34 NewBackoffRetry` — constructor.
- `retry/http-retry.go:39 Send` — body buffer (lines 40-49), the `opFn` closure that recreates the body per attempt (line 55), `RetryNotify` at line 96. Status classification: 200 OK at line 74 returns nil; 5xx or 429 at line 88 returns the error for retry; otherwise `backoff.Permanent`.
- `retry/http-retry.go:102 handleErr` — wraps non-retryable errors in `backoff.Permanent`.
- `retry/http-retry.go:111 isErrorRetryable` — `errors.As` chain over `ConnectionError`, `Temporary`, `Timeout` interface adapters, plus string-match for "connection reset" and `*url.Error` "connection refused" cases.

## Evidence
- `retry/http_retry_test.go:28 TestBackoffHTTPRetry_Send_Success` — tagged `// SW-REQ-030:error_handling:example`; nominal success path.
- `retry/http_retry_test.go:44 TestBackoffHTTPRetry_Send_PermanentOn4xx` — tagged `// SW-REQ-030:error_handling:negative`; asserts 4xx exits without retry.
- `retry/http_retry_test.go:64 TestIsErrorRetryable` — tagged `// SW-REQ-030:error_handling:boundary`; table-driven classification.
- `retry/http_retry_branches_test.go:42 TestIsErrorRetryable_AllBranches` — tagged `// SW-REQ-030:error_handling:negative`; MC/DC-style coverage of each branch in `isErrorRetryable`.
- `retry/http_retry_branches_test.go:68 TestBackoffHTTPRetry_Send_WithBody_RetriesOn5xx` — body replay on retry.
- `retry/http_retry_branches_test.go:TestBackoffHTTPRetry_Send_ReplaysRequestBodyOnRetry`
  carries the local `request_body_replay_preserved` obligation evidence. It is
  not a formal MC/DC witness for the retry-trigger formula.
- `retry/http_retry_branches_test.go:96 TestBackoffHTTPRetry_Send_ErrorSurfacedAfterRetriesExhausted` — `maxRetries` exhaustion.

## Open questions
- The "idempotent backend calls" framing in the req is operator-stated, not enforced: `Send` will retry a `POST` just as cheerfully as a `GET`. Callers must only pass idempotent requests; nothing guards against double-write of a non-idempotent payload.
- `NewExponentialBackOff()` uses library defaults (initial 500ms, multiplier 1.5, max interval 60s, randomization 0.5, max elapsed time 15 minutes). Wrapping it in `WithMaxRetries(..., maxRetries)` adds an attempt-count cap. There is no per-attempt request timeout — the underlying `http.Client.Timeout` is the only stop-the-attempt control.
- `io.Copy(io.Discard, resp.Body)` runs in a `defer` — if the body read itself errors (e.g. connection reset mid-response), the error is logged but does not become a retry signal.
