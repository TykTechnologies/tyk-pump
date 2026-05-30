# SW-REQ-020: Elasticsearch pump — bulk-API writes with rolling indices, multi-version

## Intent
The Elasticsearch pump shall write analytics records using the Elasticsearch
bulk API (via per-version `BulkProcessor` helpers) to rolling daily indices
when `RollingIndex` is enabled (`<index>-YYYY.MM.DD`). The pump shall support
ES versions 3, 5, 6, and 7 through per-version client libraries selected at
`Init` time. Derived from SYS-REQ-004 (independent per-backend delivery).

## Motivation
Elasticsearch's wire and library API changed materially between major versions
(3 → 5 → 6 → 7), and forcing operators onto a single version would exclude
sites still running older clusters or pinned by a SaaS provider. The pump
therefore embeds four distinct client libraries (`elasticv3` through
`elasticv7`) and four corresponding operator types
(`Elasticsearch3Operator` through `...7Operator`), each producing its own
`BulkProcessor` configured with the same worker/flush/size knobs.

The bulk API is essential at production volumes: per-record HTTP requests
would saturate both the pump and the cluster. The `DisableBulk` knob exists
for debugging only. Rolling indices let operators apply size-bounded
retention via index lifecycle management without touching the pump.

Failure modes addressed:
- Lost data on transient ES outage — `connect()` retries with a 5-second
  sleep on initial connection failure.
- Authentication interoperability — both `ApiKey` (via `ApiKeyTransport`) and
  Basic auth are wired up.
- Mixed analytics/MCP traffic — `MCPIndexName` lets MCP records land in a
  separate index when set (`getIndexNameForRecord`).
- TLS — when `UseSSL` is set the shared `NewTLSConfig` helper is invoked
  with cert/key/CA/skip-verify operator options.

## Code references
- `pumps/elasticsearch.go:24-28` — `ElasticsearchPump` struct embedding
  `CommonPumpConfig`.
- `pumps/elasticsearch.go:117-139` — four `Elasticsearch{3,5,6,7}Operator`
  structs.
- `pumps/elasticsearch.go:158-339` — `getOperator()` dispatches on
  `conf.Version` and builds the bulk processor for each ES major.
- `pumps/elasticsearch.go:171-182` — `UseSSL` path delegates to
  `NewTLSConfig`.
- `pumps/elasticsearch.go:419-432` — `WriteData` either reconnects or calls
  `e.operator.processData`.
- `pumps/elasticsearch.go:435-460` — `getIndexName` / `getIndexNameForRecord`
  implement `RollingIndex` (`-YYYY.MM.DD` suffix) and per-MCP index routing.
- `pumps/elasticsearch.go:686-692` — `Shutdown` flushes the bulk processor.

## Evidence
- `pumps/elasticsearch_test.go` covers index-name and mapping logic.
- End-to-end tests need a running Elasticsearch cluster and are excluded from
  the local audit MC/DC scope (recorded as a known issue).

## Open questions
- The four `Elasticsearch{N}Operator.processData` implementations are
  near-duplicates (different library types). The requirement says nothing
  about which versions are "supported" vs deprecated; ES 3 and 5 are end-of-life
  upstream and arguably should be removed.
- `WriteData`'s reconnect path (`elasticsearch.go:422-425`) calls
  `e.WriteData(ctx, data)` recursively without bound, but does not return its
  result — a transient outage could double-process records.
- `getOperator` calls `e.log.Fatal` on unknown version after the switch
  default; `connect()` recurses on failure with no max retry count.
- The pump uses `http.DefaultTransport` when `UseSSL` is false and
  ApiKey auth is configured — meaning TLS for HTTPS endpoints would use Go's
  defaults, not the operator-supplied CA bundle. This nuance is not captured
  in the requirement.
