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
`ApiKeyTransport`, and optional custom TLS construction) and corresponding
`BulkProcessor`. Versions other than 3/5/6/7 shall be rejected at Init
via `log.Fatal`. When `ExtendedStatistics` and `decode_base64` are enabled,
`getMapping` stores `raw_request` and `raw_response` as decoded plaintext
strings, not `[]byte` values that JSON would encode as base64 again. Derived
from SYS-REQ-004.

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
- `pumps/elasticsearch.go:getMapping` — per-record document mapping, including
  extended raw payload mapping and `decode_base64` conversion.
- `docs/requirements/SW-REQ-100.md` — child requirement that pins the
  historical `AnalyticsRecord.Alias` to Elasticsearch `"alias"` field
  projection.

## Evidence
- `pumps/elasticsearch_test.go:TestGetMapping_*` (re-annotated `Verifies:
  SW-REQ-068`) — exercise the per-record mapping path used by every
  version operator.
- `pumps/elasticsearch_test.go:TestGetMapping_ExtendedStatistics` covers the
  TN-6 decoded-payload text contract: decoded `raw_request` / `raw_response`
  fields are plaintext strings, not byte slices that JSON would re-encode.
- `pumps/elasticsearch_test.go:TestGetMapping_BasicFields` and
  `TestGetMapping_AliasProjection_EmptyAlias` cover SW-REQ-100 alias
  projection.
- `pumps/elasticsearch_test.go:TestElasticsearchPump_TLSConfig_ErrorCases`
  (re-annotated `Verifies: SW-REQ-068`) — exercises TLS-config error
  propagation from `getOperator`.
- `pumps/elasticsearch_test.go:TestElasticsearchPump_TLSConfig_EnvSkipVerify`
  — verifies the Elasticsearch environment spelling
  `SSLINSECURESKIPVERIFY` reaches `SSLInsecureSkipVerify` and is passed into
  the TLS helper as an explicit operator opt-in that emits the shared warning
  for skipped certificate verification.
- `pumps/elasticsearch_test.go:TestElasticsearchPump_GetOperatorPassesTLSConfigFields`
  — verifies `getOperator` hands `ssl_cert_file`, `ssl_key_file`,
  `ssl_ca_file`, and `ssl_insecure_skip_verify` through to the shared
  `NewTLSConfig` helper.
- `pumps/elasticsearch_mcdc_100_test.go:TestElasticsearchPump_getOperator_UseSSL_TLSSuccess`
  — verifies the TLS construction path succeeds with a configured CA root and
  strict `InsecureSkipVerify=false`. This is configuration-construction
  evidence; the in-scope Elasticsearch testcontainer URL is still HTTP.
- `pumps/tls_verification_explicit_test.go:TestTLSVerificationExplicit_DefaultsAreSecure`
  — verifies Elasticsearch `SSLInsecureSkipVerify` defaults to false so
  insecure TLS requires explicit operator opt-in; strict production
  certificate validation remains tracked by KI `tls-insecure-skip-verify-allowed`.

## Open questions
- ES versions 8+ are not supported; the `log.Fatal` will fire if an
  operator sets `Version: "8"`. Worth a Phase-B follow-up to add v8
  support.
- `connect()` recurses without bound on connection failure (`elasticsearch.go:412-415`)
  — tracked as a separate known issue.
- API-key auth is currently overwritten by the `UseSSL` transport assignment
  when both are configured — tracked as KI
  `elasticsearch-api-key-auth-dropped-when-use-ssl`.
