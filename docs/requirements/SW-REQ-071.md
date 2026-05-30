# SW-REQ-071: Analytics GeoIP enrichment short-circuits when DB is nil

## Intent
The analytics GeoIP enrichment functions `GetGeo` and `GeoIPLookup` shall return cleanly — with no panic and with empty/no enrichment — when the `*maxminddb.Reader` handle is `nil` or when the MaxMind DB returns no record for an IP. This is the SW-level design point that realises the SYS-REQ-034 guarantee that "GeoIP shall short-circuit when the DB is absent", which in turn derives from the STK-REQ-001 contract that tyk-pump runs in operator environments that may or may not include MaxMind.

## Motivation
Before this requirement existed, the nil-guard branches in `analytics/analytics.go` were attributed to SW-REQ-009 (an unrelated SW req about record field-name accessor determinism) and the SYS-level intent was buried inside a single SYS-REQ-032 that mixed assumption and guarantee. The SYS review surfaced the mixing as a decomposition smell; this SW req is the corresponding split on the software layer. It gives the GeoIP-nil-guard code a home of its own so that future GeoIP changes (e.g. switching from `maxminddb` to a different driver, or supporting GeoIP2 Enterprise schemas) have a single, narrowly-scoped requirement to update and re-verify.

## Formalization
```
analytics_geo shall always satisfy geo_lookup_short_circuits_when_db_absent
```
Always-pattern at the function boundary. For every call to `GetGeo(ipStr, GeoIPDB)`: if `GeoIPDB == nil`, the function returns without touching the record's `Geo` field. For every call to `GeoIPLookup(ipStr, GeoIPDB)`: if `ipStr == ""`, the function returns `(nil, nil)` before touching the DB. Both paths are panic-free.

## Code references
- `analytics/analytics.go:371-372 GetGeo` — annotated `// reqproof:implements SW-REQ-071`.
- `analytics/analytics.go:374 if GeoIPDB == nil { return }` — the nil-handle guard.
- `analytics/analytics.go:379 if err != nil //mcdc:ignore requires a real MaxMind GeoIP database to provoke` — error branch on DB-bound lookup.
- `analytics/analytics.go:383 if geo == nil //mcdc:ignore depends on MaxMind DB Lookup outcome` — no-record branch.
- `analytics/analytics.go:398-399 GeoIPLookup` — annotated `// reqproof:implements SW-REQ-071`.
- `analytics/analytics.go:400-402 if ipStr == "" { return nil, nil }` — empty-IP guard.

## Evidence
- `analytics/geoip_test.go:6 TestGeoIPLookup_Coverable` — verifies empty-IP and syntactically-invalid-IP both return cleanly before touching the DB.
- `analytics/geoip_test.go:20 TestGetGeo_NilDatabase` — verifies `GetGeo("1.2.3.4", nil)` does not populate any geo field and does not panic.
- Annotations on the test file (`// Verifies: SW-REQ-071`) autolinked into `traces.verified_by` by `proof trace autolink`.

## Open questions
- The DB-present positive path is not unit-tested (would require shipping a MaxMind DB); the `//mcdc:ignore` markers on the DB-bound branches inside `GetGeo` are justified by parent assumption SYS-REQ-032. No KI is needed because the contract is "short-circuit when absent", and that contract is exercised.
- `GeoIPLookup` for a syntactically-invalid IP returns `(nil, error)` — this is technically a "nominal" error path, not a "DB absent" path, but is verified in the same test for convenience. If a future split surfaces, it would belong to a new SW req about IP validation rather than this one.
