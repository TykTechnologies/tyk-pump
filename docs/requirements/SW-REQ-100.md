# SW-REQ-100: Elasticsearch alias projection is preserved

Documents: SW-REQ-100

## Contract

When `getMapping` projects an `analytics.AnalyticsRecord` into the
Elasticsearch document map, a populated `AnalyticsRecord.Alias` must be emitted
as the `"alias"` field with the exact same value. Empty aliases must remain
empty and must not be replaced by another identity field.

This is a child of SW-REQ-068. SW-REQ-068 owns the broader Elasticsearch
operator and mapping helper surface; SW-REQ-100 pins the historical field-loss
case fixed by commit `3bb755d`.

## Evidence

- `pumps/elasticsearch_test.go:TestGetMapping_BasicFields` asserts a populated
  `Alias` value is present in the Elasticsearch mapping as `"alias"`.
- `pumps/elasticsearch_test.go:TestGetMapping_AliasProjection_EmptyAlias`
  asserts an empty source alias maps to an empty `"alias"` value, preventing a
  stale or synthetic user identifier from satisfying the projection contract.

## Known Issues

This requirement covers only the in-process `getMapping` projection. Live
Elasticsearch write failure visibility, recursive reconnect behavior, TLS/API
key transport composition, and malformed `decode_base64` behavior remain
tracked by their existing KnownIssue records.
