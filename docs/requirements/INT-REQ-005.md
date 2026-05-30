# INT-REQ-005: AnalyticsStorage.GetAndDeleteSet atomic-pop contract

## Intent
The contract between the pump core and the temporal store (Redis, via the
`AnalyticsStorage` interface) for draining a chunk of analytics records:
the call must return a chunk of records and atomically remove exactly
those records from the store, so each record is forwarded at most once
per pump run. It satisfies SYS-REQ-007.

## Motivation
This is the at-most-once guarantee that lets the rest of the pipeline
reason about duplicates. If the pop and the delete were two separate Redis
operations, a crash between them would yield duplicates on restart; a
non-atomic read+delete would also race with the gateway, which is concurrently
appending records. By contract, the underlying implementation uses
`LPOP n` (Redis 6.2+) for the pop, which already removes the popped
elements atomically — but the contract is what lets the core treat the
returned slice as authoritative.

The contract also defines an `expire` parameter so the caller can extend
the TTL on the residual list, preventing a half-drained list from being
deleted out from under the pump by an aggressive idle TTL policy.

## Code references
- `storage/store.go:10-14` `AnalyticsStorage` interface declaration:
  ```
  type AnalyticsStorage interface {
      Init() error
      GetName() string
      GetAndDeleteSet(setName string, chunkSize int64, expire time.Duration)
          ([]interface{}, error)
  }
  ```
- Implementation: `storage/temporal_storage.go:262`
  `func (r *TemporalStorageHandler) GetAndDeleteSet(keyName string,
  chunkSize int64, expire time.Duration) ([]interface{}, error)`.
- The pop is at `storage/temporal_storage.go:288`
  `result, err := r.list.Pop(ctx, fixedKey, chunkSize)` — backed by the
  `TykTechnologies/storage/temporal` list driver, which issues `LPOP n`
  under the hood.
- The TTL extension is at `storage/temporal_storage.go:294`
  `r.kv.Expire(ctx, fixedKey, expire)` — only called when
  `chunkSize != -1` (i.e. when the caller specified a chunk; full-drain
  callers pass `chunkSize=0` which is rewritten to `-1` at line 285,
  signalling "pop everything, do not bother extending TTL").
- Callers: `main.go:278` (request analytics), `main.go:294` (uptime
  stream).

## Evidence
- `storage/temporal_storage_test.go` covers the happy path including
  chunk size, TTL extension, and empty-list returns.
- `storage/temporal_storage_negative_test.go:32` exercises the
  error-on-no-connection branch.

## Open questions
- "Atomically" is the contract's word, but the implementation issues
  `LPOP n` and a separate `EXPIRE` (lines 288 and 294). The EXPIRE failure
  branch (`return nil, err` at line 296) is a problem: the records have
  already been popped, but the function returns `nil` records and an
  error. The caller (`main.go:278`) logs the error and skips the batch,
  so those popped records are lost. This is a real at-most-once violation
  on the "or zero times" side and is not noted in the SYS req.
- `chunkSize=0` semantics are mapped to "drain everything" at
  `temporal_storage.go:285`, but the interface signature uses `int64` and
  the docstring on the interface doesn't say so. A new caller passing 0
  expecting "no work" gets the opposite.
