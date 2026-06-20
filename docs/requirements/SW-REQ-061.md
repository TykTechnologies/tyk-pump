# SW-REQ-061: Mongo aggregate — tag-list bounding alert

## Parent
This requirement is a per-significant-behaviour decomposition of
SW-REQ-036 (mongo-aggregate). It carries the tag-list-bound alerting
obligation in isolation.

## Intent
When the count of tags on an aggregated document exceeds
`ThresholdLenTagList` (default 1000), the MongoAggregate pump shall emit a
`Warn`-level alert listing up to `CommonTagsCount` (5) common tag prefixes
computed by `getListOfCommonPrefix`, instructing the operator to suppress
noisy tags via `ignore_tag_prefix_list`. A configured value of `-1` shall
disable the alert entirely. Derived from SYS-REQ-010 (record bounded size)
— this is an operator-warning bound rather than a hard truncation.

## Motivation
Tags are operator-supplied labels (often per-API or per-endpoint) attached
to each analytics record. If an operator inadvertently emits high-
cardinality tags (e.g. one tag per unique URL), the aggregate document
balloons and may exceed Mongo's 16 MiB per-document limit. The alert
exists to give operators an actionable warning before the document-size
self-heal (SW-REQ-062) kicks in. The hard ceiling lives in
`ThresholdLenTagList`; the warning text identifies the noisy prefixes via
the `getListOfCommonPrefix` helper.

## Code references
- `pumps/mongo_aggregate.go:MongoAggregatePump.printAlert` — emits the
  warning.
- `pumps/mongo_aggregate.go:getListOfCommonPrefix` — common-prefix
  extraction.
- `pumps/mongo_aggregate.go:DoAggregatedWriting` — checks
  `ThresholdLenTagList` before each upsert.

## Evidence
- The `denial_of_service_resistant` obligation's `fuzz` evidence remains
  tracked as open KI-backed debt: a fuzz harness exercising the printAlert /
  getListOfCommonPrefix path with adversarial tag distributions does not
  exist today; the fact-based ceiling enforced by `ThresholdLenTagList` is
  the current mitigation.

## Open questions
- The check fires *after* the aggregate document is built; pathological
  tag explosions can still produce a too-large document for the upsert
  step (where SW-REQ-062 self-heal kicks in).
- Future fuzz harness should exercise `getListOfCommonPrefix` with random
  byte sequences to verify O(n log n) bounded complexity.
