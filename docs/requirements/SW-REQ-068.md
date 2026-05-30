# SW-REQ-068: Elasticsearch — per-version bulk processor dispatch

## Parent
This requirement is a per-significant-behaviour decomposition of the
previous family req SW-REQ-020 (elasticsearch). It carries the per-version
operator-selection obligation in isolation.

## Intent
The Elasticsearch pump shall, at Init, select an `ElasticsearchOperator`
implementation per configured `Version` (`"3"`, `"5"`, `"6"`, or `"7"`,
defaulting to `"3"` when unset) — instantiating the corresponding
`elastic.v3` / `elastic.v5` / `elastic.v6` / `elastic/v7` client (with
sniffing toggle, basic auth, optional `ApiKey-auth` via
`ApiKeyTransport`, and optional custom TLS) and corresponding
`BulkProcessor`. Versions other than 3/5/6/7 shall be rejected at Init
via `log.Fatal`. Derived from SYS-REQ-004.

## Motivation
Elasticsearch has had multiple breaking client-library splits across the
3/5/6/7 versions, and the wire-level APIs differ enough that each version
needs its own client library and `BulkProcessor`. The
`ElasticsearchOperator` interface abstracts the per-version differences
behind a small surface so the rest of the pump can be version-agnostic.
The `log.Fatal` on unknown versions is harsh but matches the operator
expectation that Init failure aborts startup.

## Code references
- `pumps/elasticsearch.go:ElasticsearchPump.getOperator` — version
  dispatch.
- `pumps/elasticsearch.go:ElasticsearchPump.connect` — instantiates the
  per-version client.
- `pumps/elasticsearch.go:Elasticsearch{3,5,6,7}Operator` — per-version
  client wrappers.
- `pumps/elasticsearch.go:ApiKeyTransport` — optional API-key header
  injection.

## Evidence
- `pumps/elasticsearch_test.go:TestGetMapping_*` (re-annotated `Verifies:
  SW-REQ-068`) — exercise the per-record mapping path used by every
  version operator.
- `pumps/elasticsearch_test.go:TestElasticsearchPump_TLSConfig_ErrorCases`
  (re-annotated `Verifies: SW-REQ-068`) — exercises TLS-config error
  propagation from `getOperator`.

## Open questions
- ES versions 8+ are not supported; the `log.Fatal` will fire if an
  operator sets `Version: "8"`. Worth a Phase-B follow-up to add v8
  support.
- `connect()` recurses without bound on connection failure (`elasticsearch.go:412-415`)
  — tracked as a separate known issue.
