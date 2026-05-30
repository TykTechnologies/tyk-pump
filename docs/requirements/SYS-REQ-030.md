# SYS-REQ-030: Gateway publishes analytics under known list keys (environmental assumption)

## Intent
The Tyk gateway publishes serialized analytics records to the temporal store under the list keys `tyk-system-analytics` and `tyk-system-analytics_0` through `tyk-system-analytics_9` (when sharding is enabled), and uptime records under `tyk-uptime-analytics`. This is an environmental assumption that satisfies parent **STK-REQ-001** by surfacing the gateway-side contract tyk-pump consumes but does not control.

## Motivation
tyk-pump is downstream of the gateway: the list keys it polls are not a tyk-pump design choice but a gateway-side convention. If the gateway changed the key naming scheme — added more shards, renamed the canonical key, or moved uptime data to a different key — tyk-pump would silently stop draining records, and operators would see growing Redis memory with no error in the pump logs. Capturing the key convention as a SYS-layer assumption makes the trust boundary first-class so any gateway-side rename is recognised as a coordinated change requiring a paired tyk-pump update. It also documents the hard-coded `i < 10` shard ceiling in `StartPurgeLoop` as the SYS-level promise tyk-pump makes about how much sharding it can accept.

## Formalization
```
gateway_integration shall always satisfy gateway_publishes_keys
```
This is an environmental input invariant: whenever a Tyk gateway is producing analytics for this tyk-pump instance, the records appear on the named list keys and nowhere else. There is no trigger — the assumption must hold for the ingestion path to function at all. Truth condition is owned by the gateway's storage layer (`tyk` repo, analytics module).

## Code references
- `storage/store.go:18-19` — the constants `ANALYTICS_KEYNAME = "tyk-system-analytics"` and `UptimeAnalytics_KEYNAME = "tyk-uptime-analytics"`.
- `main.go:267-274` — the `for i := -1; i < 10; i++` loop that consumes the bare key and `_0..._9` shard variants; the `i < 10` ceiling encodes the maximum shard count tyk-pump will look for.
- `main.go:278 AnalyticsStore.GetAndDeleteSet(analyticsKeyName, chunkSize, expire)` — the actual read against each assumed key.
- `main.go:294 UptimeStorage.GetAndDeleteSet(storage.UptimeAnalytics_KEYNAME, ...)` — the uptime-side read.
- `main.go:270` comment: "if it's the first iteration, we look for tyk-system-analytics to maintain backwards compatibility or if analytics_config.enable_multiple_analytics_keys is disabled in the gateway" — documents the contract from the consumer side.

## Evidence
- External-owner review: `assumption.external_owner: team:tyk-gateway`, status `open`, reviewed via `proof req assumptions review` in Phase B (`verification.review.comment`: "external-owner reviewed (Redis protocol / MongoDB / Tyk gateway / MaxMind)"). Next review date: `2026-11-30`.
- Rationale field: "reviewed against current gateway source" — the assumption was verified by inspection of the gateway's publishing module at the time of approval.
- Companion req SYS-REQ-031 covers the encoding-by-suffix convention; together they describe the full gateway contract.

## Open questions
- The `i < 10` shard ceiling is an undocumented promise: if a future gateway release allowed more than 10 shards via `analytics_config.enable_multiple_analytics_keys`, tyk-pump would silently miss anything beyond `_9`. This is open across SYS-REQ-001 / SYS-REQ-030 and worth resolving with a gateway-side cross-reference.
- No KI linkage today; the gateway is an internal Tyk Technologies dependency so review cadence (`2026-11-30`) aligns with the gateway release calendar.
- The assumption does not address gateway versions older than the introduction of `enable_multiple_analytics_keys`; backwards compatibility is implicitly covered by the `i == -1` bare-key case in `main.go:269`.
