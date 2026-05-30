# SYS-REQ-033: Mongo capped-collection size is int64 (currently FAILED — KI mongo-test-panic-under-mongo6)

## Intent
The MongoDB Go driver shall return capped-collection size values typed as `int64`. This is an environmental assumption that satisfies parent **STK-REQ-002** by surfacing the type-shape contract the mongo pump's capped-collection test relies on. **This assumption is currently FAILED under mongo-server v6** (which returns `int`); the failure is tracked by known-issue `mongo-test-panic-under-mongo6`.

## Motivation
The mongo pump's capped-collection setup (`SW-REQ-018`) creates a capped collection when configured, and `TestMongoPump_capCollection_OverrideSize` asserts the resulting `maxSize` value via `colStats["maxSize"].(int64)`. Under mongo-server v6 the driver returns `int` instead of `int64`, and the unchecked type assertion panics — taking down the entire `pumps` package test run and erasing the evidence base for `SW-REQ-018`. Capturing the type-shape expectation as an explicit assumption (rather than burying it inside the test) makes the regression first-class: it becomes a tracked spec-level FAIL that the proof tooling can surface, not a hidden CI flake. It also justifies the current CI mitigation (pin the mongo server image to mongo:4 or mongo:5) as a deliberate spec-level workaround rather than as test-pinning by superstition.

## Formalization
```
pumps_mongo shall always satisfy capped_collection_size_is_int64
```
This is an environmental input invariant: the Mongo Go driver, when reporting capped-collection statistics, returns `maxSize` as a value typed `int64`. The assumption holds against the historical driver/server pairings the test was written for (driver v1.13 against server v4/v5); it fails against server v6. Truth condition is owned by the MongoDB Go driver type-conversion code path for `collStats`.

## Code references
- `pumps/mongo_test.go:316 if colStats["maxSize"].(int64) != int64(...)` — the type assertion that panics when the assumption is violated.
- `pumps/mongo_test.go:288 TestMongoPump_capCollection_OverrideSize` — the test that depends on the assumption.
- `pumps/mongo.go:274 capCollection` — the production code path that creates the capped collection; the assumption is about the *server response*, not about this code.
- `pumps/mongo.go:152 CollectionCapMaxSizeBytes int` — the configuration field that ultimately becomes the `maxBytes` value the server is asked to use.
- `pumps/mongo.go:313 m.store.Migrate(context.Background(), ..., model.DBM{"capped": true, "maxBytes": colCapMaxSizeBytes})` — the call that creates the capped collection.

## Evidence
- External-owner review: `assumption.external_owner: team:mongodb`, status `known_issue`, reviewed via `proof req assumptions review` in Phase B (`verification.review.comment`: "external-owner reviewed (Redis protocol / MongoDB / Tyk gateway / MaxMind), linked to KI for failed assumption A6"). Next review date: `2026-11-30`.
- Linked known-issue: `.proof/known-issues/mongo-test-panic-under-mongo6.yaml`:
  - id: `mongo-test-panic-under-mongo6`
  - status: `open`, severity `medium`, release_disposition `ship`
  - reproducer: `MONGO_DRIVER=mongo-go go test -run TestMongoPump_capCollection_OverrideSize -count=1 ./pumps`
  - mitigation: pin CI mongo image to mongo:4 or mongo:5
  - remediation: coerce the type assertion to accept both `int` and `int64`
- Phase B comment: "the latter is a currently-failed assumption tracked by Phase E known-issue mongo-test-panic-under-mongo6."

## Open questions
- This is the FAILED assumption (Assumption A6 in the spec). The remediation in the linked KI (`switch on colStats["maxSize"].(type)`) would resolve the failure by making the test tolerant of both int and int64 — but the *assumption itself* would still be technically false against mongo:6. A cleaner long-term resolution is to either weaken this req to "capped-collection size is numeric and fits in int64" or to bump the mongo-driver dependency to a version that normalises the type.
- The KI mitigation (pin mongo:4/5 in CI) preserves test-suite stability but kicks the can down the road for operators running mongo:6 in production who try to reproduce the test locally.
- External-vendor confirmation status: open with `team:mongodb`; the next review on `2026-11-30` should incorporate any clarification from the driver maintainers about the intended `collStats` return type across server versions.
- Cascading impact: because the panic crashes the whole `pumps` package test run, it suppresses evidence for `SW-REQ-018` (mongo pump capped collection) — so this assumption's failure has knock-on effects on SW-layer coverage reporting, not just on this one test.
