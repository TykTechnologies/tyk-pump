# SW-REQ-036: MongoDB aggregate pump — parent requirement for per-org aggregate documents

## Intent
The `MongoAggregatePump` shall aggregate non-MCP analytics records into
per-organisation aggregate documents (keyed by `{orgid, timestamp}`) and
upsert them into MongoDB. This parent requirement anchors the six
per-significant-behavior sub-requirements that carry the substantive
obligations: SW-REQ-058 (aggregation window), SW-REQ-059 (per-org collection
sharding + mixed-collection), SW-REQ-060 (`$inc` upsert semantics),
SW-REQ-061 (tag-list bound alert), SW-REQ-062 (max-doc-size self-healing),
and SW-REQ-063 (index-ensure lifecycle). Derived from SYS-REQ-003
(aggregation windowing) via Phase A decomposition of SW-REQ-018.

## Motivation
The aggregate pump is the most behaviorally rich of the six MongoDB writers
and the only one with self-healing logic (it halves `AggregationTime` and
retries on a document-size error). Bundling all of its substantive behaviours
under a single SW req hid the windowing/sharding/$inc/self-heal/index-ensure
distinctions. The decomposition exists to anchor each sub-behaviour to its
own atomic obligation; this parent carries `nominal` because the sub-reqs
carry the real obligations.

## Code references
- `pumps/mongo_aggregate.go:MongoAggregatePump.WriteData` — orchestrator.
- `pumps/mongo_aggregate.go:DoAggregatedWriting` — per-org upsert pipeline.
- Sub-behaviour entry points are listed in each sub-req's "Code references"
  section.

## Evidence
- `pumps/mongo_aggregate_test.go` (re-annotated to point at the relevant
  sub-req per test function):
  - `TestAggregationTime` / `TestMongoAggregatePump_StoreAnalyticsPerMinute` →
    SW-REQ-058.
  - `TestDoAggregatedWritingWithIgnoredAggregations` → SW-REQ-059.
  - `TestMongoAggregatePump_SelfHealing` /
    `TestMongoAggregatePump_ShouldSelfHeal` /
    `TestMongoAggregatePump_divideAggregationTime` → SW-REQ-062.
  - `TestDecodeRequestAndDecodeResponseMongoAggregate`,
    `TestDefaultDriverAggregate`, `TestMongoAggregatePump_SkipsMCPRecords`,
    and the `dummyObject` helpers remain annotated against this parent.
- Live-MongoDB tests are excluded from the local audit MC/DC scope (known
  issue).

## Open questions
- Self-heal recursion has no depth bound beyond the
  `AggregationTime > 1` guard (see SW-REQ-062).
- The pump skips MCP records via `IsMCPRecord()`; MCP aggregation is the
  responsibility of `MCPMongoAggregatePump` (SW-REQ-039).
- The previous family req SW-REQ-018 is retained as a `[SUPERSEDED by Phase A
  decomposition: ...]` anchor.
