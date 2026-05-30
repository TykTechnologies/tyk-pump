# SYS-REQ-028: Redis LPOP is atomic (environmental assumption)

## Intent
The Redis-protocol temporal store provides atomic LPOP semantics on list values: a popped element is removed from the list as a single observable operation, so two concurrent consumers cannot both observe the same element. This is an environmental assumption that satisfies parent **STK-REQ-002** by surfacing the at-most-once delivery guarantee tyk-pump relies on but cannot itself enforce.

## Motivation
SYS-REQ-007 (atomic record removal) and INT-REQ-005 (storage contract) both require that a record consumed by the purge loop is *also* removed from the temporal store — otherwise restart-after-crash or two concurrent purgers would double-deliver. Tyk-pump cannot enforce that atomicity in code; it is a property of the Redis wire protocol. Surfacing the assumption explicitly makes the trust boundary first-class: any future change that swapped to a custom queue or to a non-atomic pop primitive (e.g. `LRANGE` + `LTRIM` without `MULTI`) would invalidate the assumption and therefore SYS-REQ-007. Capturing it as a SYS req also gives external owners (the Redis protocol stewards) a stable artifact to review against.

## Formalization
```
storage shall always satisfy lpop_is_atomic
```
This is an environmental input invariant: across every Redis-protocol backend tyk-pump connects to, the LPOP-equivalent primitive used by `GetAndDeleteSet` must be atomic with respect to concurrent consumers. There is no trigger — the assumption must hold whenever the pump talks to the store. The truth condition is owned by the Redis protocol specification, not by tyk-pump.

## Code references
- `storage/temporal_storage.go:288 result, err := r.list.Pop(ctx, fixedKey, chunkSize)` — the single call site the entire ingestion path depends on; correctness assumes Pop is atomic.
- `storage/temporal_storage.go:262 GetAndDeleteSet` — the wrapper that callers see; the "and-delete" semantic in the name encodes the atomicity assumption.
- `storage/store.go:13 GetAndDeleteSet(setName string, chunkSize int64, expire time.Duration)` — interface declaration that the assumption is anchored to.
- `main.go:278 AnalyticsStore.GetAndDeleteSet(analyticsKeyName, chunkSize, expire)` — the call site that consumes records the gateway has pushed.

## Evidence
- External-owner review: `assumption.external_owner: team:redis-protocol`, status `open`, reviewed via `proof req assumptions review` in Phase B (`verification.review.comment`: "external-owner reviewed (Redis protocol / MongoDB / Tyk gateway / MaxMind)"). Next review date: `2026-11-30`.
- Redis documentation for `LPOP` / `LMPOP` states atomic removal as part of the contract.
- `storage/temporal_storage_test.go:47 TestRedisClusterStorageManager_GetAndDeleteSet` exercises the wrapper against a real Redis but cannot itself prove atomicity under concurrent contention — the assumption stands on the protocol guarantee, not the test.

## Open questions
- The assumption is held against the Redis protocol abstractly; non-Redis derivatives (Valkey, KeyDB) inherit it via SYS-REQ-025, but exotic protocol-compatible-but-semantically-divergent stores could lie. No CI matrix run exercises a non-canonical implementation.
- No linked KI for this assumption — the upstream-vendor confirmation status is "trusted by docs," which is the default for first-party-specified protocol primitives.
- The assumption is silent about server-side scripting (Lua, functions) that might non-atomically extend the LPOP semantic; not relevant today but worth noting if `GetAndDeleteSet` is ever rewritten on top of a script.
