# SYS-REQ-034: GeoIP lookup short-circuits when MaxMind DB is absent

## Intent
GeoIP enrichment shall return safely (no panic, no partial geo data) when the operator-supplied MaxMind database is not present at runtime. This is the **derived guarantee** half of what used to live inside SYS-REQ-032 — the assumption ("the DB may be absent") is captured by SYS-REQ-032; this requirement captures the obligation it places on the system ("when the DB is absent, behaviour is well-defined").

## Motivation
The SYS review of the spec flagged that SYS-REQ-032 mixed two distinct concerns: an environmental assumption about MaxMind availability AND a system-behaviour guarantee about how the absence is handled. Mixing assumption and guarantee in a single requirement breaks decomposition cleanliness (an assumption is something the system relies on; a guarantee is something the system delivers) and obscures the SW-level implementation point. Splitting them into SYS-REQ-032 (assumption) + SYS-REQ-034 (derived guarantee) makes both halves traceable: SYS-REQ-032 is reviewed by the external-owner workflow, SYS-REQ-034 is satisfied by a concrete SW design point (SW-REQ-071) which is in turn implemented in `analytics/analytics.go` and verified in `analytics/geoip_test.go`.

## Formalization
```
analytics_geo shall always satisfy geo_lookup_short_circuits_when_db_absent
```
Always-pattern: across every call to `GetGeo`, regardless of operator deployment, when the GeoIP handle is `nil` (or returns nothing for the queried IP), the function returns cleanly and the record's geo fields remain empty. The truth condition is observable at the boundary: after `GetGeo` returns, no panic occurred and `a.Geo` is the zero value when the DB was absent.

## Code references
- `analytics/analytics.go:372 GetGeo(ipStr string, GeoIPDB *maxminddb.Reader)` — entry point.
- `analytics/analytics.go:374 if GeoIPDB == nil { return }` — the short-circuit that realises the guarantee.
- `analytics/analytics.go:399 GeoIPLookup` — only entered after the nil-guard passes.
- `analytics/geoip_test.go:20 TestGetGeo_NilDatabase` — the verifying test, asserts no panic + empty geo fields when DB is nil.
- `analytics/geoip_test.go:6 TestGeoIPLookup_Coverable` — verifies `GeoIPLookup("", nil)` and `GeoIPLookup("not-an-ip", nil)` short-circuit before touching the DB.

## Evidence
- `analytics/geoip_test.go` (`negative` evidence: drives the DB-absent path).
- Implements: `analytics/analytics.go:GetGeo`, `analytics/analytics.go:GeoIPLookup` (autolinked from `// reqproof:implements SW-REQ-071` annotations on the SW child).
- Satisfies: `STK-REQ-001` (parent), with the linkage tightened through child `SW-REQ-071` so the trace from STK through SYS-034 to the code is complete.

## Open questions
- The "DB-present" path (a real MaxMind DB returning a record) is verified only in environments that bundle a sample DB and is excluded from the standard `make test` run; the `//mcdc:ignore` annotations on those branches are justified by the parent assumption SYS-REQ-032.
- An optional future enhancement is to surface a one-time log warning when `GeoIPDB == nil` so operators can confirm they intentionally omitted the DB — currently the absence is silent.
