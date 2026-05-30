# SW-REQ-031: Storage retry exponential backoff strategy

## Intent
Realises the storage-call resilience side of parent **SYS-REQ-006**. `retry.GetTemporalStorageExponentialBackoff` returns a freshly-constructed `*backoff.ExponentialBackOff` configured with `Multiplier = 2`, `MaxInterval = 10 * time.Second`, and `MaxElapsedTime = 0` (meaning "no overall deadline"). The storage layer (`storage/temporal_storage.go:ensureConnection`) plugs this strategy into `backoff.Retry` so that connection-loss recovery against the temporal backend (Redis) doubles its sleep between attempts up to a 10s ceiling and never gives up of its own accord.

## Motivation
A separate factory (rather than calling `NewExponentialBackOff` inline) means every storage caller gets identical retry semantics, and the strategy can be tuned in one place. The 10s cap is a deliberate ceiling: with the library default `MaxInterval = 60s`, a long Redis outage would degrade to one reconnect attempt per minute — too slow to recover smoothly when the backend comes back. `MaxElapsedTime = 0` ("retry forever") is appropriate for a long-running data pipeline that should keep trying as long as the process is alive; the surrounding code (`ensureConnection`) is the only way to escape — via process shutdown. Trade-off: a `*ExponentialBackOff` carries internal state (next interval, start time), so the factory must return a *fresh* instance per use — re-using one across calls would make later attempts start from the already-grown interval. The current callers (`ensureConnection`) call the factory each time.

## Code references
- `retry/storage-retry.go:10 GetTemporalStorageExponentialBackoff` — the 7-line factory.
- `storage/temporal_storage.go:334` — the only production caller: `backoffStrategy := retry.GetTemporalStorageExponentialBackoff(); backoff.Retry(operation, backoffStrategy)` inside `ensureConnection`.
- Library defaults inherited from `backoff.NewExponentialBackOff()`: initial interval 500ms, randomization factor 0.5.

## Evidence
- `retry/storage_retry_test.go:10 TestGetTemporalStorageExponentialBackoff` — tagged `// SW-REQ-031:error_handling:example`; asserts the three configured fields (multiplier, max interval, max elapsed time).
- `retry/storage_retry_test.go:25 TestGetTemporalStorageExponentialBackoff_FreshInstances` — tagged `// SW-REQ-031:error_handling:boundary`; asserts each call returns an independent instance (no shared state across callers).
- `storage/temporal_storage_negative_test.go:13 TestTemporalStorageHandler_GetAndDeleteSet_BackendUnreachable` — tagged `// SW-REQ-031:error_handling:negative`; integration-style assertion that the strategy actually drives the retry loop in `ensureConnection`.

## Open questions
- `MaxElapsedTime = 0` is documented in the `cenkalti/backoff` library as "no max elapsed time"; the req description says "bounded exponential backoff" but the bound is per-interval (10s), not on total elapsed time. Wording overstates the boundedness slightly — see also SW-REQ-007's open question.
- The strategy is *not* parameterised — there is no way for a deployment to lengthen the cap or shorten the multiplier without recompiling. If operators ever need this knob, it would belong in `config.go` rather than as a hard-coded factory.
