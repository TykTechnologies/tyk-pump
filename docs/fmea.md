# FMEA — tyk-pump

Failure Modes and Effects Analysis for tyk-pump, authored as Phase D of the ReqProof coverage initiative. Severity scale: **H** = data loss / pipeline halt, **M** = degraded behaviour (per-pump impact, partial loss, silent drop), **S** = service-affecting but recoverable (warnings, transient noise, evidence gaps). Every row binds to a real REQ-ID under `specs/*/requirements/`; confirmed defects also link to the matching Phase E known-issue.

---

## Failure modes

### FMEA-001 — Per-pump write timeout silently dropped by mongo pump
- **Subsystem:** pump-mongo
- **Failure mode:** `MongoPump.WriteData` accepts the caller's `ctx` but passes `context.Background()` to the driver at `pumps/mongo.go:435` (and `:589` for uptime). The per-pump timeout built at `main.go:461` is not honoured for any mongo write.
- **Cause:** Defect — `ctx` is shadowed by a fresh `context.Background()` inside the inner goroutine.
- **Effect:** Under a slow / hung mongo cluster the write goroutine blocks indefinitely. `execPumpWriting` still fires `ctx.Done()` and logs "Timeout Writing to: …" (`main.go:480-491`), but the orphan goroutine holds records and a connection. Worst case: pool exhaustion and a delayed "Completed purging" log once the orphan finally returns.
- **Severity:** **H** (violates the timeout guarantee operators rely on).
- **Detection:** Partial — `main` logs a timeout; no metric correlates orphan goroutines or actual write completion.
- **Mitigation:** None in code. Tracked as **Phase E known-issue E1**. Operator workaround: lower `purge_delay`.
- **Owning requirements:** Violates **SYS-REQ-005**, **INT-REQ-004**. Add `ship_with_known_issue` carve-out on SYS-REQ-005.

### FMEA-002 — Purge loop hangs because one pump's goroutine never returns
- **Subsystem:** purge-core
- **Failure mode:** `writeToPumps` (`main.go:361`) fans out to all pumps and `wg.Wait()`s; if any goroutine never returns, the next purge tick is delayed indefinitely.
- **Cause:** A pump ignores its ctx (FMEA-001) or a backend client blocks before the request reaches the wire (DNS, TLS, pool-acquire).
- **Effect:** Records pile up in Redis. `StorageExpirationTime` (applied at `temporal_storage.go:294`) may start trimming the unread tail, causing data loss.
- **Severity:** **H**
- **Detection:** `time.AfterFunc(purge_delay, …)` (`main.go:436`) emits a warning when a pump exceeds `purge_delay`. No alerting metric.
- **Mitigation:** Each pump must honour ctx. SQL pump does (`pumps/sql.go:325` `c.db.WithContext(ctx).Create(...)`); mongo does not (FMEA-001).
- **Owning requirements:** **SYS-REQ-005**, **SYS-REQ-022**, **INT-REQ-004**.

### FMEA-003 — Temporal store (Redis) unreachable mid-purge
- **Subsystem:** storage
- **Failure mode:** `AnalyticsStore.GetAndDeleteSet` errors inside the purge loop (`main.go:278-283`); the cycle skips the affected key, logs, and continues.
- **Cause:** Redis restart, network partition, sentinel re-election, TLS expiry, cluster shard down.
- **Effect:** Records remain in Redis (LPOP did not run); picked up next successful cycle unless `StorageExpirationTime` elapses first.
- **Severity:** **M**
- **Detection:** ERROR "Is Temporal Storage down?" (`main.go:282`). No metric/alert.
- **Mitigation:** `TemporalStorageHandler.ensureConnection` (`storage/temporal_storage.go:328`) retries with bounded exponential backoff via `retry.GetTemporalStorageExponentialBackoff()`. Implements SW-REQ-007 / SYS-REQ-006.
- **Owning requirements:** **SYS-REQ-006**, **SW-REQ-007**, **SW-REQ-031**.

### FMEA-004 — Mongo aggregate document exceeds BSON 16 MiB cap
- **Subsystem:** pump-mongo (aggregate)
- **Failure mode:** `DoAggregatedWriting` upsert fails with "document size too large" when an org/window doc grows past 16 MiB (typically high-cardinality endpoint counters).
- **Cause:** High traffic across many distinct endpoints within one `AggregationTime` window.
- **Effect:** Without self-healing the batch is returned to the caller; cycle continues for other orgs. With self-healing the pump halves `AggregationTime` (`pumps/mongo_aggregate.go:430-438`) and recurses (`:327`).
- **Severity:** **M**
- **Detection:** ERROR "UPSERT Failure" (`pumps/mongo_aggregate.go:374`); `ShouldSelfHeal` (`:442`) classifies.
- **Mitigation:** `EnableAggregateSelfHealing` flag (`:62`), bounded by `AggregationTime == 1` floor (`:450`).
- **Owning requirements:** **SW-REQ-018**, **SYS-REQ-003**. Add `denial_of_service_resistant` obligation to SW-REQ-018.

### FMEA-005 — Storage cluster-mode unavailable in CI
- **Subsystem:** storage / verification environment
- **Failure mode:** Tests requiring Redis Cluster on port 6390 are skipped/fail under default CI (standalone Redis only).
- **Cause:** CI provides only standalone Redis.
- **Effect:** SW-REQ-006/007 branches unverified in cluster mode. Evidence gap, not a runtime defect.
- **Severity:** **S**
- **Detection:** Test skip / runner log.
- **Mitigation:** Tracked as Phase E known-issue E4. Recommend `environment_conditional` carve-out on SW-REQ-006/007.
- **Owning requirements:** **SW-REQ-006**, **SW-REQ-007**.

### FMEA-006 — Serializer wire-format drift between gateway and pump
- **Subsystem:** serializer
- **Failure mode:** Gateway emits a field the pump's schema doesn't understand (or vice-versa). No version negotiation exists — `serializer/serializer.go:20` selects by suffix only (`""` → msgpack, `_protobuf` → protobuf).
- **Cause:** Rolling upgrade with skew; field added without forward/backward compat thought.
- **Effect:** Msgpack silently drops unknown fields. Protobuf decode error logs at `main.go:321-325` and discards the record. No DLQ.
- **Severity:** **M** (silent field-level loss is the realistic outcome).
- **Detection:** WARN "Couldn't unmarshal analytics data" (`main.go:324`). Field-level drops are undetectable.
- **Mitigation:** None at the protocol layer. INT-REQ-006 asks pumps to tolerate unknown fields but doesn't pin a handshake. Flag as a standing `proof risk`.
- **Owning requirements:** **INT-REQ-003**, **INT-REQ-006**, **SW-REQ-008**.

### FMEA-007 — SQL pump schema migration mid-flight
- **Subsystem:** pump-sql
- **Failure mode:** Operator migrates a day-sharded `analytics_YYYYMMDD` table while a different-version pump still writes to it.
- **Cause:** Rolling upgrade; cross-pump deployment skew.
- **Effect:** Expand-only migration → safe. Contract migration → GORM `Create` (`pumps/sql.go:325`) fails with column-not-found, logged at `:327`; batch is lost (no DLQ, no rollback, no skip-on-error).
- **Severity:** **M**
- **Detection:** ERROR per failed batch.
- **Mitigation:** Operator process: expand-then-contract (INT-REQ-007 `forward_compatible`). No code-level guard.
- **Owning requirements:** **INT-REQ-007**, **SW-REQ-019**.

### FMEA-008 — Backend test environments (Kafka, ES, Influx, AWS, Moesif) unavailable in CI
- **Subsystem:** verification environment (cross-pump)
- **Failure mode:** Integration tests requiring external services don't run by default; backend-specific branches are not MC/DC-covered in the standard run.
- **Cause:** Only mongo + Redis are containerised by default.
- **Effect:** Evidence for SW-REQ-020/021/022/027/028 partially environment-conditional. No runtime defect.
- **Severity:** **S**
- **Detection:** Test skip / runner log.
- **Mitigation:** Phase E known-issue E5. Recommend `environment_conditional` carve-outs per affected SW-REQ.
- **Owning requirements:** **SW-REQ-020**, **SW-REQ-021**, **SW-REQ-022**, **SW-REQ-027**, **SW-REQ-028**.

### FMEA-009 — Aggregate counter overflow for very high-volume orgs
- **Subsystem:** pump-mongo (aggregate) / analytics
- **Failure mode:** `analytics.Counter.Hits` is `int` (`analytics/aggregate.go:39`). On 64-bit hosts this is int64 (overflow unreachable in practice); on a 32-bit target it would wrap at ~2.1 B per window, breaking the monotonicity claimed by SW-REQ-011.
- **Cause:** Extreme volume per org/dimension within an unbounded window; or a 32-bit build.
- **Effect:** Counters wrap, dashboards show negative deltas, downstream aggregators may panic.
- **Severity:** **S** (low realistic likelihood on supported 64-bit targets; uncertainty noted).
- **Detection:** None.
- **Mitigation:** None. Recommend a non-negativity assertion or explicit `int64` typing for portability.
- **Owning requirements:** **SW-REQ-011**. Add `idempotency` obligation (re-aggregation under self-healing must not double-count).

### FMEA-010 — StatsD / DogStatsD sink unreachable
- **Subsystem:** observability / pump-statsd
- **Failure mode:** Metric emission fails (UDP error, listener gone).
- **Cause:** StatsD daemon down, misconfigured `statsd_connection_string`.
- **Effect:** Purge loop continues per SYS-REQ-017; emission errors do not propagate to records. Metrics gap is observability noise.
- **Severity:** **S**
- **Detection:** Logged on emit. The most likely operator-visible signal is alerts going quiet.
- **Mitigation:** SYS-REQ-017 `purge_loop_continues` is implemented correctly today.
- **Owning requirements:** **SYS-REQ-017**, **SW-REQ-005**, **SW-REQ-023**.

### FMEA-011 — TLS verification disabled in HTTP-log pumps (SSLInsecureSkipVerify)
- **Subsystem:** pump-http-log / pump-elasticsearch / pump-kafka / pump-hybrid
- **Failure mode:** Operator sets `ssl_insecure_skip_verify: true` (e.g. `pumps/elasticsearch.go:88`, `pumps/kafka.go:58`, `pumps/hybrid.go:102`). Records — potentially including raw request/response payloads — are forwarded over TLS without certificate validation.
- **Cause:** Misconfig, copy-paste from dev sample, expired-cert workaround.
- **Effect:** Analytics interceptable on the wire; PII exposure risk.
- **Severity:** **M** (operator-enabled, but silent — no warn at startup).
- **Detection:** None. `pumps/common.go:271,297` reference InsecureSkipVerify without logging a security warning.
- **Mitigation:** SW-REQ-027 marks TLS policy as "operator-configurable." Recommend emitting a WARN at init when the flag is true, plus a startup-time aggregate WARN listing all insecure pumps.
- **Owning requirements:** **SW-REQ-027**. Add `security_logged` + `tls_default_secure` obligations.

### FMEA-012 — Pump config meta map mis-typed for declared pump type
- **Subsystem:** config
- **Failure mode:** A pump's `meta` map carries fields valid for a different backend (e.g. `collection_name` under `type: sql`).
- **Cause:** Operator hand-edits, JSON merge mishap.
- **Effect:** INT-REQ-008 expects loud rejection. In practice mapstructure silently ignores unknown fields, so the pump initialises and the typo never surfaces.
- **Severity:** **M**
- **Detection:** None.
- **Mitigation:** None. Recommend `mapstructure.DecoderConfig{ErrorUnused: true}` on pump-meta decode.
- **Owning requirements:** **INT-REQ-008**, **SW-REQ-002**.

### FMEA-013 — Decode failure of a single record poisons batch
- **Subsystem:** purge-core / serializer
- **Failure mode:** A malformed record causes `serializerMethod.Decode` to error at `main.go:320`. The record is `continue`d, but the slice slot is left as a zero-value `interface{}` because `keys[i] = …` only runs on success.
- **Cause:** Corrupt msgpack/protobuf bytes from gateway, partial Redis write, codec mismatch.
- **Effect:** Downstream pumps may receive nil-interface holes; `filterData` type-asserts to `analytics.AnalyticsRecord` and would panic on nil for pumps that don't guard.
- **Severity:** **M**
- **Detection:** ERROR "Couldn't unmarshal analytics data" (`main.go:323`). Subsequent panic surfaces via Go runtime.
- **Mitigation:** Recommend compacting `keys` to drop nil entries before `writeToPumps`; add a regression test.
- **Owning requirements:** **SW-REQ-001**, **SW-REQ-008**, **INT-REQ-003**.

---

## Obligation re-tuning recommendations

| Req | Add obligation | Rationale |
|---|---|---|
| **SW-REQ-018** (mongo pump family) | `denial_of_service_resistant` | Self-healing's whole point is defending against pathological-input-driven doc-size blowups (FMEA-004). Making it explicit forces a verification check that self-healing actually engages. |
| **SW-REQ-011** (aggregate counter monotonicity) | `idempotency` | Self-healing recursion (FMEA-004) re-invokes `WriteData(ctx, data)` with the same data; nothing prevents the first partial write from being counted twice. |
| **SW-REQ-027** (HTTP-log pumps) | `security_logged` + `tls_default_secure` | FMEA-011: insecure-TLS flag is silent today. Pumps must default-verify and WARN at startup if an operator opts out. |
| **SYS-REQ-005** (per-pump timeout) | `ship_with_known_issue` carve-out | FMEA-001 / E1: mongo violates the contract; the audit must reflect this honestly. |
| **INT-REQ-004** (Pump interface contract) | new `context_honored` | FMEA-001 shows the contract is too narrow — it mandates errors-via-ctx but not that the implementation actually uses ctx. Verify each pump's `WriteData` passes caller ctx to its driver. |
| **INT-REQ-007** (SQL schema migration) | `expand_then_contract_documented` | FMEA-007: operator-process only today; a documented procedure should be the evidence. |
| **INT-REQ-008** (pump config loudness) | `strict_decode` | FMEA-012: silent ignoring of unknown meta fields breaks "reject loudly." |
| **INT-REQ-003** (serializer wire format) | `version_negotiation` (open) | FMEA-006: no handshake exists. Either add the obligation (and feature) or accept residual risk via a `proof risk` artifact. |
| **SW-REQ-001** (purge loop main flow) | `partial_batch_safe` | FMEA-013: decode failure leaves nil holes; obligation forces compaction or per-element guarding before dispatch. |
| **SW-REQ-006 / SW-REQ-007** (storage) | `environment_conditional` evidence note | FMEA-005: cluster-mode evidence is environment-conditional; document rather than imply universal verification. |
| **SW-REQ-020/021/022/027/028** (backend pumps) | `environment_conditional` evidence note | FMEA-008: same carve-out for backend integration evidence. |

A separate `proof risk` artifact is recommended for FMEA-006 (serializer version-negotiation absence) with owner = tyk-team and a 6-month review trigger.
