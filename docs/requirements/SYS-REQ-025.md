# SYS-REQ-025: Temporal store must be Redis-protocol compatible

## Intent
The temporal-store backend shall be Redis-protocol compatible — Redis v5+ or a derivative that speaks the same wire protocol (Valkey, KeyDB, AWS ElastiCache, etc.). Non-Redis storage protocols are out of scope. This constraint satisfies parent **STK-REQ-002** by capturing the wire-level contract the ingestion path is verified against.

## Motivation
The temporal store sits on the hot path between the gateway and every backend pump; switching it to a non-Redis-protocol store (Memcached, etcd, a SQL queue) would require rewriting the entire ingestion path, not just swapping a driver. Documenting the protocol pin as a SYS-layer constraint makes that boundary explicit so operators do not silently misconfigure a non-compatible backend (which would surface as runtime errors rather than as a startup failure), and so reviewers of future storage-layer changes know they must preserve LPUSH/LPOP semantics. The constraint underpins SYS-REQ-007 (atomic record removal), SYS-REQ-028 (LPOP atomicity assumption), and the entire `storage/temporal_storage.go` design.

## Formalization
```
storage shall always satisfy redis_protocol_compatible
```
This is a project-level invariant: the storage backend tyk-pump connects to must implement the Redis wire protocol, version 5 or later. Verified by the `TykTechnologies/storage` library that `storage/temporal_storage.go` is built on — that library speaks only the Redis protocol. Connecting to a non-Redis-protocol endpoint produces a connection error at `ensureConnection()` rather than a degraded write path.

## Code references
- `storage/temporal_storage.go:3-11` — imports `github.com/TykTechnologies/storage/temporal/connector|keyvalue|list|model`, the Redis-protocol-only client library.
- `storage/temporal_storage.go:262 GetAndDeleteSet` — the LPOP-equivalent the entire purge loop is built on; only meaningful against a Redis-protocol store.
- `go.mod:9 github.com/TykTechnologies/storage v1.3.1` — pinned dependency that realises the constraint.
- `storage/store.go:13 AnalyticsStorage` interface — declared only in terms of operations that map to Redis primitives.

## Evidence
- `storage/temporal_storage_test.go:47 TestRedisClusterStorageManager_GetAndDeleteSet` — exercises the LPOP-equivalent against a real Redis instance.
- `storage/temporal_storage_negative_test.go:13 TestTemporalStorageHandler_GetAndDeleteSet_BackendUnreachable` — verifies clean failure when the protocol endpoint is unreachable.
- Phase B constraint review (`verification.review.comment`): "Redis dep in storage/temporal_storage.go imports."

## Open questions
- "Redis-protocol compatible" is a spectrum: Valkey is a clean drop-in but earlier KeyDB forks diverged on subtle ACL behaviour. The constraint does not enumerate which derivatives are tested in CI; operators are on the hook for verifying compatibility with their chosen variant.
- Operator-facing follow-up: a Helm-chart-level precondition check (`PING` + version probe) at pump startup would catch protocol mismatches earlier than the first `GetAndDeleteSet` call.
- The constraint pins protocol compatibility but not high-availability shape — Sentinel, Cluster, and standalone are all covered, but the SYS req does not say which the pump *prefers*.
