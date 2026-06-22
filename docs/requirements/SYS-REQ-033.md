# SYS-REQ-033: Mongo capped-collection size is int64

## Intent
The MongoDB Go driver shall return capped-collection size values typed as `int64`. This is an environmental assumption that satisfies parent **STK-REQ-002** by surfacing the type-shape contract the mongo pump's capped-collection test relies on. This assumption is currently false under mongo-server v6, which can return `int`; it is retained as an external assumption rather than a tyk-pump product KnownIssue.

## Motivation
The mongo pump's capped-collection setup (`SW-REQ-018`) creates a capped collection when configured, and `TestMongoPump_capCollection_OverrideSize` historically asserted the resulting `maxSize` value via `colStats["maxSize"].(int64)`. Under mongo-server v6 the driver can return `int` instead of `int64`. Capturing the type-shape expectation as an explicit assumption keeps the external dependency contract visible without counting it as a product KnownIssue.

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
- External-owner review: `assumption.external_owner: team:mongodb`, status `open`. Next review date: `2026-11-30`.
- The former environment/test-harness KnownIssue was removed from the product KnownIssue backlog; this remains tracked as an external assumption.

## Open questions
- This is an open external assumption. A cleaner long-term resolution is to either weaken this req to "capped-collection size is numeric and fits in int64" or to bump the mongo-driver dependency to a version that normalises the type.
- External-vendor confirmation status: open with `team:mongodb`; the next review on `2026-11-30` should incorporate any clarification from the driver maintainers about the intended `collStats` return type across server versions.
- Cascading impact: because the panic crashes the whole `pumps` package test run, it suppresses evidence for `SW-REQ-018` (mongo pump capped collection) — so this assumption's failure has knock-on effects on SW-layer coverage reporting, not just on this one test.
