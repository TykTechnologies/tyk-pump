# SW-REQ-069: Elasticsearch — rolling and MCP bulk index naming policy

## Parent
This requirement is a per-significant-behaviour decomposition of the
previous family req SW-REQ-020 (elasticsearch). It carries the rolling-
index naming obligation in isolation.

## Intent
When `RollingIndex` is true, the Elasticsearch pump shall append
`-YYYY.MM.DD` (UTC system time) to the target index name. The bulk write
routing path shall use `getIndexNameForRecord`: when `MCPIndexName` is
non-empty and the record is an MCP record, the MCP-specific index name is
used (with the same rolling suffix when enabled); otherwise the standard
`IndexName` is used. Derived from SYS-REQ-004.

## Motivation
Rolling daily indexes are the standard ES idiom for time-series data:
each day gets its own index, which gives operators cheap per-day
retention (drop the index), simpler shard management, and faster
queries when the operator can specify a date range. The MCP-specific
routing exists so MCP records land in a dedicated index that can be
queried independently of the main analytics stream.

## Code references
- `pumps/elasticsearch.go:getIndexName` — appends the rolling suffix.
- `pumps/elasticsearch.go:getIndexNameForRecord` — MCP-aware per-record
  routing with optional rolling suffix.

## Evidence
- `pumps/elasticsearch_test.go:TestGetIndexName_NoRolling` /
  `TestGetIndexName_Rolling` (re-annotated `Verifies: SW-REQ-069`) —
  exercise the rolling-suffix on/off matrix.
- `pumps/elasticsearch_test.go:TestGetIndexNameForRecord_*`
  (re-annotated `Verifies: SW-REQ-069`) — exercise the MCP-aware routing
  with `MCPIndexName` set vs unset and with `RollingIndex` on vs off.

## Open questions
- The rolling suffix uses UTC; operators in non-UTC zones may see index
  rollovers at a non-local-midnight time.
- The MCP index is *additional* — non-MCP records still go to the
  standard index; the routing is a per-record decision rather than a
  pump-wide one.
- The non-bulk MCP routing path is currently tracked by KnownIssue
  `elasticsearch-mcp-routing-non-bulk-ignored`; the positive SW-REQ-069
  evidence is the `getIndexNameForRecord` / bulk-routing contract.
