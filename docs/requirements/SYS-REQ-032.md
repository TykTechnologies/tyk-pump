# SYS-REQ-032: MaxMind GeoIP database is optional (environmental assumption)

## Intent
The MaxMind GeoIP database is operator-supplied and may not be present in any given deployment; GeoIP-lookup code shall short-circuit cleanly when the database handle is `nil` rather than panicking or producing partial geo data. This is an environmental assumption that satisfies parent **STK-REQ-001** by surfacing the GeoIP-DB-optionality contract and justifying the `//mcdc:ignore` annotations on the DB-bound code paths.

## Motivation
Many Tyk operators do not run MaxMind: they either do not need geo enrichment, are licensing-constrained, or rely on upstream CDN headers. Capturing the GeoIP DB as optional at the SYS layer matters for two reasons. First, it documents that "no geo data" is a supported steady state — not a degraded one to be alerted on. Second, it gives a principled justification for the `//mcdc:ignore` annotations on the lookup paths: those branches cannot be covered by unit tests without bundling a real MaxMind DB (which licensing and binary-size constraints forbid), so the coverage tool is told to skip them, and the trade-off is captured by this req rather than buried in a comment.

## Formalization
```
analytics_geo shall always satisfy geo_db_is_optional
```
This is an environmental input invariant: across every tyk-pump deployment, the GeoIP DB may or may not be present, and code that uses it must tolerate either case. There is no trigger — the assumption applies on every record `GetGeo` is called for. Truth condition is that operator deployment topology is permitted to omit the DB; the code accommodates the absent state by checking `GeoIPDB == nil` at the call site.

## Code references
- `analytics/analytics.go:372 GetGeo(ipStr string, GeoIPDB *maxminddb.Reader)` — the entry point that takes the DB handle as a parameter; first action is the `nil` short-circuit.
- `analytics/analytics.go:374 if GeoIPDB == nil { return }` — the explicit nil-handle short-circuit that realises the assumption.
- `analytics/analytics.go:379 //mcdc:ignore requires a real MaxMind GeoIP database to provoke; not available in unit tests` — the coverage-tool annotation the assumption justifies.
- `analytics/analytics.go:383 //mcdc:ignore depends on MaxMind DB Lookup outcome; not reachable without a real GeoIP DB` — second ignore annotation, same justification.
- `analytics/analytics.go:399 GeoIPLookup` — internal lookup that is only ever called once the nil-guard has passed.

## Evidence
- External-owner review: `assumption.external_owner: team:maxmind`, status `open`, reviewed via `proof req assumptions review` in Phase B (`verification.review.comment`: "external-owner reviewed (Redis protocol / MongoDB / Tyk gateway / MaxMind)"). Next review date: `2026-11-30`.
- `analytics/geoip_test.go:24 "nil GeoIPDB must not populate geo fields"` — the unit test that proves the short-circuit branch.
- Rationale: "MaxMind GeoIP DB is operator-supplied; optionality is reviewed against current usage patterns."

## Open questions
- The assumption is about *optionality*, not about *which DB* — different MaxMind product tiers (GeoLite2 City vs. GeoIP2 City vs. Enterprise) expose different schemas. The pump's `GeoData` struct maps to GeoLite2/GeoIP2 City; using a different schema is out of scope and not flagged.
- The `//mcdc:ignore` annotations are visible to coverage tooling but not cross-referenced from this assumption; a follow-up could insert `reqproof:assumes SYS-REQ-032` so the link is bidirectional.
- No KI linkage; the "DB present" path is tested in integration environments that bundle a sample MaxMind DB, but those are not part of the standard `make test` run.
