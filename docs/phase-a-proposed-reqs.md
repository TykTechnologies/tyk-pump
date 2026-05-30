# Phase A — Proposed SW requirement decomposition

This report decomposes the family-level SW reqs (SW-018, SW-019, SW-020, SW-022,
SW-027, SW-028) into per-implementation atomic SW reqs, and further splits the
two most complex implementations (mongo-aggregate, sql-aggregate) and
elasticsearch into per-significant-behavior sub-reqs.

All descriptions are code-verified against the current `pumps/` files. Where a
clean obligation match is uncertain, candidates and the top pick are listed.
Obligation IDs are from the bundled reqforge catalog (`pkg/catalog/data/domain/`
plus the legacy classes catalog: `nominal`, `error_handling`, `errors_propagated`,
`parameterized_only_write`, `cert_validation_strict`, `idempotency`,
`denial_of_service_resistant`, `untrusted_input_bounded`, `atomicity`,
`boundary`, `monotonicity`, `determinism`, `encoding_safety`,
`malformed_input`, `binary_data_preserved`, `connection_leak_free`,
`request_timeout_bounded`, `external_call_timeout_bounded`,
`external_call_failure_observable`).

Total new SW reqs proposed: **30**.

---

## Pump family per-implementation splits

### 1. mongo-standard — pumps/mongo.go
- Component: `pumps-mongo-standard`
- Parent SYS-REQ: SYS-REQ-004 (independent per-backend delivery)
- Primary obligation_class: `errors_propagated`
- Additional obligation_checklist: [`errors_propagated`, `boundary`]
- Description: "The standard MongoDB pump shall, on each purge, filter out
  MCP-classified analytics records, accumulate the remaining records into
  batches bounded by `MaxInsertBatchSizeBytes` (default 10 MiB) while
  rewriting raw bodies of documents whose serialized size exceeds
  `MaxDocumentSizeBytes`, then concurrently insert each batch into the single
  configured collection, returning the first per-batch insert error to the
  caller; when `CollectionCapEnable` is true on a 64-bit host with a
  non-existing collection, it shall create the collection as a capped
  collection of `CollectionCapMaxSizeBytes` (default 5 GiB)."
- Files: pumps/mongo.go
- Key symbols: `MongoPump.Init`, `MongoPump.WriteData`, `MongoPump.AccumulateSet`,
  `MongoPump.capCollection`, `MongoPump.ensureIndexes`, `handleLargeDocuments`,
  `shouldProcessItem`, `getItemSizeBytes`, `accumulate`, `getMongoDriverType`
- Notes: `WriteData` ignores the caller `ctx` (uses `context.Background()` at
  pumps/mongo.go:435) — flag against known-issue
  `mongo-pump-ignores-caller-context` and SYS-REQ-005. Same context-loss is
  present in `WriteUptimeData` (pumps/mongo.go:589). The req description
  honestly says "concurrently insert" but should not claim ctx-cancellation.

### 2. mongo-selective — pumps/mongo_selective.go
- Component: `pumps-mongo-selective`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `errors_propagated`
- Additional obligation_checklist: [`errors_propagated`, `boundary`]
- Description: "The MongoSelective pump shall partition incoming non-MCP
  analytics records by `OrgID` and route each org's records into a
  per-organisation collection named `z_tyk_analyticz_<orgid>`, dropping records
  with empty OrgID; for each org collection it shall ensure baseline indexes
  (`apiid`, TTL on `expireAt` (skipped for CosmosDB), and a composite
  `logBrowserIndex`) and then insert records in size-bounded batches up to
  `MaxInsertBatchSizeBytes` (default 10 MiB), skipping individual documents
  larger than `MaxDocumentSizeBytes`."
- Files: pumps/mongo_selective.go
- Key symbols: `MongoSelectivePump.WriteData`, `GetCollectionName`,
  `AccumulateSet`, `ensureIndexes`, `processItem`, `accumulate`
- Notes: Per-batch insert errors are logged but **not returned** (line 244 in
  pumps/mongo_selective.go) — the function always returns `nil`. The req
  description must reflect this honestly (i.e., the selective pump does *not*
  propagate insert errors). Recommend `obligation_class: nominal` if
  `errors_propagated` is interpreted strictly; otherwise keep
  `errors_propagated` with an explicit note about the per-batch swallowing.
  Also uses `context.Background()` (line 242), same known-issue scope.

### 3. mongo-aggregate — pumps/mongo_aggregate.go
- Component: `pumps-mongo-aggregate`
- Parent SYS-REQ: SYS-REQ-003 (aggregation windowing) — this file is fundamentally
  an aggregation implementation; SYS-REQ-004 is also satisfied transitively.
- Primary obligation_class: `nominal` (this is a coarse parent req; the
  substantive behavior obligations live on the per-behavior sub-reqs)
- Additional obligation_checklist: [`nominal`]
- Description: "The MongoAggregate pump shall aggregate non-MCP analytics
  records into per-organisation aggregate documents and upsert them into
  MongoDB; the substantive behaviors (windowing, sharding, $inc semantics,
  tag bounding, self-healing, index ensure) are decomposed into the six
  mongo-aggregate sub-reqs below."
- Files: pumps/mongo_aggregate.go
- Key symbols: `MongoAggregatePump.WriteData`, `DoAggregatedWriting`
- Notes: This req exists only to anchor the six sub-reqs that follow. Each
  sub-req carries its own obligation and description.

### 4. graph-mongo — pumps/graph_mongo.go
- Component: `pumps-mongo-graph`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `errors_propagated`
- Additional obligation_checklist: [`errors_propagated`]
- Description: "The Graph MongoDB pump shall, on each purge, accumulate
  incoming records into size-bounded batches via the embedded
  MongoPump.AccumulateSet (in graph-only mode skipping records whose
  `GraphQLStats.IsGraphQL` is false from conversion), convert each surviving
  `AnalyticsRecord` to a `GraphRecord` (assigning a fresh ObjectID), and
  concurrently insert each batch into the single configured graph collection;
  the first per-batch insert error shall be returned to the caller, and
  connection-closed conditions shall be logged with a 'Detected connection
  failure!' warning."
- Files: pumps/graph_mongo.go
- Key symbols: `GraphMongoPump.Init`, `GraphMongoPump.WriteData` (uses
  embedded `MongoPump.AccumulateSet(data, true)`)
- Notes: Records with `IsGraphQL == false` are *still inserted* (as bare
  AnalyticsRecord wrapped in a GraphRecord) after a warning — verified at
  graph_mongo.go:129-138. Same `context.Background()` issue as mongo-standard.

### 5. mcp-mongo — pumps/mcp_mongo.go
- Component: `pumps-mongo-mcp`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `errors_propagated`
- Additional obligation_checklist: [`errors_propagated`]
- Description: "The MCP MongoDB pump shall, on each purge, retain only
  `AnalyticsRecord`s for which `IsMCPRecord()` is true, accumulate them into
  size-bounded batches via the embedded `MongoPump.AccumulateSet`, convert
  each batch to `MCPRecord` instances with fresh ObjectIDs, and concurrently
  insert each batch into the single configured MCP collection; the first
  per-batch insert error shall be returned to the caller, and 'closed
  explicitly' errors shall be reported as a connection failure warning."
- Files: pumps/mcp_mongo.go
- Key symbols: `MCPMongoPump.WriteData`, `filterMCPData`, `convertToMCPObjects`,
  `insertMCPDataSet`
- Notes: Same `context.Background()` issue as mongo-standard. Returns
  `fmt.Errorf("no collection name")` if collection is unset (does not Fatal
  unlike the standard pump).

### 6. mcp-mongo-aggregate — pumps/mcp_mongo_aggregate.go
- Component: `pumps-mongo-mcp-aggregate`
- Parent SYS-REQ: SYS-REQ-003
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`errors_propagated`, `atomicity`]
- Description: "The MCP MongoDB Aggregate pump shall aggregate
  MCP-classified analytics records per API via `analytics.AggregateMCPData`,
  then for each API perform a two-step MongoDB upsert keyed by
  `{orgid, timestamp, owner_apiid}`: first applying $inc/$set/$max counters
  for both standard dimensions (via `AsChange`) and MCP-specific dimensions
  (`methods`, `primitives`, `names` via `addMCPDimensionUpdates`), then
  recalculating averages via `AsTimeUpdate`; the first upsert error per API
  shall be returned to the caller. When `UseMixedCollection` is true, both
  the per-org and mixed-collection variants shall be written."
- Files: pumps/mcp_mongo_aggregate.go
- Key symbols: `MCPMongoAggregatePump.WriteData`, `DoMCPAggregatedWriting`,
  `upsertMCPAggregate`, `addMCPDimensionUpdates`
- Notes: Inherits the `AggregationTime` / mixed-collection plumbing from the
  embedded `MongoAggregatePump`. Does **not** implement self-healing — only
  the standard mongo-aggregate has `ShouldSelfHeal`. Reuses
  `MongoAggregatePump.ensureIndexes`, so the same index-ensure
  behavior applies.

---

### 7. sql-standard — pumps/sql.go
- Component: `pumps-sql-standard`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `parameterized_only_write`
- Additional obligation_checklist: [`parameterized_only_write`,
  `connection_leak_free`]
- Description: "The standard SQL pump shall, on each purge, drop
  MCP-classified analytics records, then if `TableSharding` is true split the
  remaining records on each `YYYYMMDD` timestamp boundary and route each
  day-slice to a `tyk_analytics_<YYYYMMDD>` table (auto-created and indexed
  via `ensureTable`/`ensureIndex`), otherwise route all records to the
  default `tyk_analytics` table; within each table it shall issue parameter-
  bound batched inserts of `BatchSize` records (default 1000) using
  `gorm.Create` on the request context. Postgres uses a custom
  `monthEncodePlan` AfterConnect to encode `time.Month` as `int8` (TT-16980).
  Writes log per-batch errors but do **not** propagate them upward."
- Files: pumps/sql.go
- Key symbols: `SQLPump.WriteData`, `SQLPump.Init`, `Dialect`, `monthEncodePlan`,
  `SQLPump.ensureTable`, `SQLPump.ensureIndex`, `SQLPump.createIndex`,
  `WriteUptimeData`
- Notes: `WriteData` always returns `nil` even on per-batch errors (line
  327-328 just logs). The honest description above reflects this. Day-bucket
  algorithm reused in sql_aggregate / graph_sql / mcp_sql is duplicated;
  abstracting is out-of-scope for Phase A. Postgres-only `CONCURRENTLY`
  option on index creation; MySQL skips it.

### 8. sql-aggregate — pumps/sql_aggregate.go
- Component: `pumps-sql-aggregate`
- Parent SYS-REQ: SYS-REQ-003
- Primary obligation_class: `nominal` (parent; sub-reqs carry the substantive
  obligations)
- Additional obligation_checklist: [`nominal`]
- Description: "The SQL Aggregate pump shall aggregate records per
  organisation and per dimension and upsert them into a SQL table (sharded
  per day when `TableSharding` is enabled), with the four substantive
  behaviors decomposed in the sub-reqs below."
- Files: pumps/sql_aggregate.go
- Key symbols: `SQLAggregatePump.WriteData`, `DoAggregatedWriting`,
  `ensureTable`, `ensureIndex`
- Notes: Parent only; sub-reqs carry obligations.

### 9. graph-sql — pumps/graph_sql.go
- Component: `pumps-sql-graph`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `parameterized_only_write`
- Additional obligation_checklist: [`parameterized_only_write`,
  `connection_leak_free`]
- Description: "The Graph SQL pump shall, on each purge, retain only records
  for which `IsGraphRecord()` is true, convert each to a `GraphRecord`, then
  if `TableSharding` is true split on each `YYYYMMDD` boundary and route
  each day-slice to a `<TableName>_<YYYYMMDD>` table (auto-migrated when
  missing), otherwise route all records to the configured `TableName`
  (default `tyk_analytics_graph`); within each table it shall issue
  parameter-bound batched inserts of `BatchSize` records using
  `gorm.Create`."
- Files: pumps/graph_sql.go
- Key symbols: `GraphSQLPump.Init`, `GraphSQLPump.WriteData`,
  `GraphSQLPump.getGraphRecords`
- Notes: Per-batch errors are logged but not propagated. Day-bucket
  algorithm duplicated from sql.go.

### 10. graph-sql-aggregate — pumps/graph_sql_aggregate.go
- Component: `pumps-sql-graph-aggregate`
- Parent SYS-REQ: SYS-REQ-003
- Primary obligation_class: `parameterized_only_write`
- Additional obligation_checklist: [`parameterized_only_write`,
  `connection_leak_free`]
- Description: "The Graph SQL Aggregate pump shall, on each purge,
  day-bucket records when `TableSharding` is enabled and route each
  day-slice to a `<AggregateGraphSQLTable>_<YYYYMMDD>` table (auto-migrated
  when missing); within each table it shall aggregate graph analytics via
  `analytics.AggregateGraphData` (windowed per minute when
  `StoreAnalyticsPerMinute` is true, otherwise per hour) and upsert each
  per-(API, dimension) row with `clause.OnConflict` on `id` using
  `analytics.OnConflictAssignments`; per-batch errors shall be returned to
  the caller."
- Files: pumps/graph_sql_aggregate.go
- Key symbols: `GraphSQLAggregatePump.WriteData`, `DoAggregatedWriting`
- Notes: This implementation **does** propagate write errors (unlike
  sql-standard / graph-sql / mcp-sql which swallow them per-batch). Honest
  reflection of the code.

### 11. mcp-sql — pumps/mcp_sql.go
- Component: `pumps-sql-mcp`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `parameterized_only_write`
- Additional obligation_checklist: [`parameterized_only_write`,
  `connection_leak_free`]
- Description: "The MCP SQL pump shall, on each purge, retain only records
  for which `IsMCPRecord()` is true, convert each to an `MCPRecord`, then if
  `TableSharding` is true split on each `YYYYMMDD` boundary and route each
  day-slice to a `<TableName>_<YYYYMMDD>` table (auto-migrated when
  missing), otherwise route all records to the configured `TableName`
  (default `tyk_analytics_mcp`); within each table it shall issue
  parameter-bound batched inserts of `BatchSize` records using
  `gorm.Create`."
- Files: pumps/mcp_sql.go
- Key symbols: `MCPSQLPump.WriteData`, `getMCPRecords`, `writeMCPBatch`,
  `ensureMCPShardedTable`
- Notes: Per-batch errors are logged but not propagated.

### 12. mcp-sql-aggregate — pumps/mcp_sql_aggregate.go
- Component: `pumps-sql-mcp-aggregate`
- Parent SYS-REQ: SYS-REQ-003
- Primary obligation_class: `parameterized_only_write`
- Additional obligation_checklist: [`parameterized_only_write`,
  `connection_leak_free`]
- Description: "The MCP SQL Aggregate pump shall, on each purge, day-bucket
  records when `TableSharding` is enabled and route each day-slice to a
  `<AggregateMCPSQLTable>_<YYYYMMDD>` table (auto-created via
  `CreateTable` when missing); within each table it shall aggregate MCP
  records via `analytics.AggregateMCPData` (windowed per minute when
  `StoreAnalyticsPerMinute` is true, otherwise per hour) and upsert each
  per-(API, dimension) row with `clause.OnConflict` on `id` using
  `analytics.OnConflictAssignments`; per-batch errors shall be returned to
  the caller. On initialisation, a composite index on `(dimension,
  timestamp, org_id, dimension_value)` is created (Postgres uses
  `CONCURRENTLY` and creates it on a background goroutine)."
- Files: pumps/mcp_sql_aggregate.go
- Key symbols: `MCPSQLAggregatePump.WriteData`, `DoAggregatedWriting`,
  `ensureTable`, `ensureIndex`, `ensureMCPAggregateShardedTable`,
  `aggregationTimeMinutes`, `writeAggregatedSlice`
- Notes: Propagates errors (unlike non-aggregate MCP SQL pump).

---

### 13. influx-v1 — pumps/influx.go
- Component: `pumps-influx-v1`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`]
- Description: "The InfluxDB v1 pump shall, on each purge, open a fresh
  HTTP client to the configured Influx v1 address, build an InfluxDB 1.x
  line-protocol `BatchPoints` with operator-configured `Tags` and `Fields`
  selected from each analytics record (timestamps recorded at `time.Now()`,
  microsecond precision), and write the batch via the v1 client."
- Files: pumps/influx.go
- Key symbols: `InfluxPump.WriteData`, `InfluxPump.connect`
- Notes: Honest assessment: `WriteData` discards the return value of
  `c.Write(bp)` and always returns `nil` (line 169) — write errors are
  silently ignored. Cannot honestly claim `errors_propagated`. The
  `connect()` function unconditionally recurses on error (no backoff cap)
  which is a latent stack-overflow risk under sustained Influx outage —
  worth flagging as a separate known-issue if not already tracked.
  Honest obligation: `nominal`.

### 14. influx-v2 — pumps/influx2.go
- Component: `pumps-influx-v2`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`]
- Description: "The InfluxDB v2 pump shall, on each purge, append one Point
  per analytics record to the v2 client's asynchronous WriteAPI for the
  configured organisation and bucket, using operator-configured `Tags` and
  `Fields` and microsecond precision; when `Flush` is true the WriteAPI
  shall be drained synchronously before the function returns. On `Init`,
  the pump checks server readiness, resolves the organisation by name, and
  optionally creates the bucket (with operator-configured retention rules)
  when `CreateMissingBucket` is enabled."
- Files: pumps/influx2.go
- Key symbols: `Influx2Pump.WriteData`, `Influx2Pump.Init`,
  `Influx2Pump.connect`, `Influx2Pump.createBucket`, `Influx2Pump.Shutdown`
- Notes: WriteAPI is asynchronous and write errors are not surfaced from
  `WriteData`; this is an inherent property of the v2 client's WriteAPI.
  `errors_propagated` would be aspirational here — `nominal` is honest.
  Init **does** propagate errors (server-not-ready, org-lookup,
  bucket-lookup).

---

### 15. http-splunk — pumps/splunk.go
- Component: `pumps-http-splunk`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `errors_propagated`
- Additional obligation_checklist: [`errors_propagated`,
  `cert_validation_strict`, `external_call_failure_observable`]
- Description: "The Splunk pump shall POST each analytics record (or a
  batch up to `BatchMaxContentLength` bytes when `EnableBatch` is true) as a
  Splunk HEC event JSON to `CollectorURL` with `Authorization: Splunk
  <token>`; TLS configuration (server-name, CA, client cert/key, and the
  `SSLInsecureSkipVerify` operator override) shall be applied via
  `NewTLSConfig`; transient send failures shall be retried up to
  `MaxRetries` via `retry.BackoffHTTPRetry`. Send errors shall be returned
  to the caller. When `ObfuscateAPIKeys` is true, API keys are masked with
  `****` plus the trailing `ObfuscateAPIKeysLength` characters."
- Files: pumps/splunk.go
- Key symbols: `SplunkPump.Init`, `SplunkPump.WriteData`, `SplunkPump.send`,
  `newSplunkClient`, `SplunkPump.FilterTags`
- Notes: `SSLInsecureSkipVerify` is operator-configurable — this is the
  reason `cert_validation_strict` belongs on the checklist (the obligation
  acknowledges the toggle exists; the obligation lives in justifying the
  operator's TLS-relaxation choices).

### 16. http-graylog — pumps/graylog.go
- Component: `pumps-http-graylog`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`]
- Description: "The Graylog pump shall, on each purge, encode each
  analytics record as a GELF JSON message containing only the
  operator-configured `Tags` field set, and send it to the configured
  Graylog host:port via the `gelf` client (typically UDP)."
- Files: pumps/graylog.go
- Key symbols: `GraylogPump.Init`, `GraylogPump.WriteData`,
  `GraylogPump.connect`
- Notes: Calls `p.log.Fatal(err)` on base64 decode and json.Marshal errors,
  which terminates the process — this is a defect-class behaviour; cannot
  claim `errors_propagated`. The transport is **GELF over UDP** (the
  underlying `robertkowalski/graylog-golang` library is UDP-only) — there
  is no TLS, no acks; `WriteData` always returns nil. If the client is nil
  it recurses into `connect()` then re-enters `WriteData(ctx, data)`
  recursively without bound — additional latent issue. Honest obligation:
  `nominal`. Recommend a known-issue for the recursive `WriteData` and the
  `log.Fatal` calls.

### 17. http-syslog — pumps/syslog.go
- Component: `pumps-http-syslog`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`]
- Description: "The Syslog pump shall, on each purge, write one JSON
  message per analytics record (with `\\n` escaping in `raw_request` and
  `raw_response`) to a syslog destination using the operator-configured
  `Transport` (udp/tcp/tls), `NetworkAddr`, `LogLevel` (syslog severity
  0-7), and `Tag`."
- Files: pumps/syslog.go
- Key symbols: `SyslogPump.Init`, `SyslogPump.initWriter`,
  `SyslogPump.initConfigs`, `SyslogPump.WriteData`
- Notes: Uses Go's stdlib `log/syslog` which emits RFC 3164 (BSD-style)
  framing, not RFC 5424. Write errors from `fmt.Fprintf(s.writer, ...)`
  are intentionally discarded (`_, _ = ...` at line 185) — `WriteData`
  always returns `nil`. `initWriter` calls `log.Fatal` on dial failure.
  `nominal` is honest. The fact that ctx is checked between records
  (`select { case <-ctx.Done(): return nil ...}`) is the only ctx-aware
  pump in this group, worth noting.

### 18. http-logzio — pumps/logzio.go
- Component: `pumps-http-logzio`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`]
- Description: "The Logz.io pump shall, on each purge, marshal each
  analytics record into a Logz.io JSON document and enqueue it via the
  `logzio-go` SDK sender, which buffers on disk in `QueueDir` and drains
  every `DrainDuration` to the configured URL (default
  `https://listener.logz.io:8071`), bounded by `DiskThreshold` (1-100%)
  when `CheckDiskSpace` is true."
- Files: pumps/logzio.go
- Key symbols: `LogzioPump.Init`, `LogzioPump.WriteData`, `NewLogzioClient`
- Notes: `sender.Send` returns no error from the SDK at the per-record
  call site (it is fire-and-enqueue) — `WriteData` cannot return a useful
  per-record error. Only marshalling errors propagate. Honest:
  `nominal`. (Has the strongest disk-buffering semantics of the
  SaaS-HTTP group — worth keeping its own req.)

### 19. http-moesif — pumps/moesif.go
- Component: `pumps-http-moesif`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`, `untrusted_input_bounded`]
- Description: "The Moesif pump shall, on each purge, parse each
  analytics record's base64-encoded raw request/response, apply
  operator-configured header/body masking and capture toggles, identify the
  user via the configured `UserIDHeader` (or fall back to record `Alias` /
  `OauthID` / parsed Authorization header), apply per-user / per-company
  sample-rate filtering fetched from the Moesif app-config endpoint, and
  enqueue qualifying events into the Moesif SDK with a weight equal to
  `floor(100/samplingPercentage)`. The app-config sampling rules shall be
  refreshed once per minute when the SDK's `X-Moesif-Config-Etag` differs
  from the cached `eTag`."
- Files: pumps/moesif.go
- Key symbols: `MoesifPump.WriteData`, `MoesifPump.parseConfiguration`,
  `MoesifPump.getSamplingPercentage`, `decodeRawData`, `maskRawBody`,
  `maskData`, `fetchIDFromHeader`, `parseAuthorizationHeader`, `buildURI`
- Notes: Multiple `p.log.Fatal(err)` calls on base64 decode failures
  (lines 349, 357, 380, 387) — like graylog, this is process-fatal and
  cannot honestly claim `errors_propagated`. The `QueueEvent` error is
  logged but not returned. `nominal` is honest. The masking and sampling
  logic is *uniquely complex* in this pump and deserves its own SW req —
  honest description above captures it. Per-record `rand.Seed(time.Now())`
  is a latent determinism quirk worth flagging.

### 20. http-segment — pumps/segment.go
- Component: `pumps-http-segment`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`]
- Description: "The Segment pump shall, on each purge, marshal each
  analytics record to JSON and submit it as a Segment `Track` event with
  `Event=\"Hit\"` and `AnonymousId` set to the record's `APIKey`, via the
  `segmentio/analytics-go` SDK using the configured `segment_write_key`."
- Files: pumps/segment.go
- Key symbols: `SegmentPump.WriteData`, `WriteDataRecord`, `ToJSONMap`
- Notes: SDK error from `Track` is logged but not returned; per-record
  marshalling error is logged but `WriteDataRecord` always returns nil.
  `WriteData` always returns nil. The smallest pump in the family.

### 21. http-resurface — pumps/resurface.go
- Component: `pumps-http-resurface`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`]
- Description: "The Resurface pump shall, on each purge, hand the data
  batch to an internal goroutine via a buffered channel (capacity 5); the
  worker shall parse each record's base64-encoded raw HTTP request and
  response, reconstruct `http.Request` / `http.Response` instances
  (including request URL, headers, custom `tyk-API-*` headers, optional
  chunked-trailer parsing), and submit them via
  `logger.SendHttpMessage`. `WriteData` shall return `ctx.Err()` on
  context cancellation and shall not block when the worker queue is full
  while the pump is enabled."
- Files: pumps/resurface.go
- Key symbols: `ResurfacePump.WriteData`, `ResurfacePump.writeData`,
  `ResurfacePump.initWorker`, `mapRawData`, `parseHeaders`, `Flush`,
  `Shutdown`
- Notes: This is the only HTTP-logging pump with a real async-worker /
  channel design and ctx-aware send. Honest description captures the
  worker pattern. The worker swallows record-level errors (`continue`).
  `nominal` is honest; could promote to `errors_propagated` only at the
  WriteData level (ctx errors).

---

### 22. aws-sqs — pumps/sqs.go
- Component: `pumps-aws-sqs`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `errors_propagated`
- Additional obligation_checklist: [`errors_propagated`,
  `external_call_failure_observable`]
- Description: "The SQS pump shall, on each purge, JSON-marshal each
  analytics record into a `SendMessageBatchRequestEntry` (Id generated as a
  ULID), optionally attaching `AWSMessageGroupID`,
  `MessageDeduplicationId` (when `AWSMessageIDDeduplicationEnabled` is
  true), and `DelaySeconds`; then submit batches of
  `AWSSQSBatchLimit` entries via `SendMessageBatch` on the resolved queue
  URL. The first batch error shall be returned to the caller."
- Files: pumps/sqs.go
- Key symbols: `SQSPump.WriteData`, `SQSPump.write`, `SQSPump.NewSQSPublisher`
- Notes: Honestly propagates batch errors (line 191-193). Code has a
  duplicate `MessageGroupId` assignment (lines 151 and 159) — harmless
  but worth a cleanup note. `AWSSQSBatchLimit` is not bounded against
  the AWS hard cap of 10; if operator sets a larger value, AWS will
  reject — could be a separate `boundary` req later.

### 23. aws-kinesis — pumps/kinesis.go
- Component: `pumps-aws-kinesis`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`, `monotonicity`]
- Description: "The Kinesis pump shall, on each purge, split records into
  batches of `BatchSize` (default 100, bounded by AWS's 500-record
  per-`PutRecords` limit), JSON-marshal each record into a Kinesis
  `PutRecordsRequestEntry` with `PartitionKey` set to a fresh
  `crypto/rand` integer (for even MD5-based shard distribution), and
  submit each batch via `PutRecords` to the configured `StreamName`. When
  `KMSKeyID` is configured at Init, the pump shall verify the stream's
  current encryption mode/key, enable KMS server-side encryption if not
  already enabled with that key, and fail Init if the stream is already
  encrypted with a different key."
- Files: pumps/kinesis.go
- Key symbols: `KinesisPump.WriteData`, `splitIntoBatches`,
  `KinesisPump.Init`
- Notes: `WriteData` always returns `nil` even when `PutRecords` errors
  (line 205-207 only logs) — cannot honestly claim
  `errors_propagated` at the WriteData level. Per-record `ErrorCode`
  responses from `PutRecords.Records[]` are debug-logged only.
  `nominal` is the honest pick. Init does propagate errors. The
  partition-key randomness is `crypto/rand` (good) but the comment
  "even distribute" is correct only modulo md5(key), which is OK.

### 24. aws-timestream — pumps/timestream.go
- Component: `pumps-aws-timestream`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `errors_propagated`
- Additional obligation_checklist: [`errors_propagated`, `boundary`]
- Description: "The Timestream pump shall, on each purge, split records
  into batches of `timestreamMaxRecordsCount=100` (AWS hard limit),
  convert each record into a multimeasure `types.Record` with
  operator-configured `Dimensions` and `Measures` (with optional
  `NameMappings` rename, optional zero-value emission, and optional
  `RateLimit` measure inclusion), and submit each batch via
  `WriteRecords` to the configured `DatabaseName`/`TableName`. The first
  batch error shall be returned to the caller; `RejectedRecordsException`
  shall be logged with the first rejection reason."
- Files: pumps/timestream.go
- Key symbols: `TimestreamPump.WriteData`,
  `TimestreamPump.BuildTimestreamInputIterator`,
  `MapAnalyticRecord2TimestreamMultimeasureRecord`,
  `GetAnalyticsRecordDimensions`, `GetAnalyticsRecordMeasures`,
  `TimestreamPump.NewTimestreamWriter`
- Notes: Honestly propagates write errors (line 141). Boundary
  obligation lives on the AWS-hard-limit batch-size cap (100).

---

## mongo-aggregate sub-reqs (further per-significant-behavior splits)

### 25. mongo-aggregate-windowing — windowing policy
- Component: `pumps-mongo-aggregate`
- Parent SYS-REQ: SYS-REQ-003
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`, `determinism`]
- Description: "The MongoAggregate pump's aggregation window shall be set
  at Init by `SetAggregationTime`: when `StoreAnalyticsPerMinute` is
  true the window shall be 1 minute, otherwise `AggregationTime`
  (configurable 1-60 minutes, defaulting to 60 when unset or
  out-of-range). The active window shall be passed to
  `analytics.AggregateData(...)` on each `WriteData` call."
- Files: pumps/mongo_aggregate.go
- Key symbols: `MongoAggregatePump.SetAggregationTime`, the
  `AggregationTime` argument in `analytics.AggregateData` at WriteData

### 26. mongo-aggregate-sharding — per-org collection sharding + mixed
- Component: `pumps-mongo-aggregate`
- Parent SYS-REQ: SYS-REQ-003
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`]
- Description: "The MongoAggregate pump shall route each per-organisation
  aggregate document into a collection named
  `z_tyk_analyticz_aggregate_<orgid>` via `GetCollectionName`; when
  `UseMixedCollection` is true, each per-org aggregate shall additionally
  be written to the shared mixed collection
  `AgggregateMixedCollectionName` (`Mixed=true` variant). Empty `orgid`
  shall return an error from `GetCollectionName`."
- Files: pumps/mongo_aggregate.go
- Key symbols: `MongoAggregatePump.GetCollectionName`,
  `MongoAggregatePump.WriteData` (writingAttempts loop),
  `DoAggregatedWriting`

### 27. mongo-aggregate-counter-inc — $inc upsert semantics
- Component: `pumps-mongo-aggregate`
- Parent SYS-REQ: SYS-REQ-018 / SYS-REQ-019 (aggregation dimensions /
  counters). Note: this also relies on assumption SYS-REQ-029 (mongo $inc
  atomicity).
- Primary obligation_class: `atomicity`
- Additional obligation_checklist: [`atomicity`, `monotonicity`]
- Description: "The MongoAggregate pump shall persist aggregation
  counters via a two-step MongoDB upsert keyed by `{orgid, timestamp}`:
  step one applies `$inc`/`$set`/`$max`/`$min` operators built by
  `AnalyticsRecordAggregate.AsChange()` (preserving monotonicity of
  hits/errors/success counters under concurrent writers via Mongo's
  document-level $inc atomicity — see SYS-REQ-029), step two
  recalculates derived averages via `AsTimeUpdate()`. The first upsert
  error shall be returned to the caller."
- Files: pumps/mongo_aggregate.go
- Key symbols: `MongoAggregatePump.DoAggregatedWriting`

### 28. mongo-aggregate-tag-bound — tag-list bounding alert
- Component: `pumps-mongo-aggregate`
- Parent SYS-REQ: SYS-REQ-010 (record bounded size) — closest fit; this
  is an operator-warning bound rather than a hard truncation.
- Primary obligation_class: `boundary`
- Additional obligation_checklist: [`boundary`,
  `denial_of_service_resistant`]
- Description: "When the count of tags on an aggregated document exceeds
  `ThresholdLenTagList` (default 1000), the MongoAggregate pump shall
  emit a `Warn`-level alert listing up to `CommonTagsCount` (5) common
  tag prefixes computed by `getListOfCommonPrefix`, instructing the
  operator to suppress noisy tags via `ignore_tag_prefix_list`. A
  configured value of `-1` shall disable the alert entirely."
- Files: pumps/mongo_aggregate.go
- Key symbols: `MongoAggregatePump.printAlert`, `getListOfCommonPrefix`,
  `ThresholdLenTagList` checks in `DoAggregatedWriting`

### 29. mongo-aggregate-self-heal — max-doc-size self-healing
- Component: `pumps-mongo-aggregate`
- Parent SYS-REQ: SYS-REQ-004 (recoverable error handling)
- Primary obligation_class: `error_handling`
- Additional obligation_checklist: [`error_handling`, `boundary`,
  `monotonicity`]
- Description: "When `EnableAggregateSelfHealing` is true and an upsert
  fails with a max-document-size error from MongoDB (`'Size must be
  between 0 and'`), CosmosDB (`'Request size is too large'`), or
  DocumentDB (`'Resulting document after update is larger than'`), the
  MongoAggregate pump shall (a) halve `AggregationTime` (unless already
  1, in which case it skips self-healing and returns the error), (b)
  reset the last-document-timestamp tracker so subsequent writes form a
  new document, and (c) recursively re-invoke `WriteData` once with the
  same input data. Self-healing only triggers on the listed size errors,
  not on arbitrary write failures."
- Files: pumps/mongo_aggregate.go
- Key symbols: `MongoAggregatePump.ShouldSelfHeal`,
  `MongoAggregatePump.divideAggregationTime`, `WriteData` recursive call

### 30. mongo-aggregate-index-ensure — index lifecycle
- Component: `pumps-mongo-aggregate`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`, `idempotency`]
- Description: "On each `DoAggregatedWriting` call (per-org), the
  MongoAggregate pump shall idempotently ensure baseline indexes on the
  target aggregate collection: (a) a TTL index on `expireAt` with TTL=0
  (skipped for CosmosDB), (b) a `timestamp` index, (c) an `orgid`
  index. Index creation shall be skipped when `OmitIndexCreation` is
  true or, for StandardMongo, when the collection already exists.
  Background index creation is used for StandardMongo; foreground for
  CosmosDB/DocumentDB."
- Files: pumps/mongo_aggregate.go
- Key symbols: `MongoAggregatePump.ensureIndexes`, `collectionExists`

---

## sql-aggregate sub-reqs (further per-behavior splits)

### 31. sql-aggregate-day-bucket — day-bucket batching with date-boundary split
- Component: `pumps-sql-aggregate`
- Parent SYS-REQ: SYS-REQ-003
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`, `boundary`]
- Description: "When `TableSharding` is enabled, the SQL Aggregate pump
  shall scan the incoming records in their existing order and, on each
  `YYYYMMDD` timestamp boundary, route the preceding contiguous slice
  to a `tyk_aggregated_<YYYYMMDD>` table (ensured via `ensureTable` and
  indexed via `ensureIndex`); the trailing slice shall be routed using
  the last record's date. When `TableSharding` is disabled, all records
  shall be routed to the single `tyk_aggregated` table in one pass."
- Files: pumps/sql_aggregate.go
- Key symbols: `SQLAggregatePump.WriteData` (day-bucket loop)

### 32. sql-aggregate-table-ensure — dimension table ensure
- Component: `pumps-sql-aggregate`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`, `idempotency`]
- Description: "The SQL Aggregate pump shall idempotently create the
  target aggregate table (default `tyk_aggregated` or a sharded
  `tyk_aggregated_<YYYYMMDD>`) via GORM `CreateTable` against the
  `SQLAnalyticsRecordAggregate` schema whenever the table is absent.
  For non-sharded mode, this is done once at Init; for sharded mode,
  on each new date boundary in `WriteData`."
- Files: pumps/sql_aggregate.go
- Key symbols: `SQLAggregatePump.ensureTable`, `SQLAggregatePump.Init`,
  WriteData loop

### 33. sql-aggregate-index-ensure — index ensure with CONCURRENTLY
- Component: `pumps-sql-aggregate`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`, `idempotency`]
- Description: "The SQL Aggregate pump shall, unless
  `OmitIndexCreation` is true, ensure a composite index named
  `<table>_idx_dimension` on `(dimension, timestamp, org_id,
  dimension_value)` via `CREATE INDEX [CONCURRENTLY] IF NOT EXISTS`.
  For PostgreSQL the `CONCURRENTLY` option shall be used and, in the
  non-sharded mode, the index creation shall run on a background
  goroutine signalling `backgroundIndexCreated` on completion; for
  MySQL the index creation shall run synchronously without
  `CONCURRENTLY`. Index creation is skipped when the index already
  exists."
- Files: pumps/sql_aggregate.go
- Key symbols: `SQLAggregatePump.ensureIndex`

### 34. sql-aggregate-upsert-on-conflict — on-conflict assignment semantics
- Component: `pumps-sql-aggregate`
- Parent SYS-REQ: SYS-REQ-018 / SYS-REQ-019 (aggregation counters)
- Primary obligation_class: `atomicity`
- Additional obligation_checklist: [`atomicity`, `monotonicity`,
  `parameterized_only_write`]
- Description: "The SQL Aggregate pump shall, per (orgID, dimension)
  row in each aggregated batch, issue a parameter-bound `INSERT ... ON
  CONFLICT (id) DO UPDATE SET ...` using
  `clause.OnConflict` with `analytics.OnConflictAssignments(table,
  \"excluded\")` so that monotonic counter columns are incremented
  rather than overwritten; insert failures shall be returned to the
  caller. Records are batched in slices of `BatchSize`."
- Files: pumps/sql_aggregate.go
- Key symbols: `SQLAggregatePump.DoAggregatedWriting`

---

## elasticsearch sub-reqs

### 35. es-version-dispatch — per-ES-version bulk processor dispatch
- Component: `pumps-es`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`,
  `cert_validation_strict`]
- Description: "The Elasticsearch pump shall, at Init, select an
  `ElasticsearchOperator` implementation per configured `Version`
  (`\"3\"`, `\"5\"`, `\"6\"`, or `\"7\"`, defaulting to `\"3\"` when
  unset) — instantiating the corresponding `elastic.v3` / `elastic.v5`
  / `elastic.v6` / `elastic/v7` client (with sniffing toggle, basic
  auth, optional ApiKey-auth via `ApiKeyTransport`, and optional
  custom TLS) and corresponding `BulkProcessor`. Versions other than
  3/5/6/7 shall be rejected at Init via `log.Fatal`."
- Files: pumps/elasticsearch.go
- Key symbols: `ElasticsearchPump.getOperator`,
  `ElasticsearchPump.connect`, `Elasticsearch{3,5,6,7}Operator`,
  `ApiKeyTransport`

### 36. es-rolling-index-naming — rolling-index naming policy
- Component: `pumps-es`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `nominal`
- Additional obligation_checklist: [`nominal`, `determinism`]
- Description: "When `RollingIndex` is true, the Elasticsearch pump
  shall append `-YYYY.MM.DD` (UTC system time) to the target index
  name. Per-record routing shall use `getIndexNameForRecord`: when
  `MCPIndexName` is non-empty and the record is an MCP record, the
  MCP-specific index name is used (with the same rolling suffix when
  enabled); otherwise the standard `IndexName` is used."
- Files: pumps/elasticsearch.go
- Key symbols: `getIndexName`, `getIndexNameForRecord`

### 37. es-bulk-flush-policy — bulk flush size + interval boundary policy
- Component: `pumps-es`
- Parent SYS-REQ: SYS-REQ-004
- Primary obligation_class: `boundary`
- Additional obligation_checklist: [`boundary`, `nominal`]
- Description: "For each ES version operator, when `DisableBulk` is
  false the `BulkProcessor` shall be configured with operator-supplied
  `BulkConfig.Workers`, `FlushInterval` (seconds), `BulkActions`
  (record count, default 1000, `-1` to disable), `BulkSize` (bytes,
  default 5 MB, `-1` to disable), and an `After` callback that logs
  the purged record count via `printPurgedBulkRecords`. When
  `DisableBulk` is true, each record is indexed individually via the
  per-version `Index().BodyJson(...).Do(ctx)` call. `Shutdown` flushes
  the bulk processor when bulk is enabled."
- Files: pumps/elasticsearch.go
- Key symbols: `getOperator` (per-version BulkProcessor config),
  `processData` per-version, `printPurgedBulkRecords`,
  `ElasticsearchPump.Shutdown`

---

## Recommended creation order

Use the following ordering with the parent agent's `proof req new` commands.
The order minimizes forward-reference issues (parents before sub-reqs) and
keeps related reqs adjacent for review.

```
# Mongo family
1.  proof req new mongo-standard          --parent SYS-REQ-004 --component pumps-mongo-standard
2.  proof req new mongo-selective         --parent SYS-REQ-004 --component pumps-mongo-selective
3.  proof req new mongo-aggregate         --parent SYS-REQ-003 --component pumps-mongo-aggregate
4.  proof req new graph-mongo             --parent SYS-REQ-004 --component pumps-mongo-graph
5.  proof req new mcp-mongo               --parent SYS-REQ-004 --component pumps-mongo-mcp
6.  proof req new mcp-mongo-aggregate     --parent SYS-REQ-003 --component pumps-mongo-mcp-aggregate

# SQL family
7.  proof req new sql-standard            --parent SYS-REQ-004 --component pumps-sql-standard
8.  proof req new sql-aggregate           --parent SYS-REQ-003 --component pumps-sql-aggregate
9.  proof req new graph-sql               --parent SYS-REQ-004 --component pumps-sql-graph
10. proof req new graph-sql-aggregate     --parent SYS-REQ-003 --component pumps-sql-graph-aggregate
11. proof req new mcp-sql                 --parent SYS-REQ-004 --component pumps-sql-mcp
12. proof req new mcp-sql-aggregate       --parent SYS-REQ-003 --component pumps-sql-mcp-aggregate

# Influx family
13. proof req new influx-v1               --parent SYS-REQ-004 --component pumps-influx-v1
14. proof req new influx-v2               --parent SYS-REQ-004 --component pumps-influx-v2

# HTTP-logging family
15. proof req new http-splunk             --parent SYS-REQ-004 --component pumps-http-splunk
16. proof req new http-graylog            --parent SYS-REQ-004 --component pumps-http-graylog
17. proof req new http-syslog             --parent SYS-REQ-004 --component pumps-http-syslog
18. proof req new http-logzio             --parent SYS-REQ-004 --component pumps-http-logzio
19. proof req new http-moesif             --parent SYS-REQ-004 --component pumps-http-moesif
20. proof req new http-segment            --parent SYS-REQ-004 --component pumps-http-segment
21. proof req new http-resurface          --parent SYS-REQ-004 --component pumps-http-resurface

# AWS family
22. proof req new aws-sqs                 --parent SYS-REQ-004 --component pumps-aws-sqs
23. proof req new aws-kinesis             --parent SYS-REQ-004 --component pumps-aws-kinesis
24. proof req new aws-timestream          --parent SYS-REQ-004 --component pumps-aws-timestream

# mongo-aggregate sub-reqs (created AFTER #3)
25. proof req new mongo-aggregate-windowing      --parent SYS-REQ-003 --component pumps-mongo-aggregate
26. proof req new mongo-aggregate-sharding       --parent SYS-REQ-003 --component pumps-mongo-aggregate
27. proof req new mongo-aggregate-counter-inc    --parent SYS-REQ-019 --component pumps-mongo-aggregate
28. proof req new mongo-aggregate-tag-bound      --parent SYS-REQ-010 --component pumps-mongo-aggregate
29. proof req new mongo-aggregate-self-heal      --parent SYS-REQ-004 --component pumps-mongo-aggregate
30. proof req new mongo-aggregate-index-ensure   --parent SYS-REQ-004 --component pumps-mongo-aggregate

# sql-aggregate sub-reqs (created AFTER #8)
31. proof req new sql-aggregate-day-bucket       --parent SYS-REQ-003 --component pumps-sql-aggregate
32. proof req new sql-aggregate-table-ensure     --parent SYS-REQ-004 --component pumps-sql-aggregate
33. proof req new sql-aggregate-index-ensure     --parent SYS-REQ-004 --component pumps-sql-aggregate
34. proof req new sql-aggregate-upsert           --parent SYS-REQ-019 --component pumps-sql-aggregate

# elasticsearch sub-reqs
35. proof req new es-version-dispatch            --parent SYS-REQ-004 --component pumps-es
36. proof req new es-rolling-index-naming        --parent SYS-REQ-004 --component pumps-es
37. proof req new es-bulk-flush-policy           --parent SYS-REQ-004 --component pumps-es
```

(Operator: 30 new reqs in total; the numbering above is presentation order. The
existing family reqs SW-018/019/020/022/027/028 should be **deprecated** —
either marked `status: superseded` with a `supersedes_by` cross-link to the
list of new IDs, or deleted after the new reqs are approved and annotations
re-pointed.)

## Annotation re-map plan

Each `// reqproof:implements` comment in production code must be
re-pointed from the family-level req to the new atomic req. Mapping by file:

| File                                  | Current        | Re-point to (new slug)            |
|---------------------------------------|----------------|-----------------------------------|
| pumps/mongo.go                        | SW-REQ-018     | `mongo-standard`                  |
| pumps/mongo_selective.go              | SW-REQ-018     | `mongo-selective`                 |
| pumps/mongo_aggregate.go              | SW-REQ-018     | `mongo-aggregate` (parent) PLUS targeted sub-req per function:<br>• `WriteData`, `SetAggregationTime` → `mongo-aggregate-windowing` <br>• `GetCollectionName`, `DoAggregatedWriting` writing-attempts loop → `mongo-aggregate-sharding`<br>• `DoAggregatedWriting` upsert calls → `mongo-aggregate-counter-inc`<br>• `printAlert`, `getListOfCommonPrefix` → `mongo-aggregate-tag-bound`<br>• `ShouldSelfHeal`, `divideAggregationTime` → `mongo-aggregate-self-heal`<br>• `ensureIndexes`, `collectionExists` → `mongo-aggregate-index-ensure` |
| pumps/graph_mongo.go                  | SW-REQ-018     | `graph-mongo`                     |
| pumps/mcp_mongo.go                    | SW-REQ-018     | `mcp-mongo`                       |
| pumps/mcp_mongo_aggregate.go          | SW-REQ-018     | `mcp-mongo-aggregate`             |
| pumps/sql.go                          | SW-REQ-019     | `sql-standard`                    |
| pumps/sql_aggregate.go                | SW-REQ-019     | `sql-aggregate` (parent) PLUS:<br>• `WriteData` day-bucket loop → `sql-aggregate-day-bucket`<br>• `ensureTable` → `sql-aggregate-table-ensure`<br>• `ensureIndex` → `sql-aggregate-index-ensure`<br>• `DoAggregatedWriting` → `sql-aggregate-upsert` |
| pumps/graph_sql.go                    | SW-REQ-019     | `graph-sql`                       |
| pumps/graph_sql_aggregate.go          | SW-REQ-019     | `graph-sql-aggregate`             |
| pumps/mcp_sql.go                      | SW-REQ-019     | `mcp-sql`                         |
| pumps/mcp_sql_aggregate.go            | SW-REQ-019     | `mcp-sql-aggregate`               |
| pumps/influx.go                       | SW-REQ-022     | `influx-v1`                       |
| pumps/influx2.go                      | SW-REQ-022     | `influx-v2`                       |
| pumps/splunk.go                       | SW-REQ-027     | `http-splunk`                     |
| pumps/graylog.go                      | SW-REQ-027     | `http-graylog`                    |
| pumps/syslog.go                       | SW-REQ-027     | `http-syslog`                     |
| pumps/logzio.go                       | SW-REQ-027     | `http-logzio`                     |
| pumps/moesif.go                       | SW-REQ-027     | `http-moesif`                     |
| pumps/segment.go                      | SW-REQ-027     | `http-segment`                    |
| pumps/resurface.go                    | SW-REQ-027     | `http-resurface`                  |
| pumps/sqs.go                          | SW-REQ-028     | `aws-sqs`                         |
| pumps/kinesis.go                      | SW-REQ-028     | `aws-kinesis`                     |
| pumps/timestream.go                   | SW-REQ-028     | `aws-timestream`                  |
| pumps/elasticsearch.go                | SW-REQ-020     | `es-version-dispatch` for `getOperator` and `connect`; `es-rolling-index-naming` for `getIndexName`, `getIndexNameForRecord`; `es-bulk-flush-policy` for per-version `processData`, `printPurgedBulkRecords`, `Shutdown`; the per-version `flushRecords` methods → `es-bulk-flush-policy` |

For each file, the existing top-level `New`/`GetName`/`GetEnvPrefix`/`Init`
helper annotations may simply re-point to the per-implementation slug
(they're scaffolding rather than substantive behavior). The behavior-
specific annotations on mongo_aggregate.go, sql_aggregate.go, and
elasticsearch.go should be split per the bullet lists above.

## STK acceptance criteria updates

Stakeholder ACs (`derived_reqs`) trace to **SYS reqs**, not SW reqs, so the
STK→AC linkage is **unaffected** by this Phase A SW decomposition. Verified by
inspecting STK-REQ-002 (lists SYS-REQ-004/005/006/007/023), STK-REQ-001
(lists SYS reqs only), STK-REQ-003 (SYS reqs only), etc.

The only system-level adjustment worth noting (not strictly required for
Phase A but useful for Phase B traceability):

- `SYS-REQ-029` (the `$inc` atomicity assumption) currently has `component:
  pumps-mongo-aggregate`. If we split the component to
  `pumps-mongo-aggregate` (parent) and the new component name is unchanged,
  no edit is needed. Verified by reading SYS-REQ-029.

No STK AC `derived_reqs` lists need editing for Phase A.

---

## Code-verified caveats and known issues to surface during req-create

These are observations from reading the actual code that the new req
descriptions must reflect (or known-issue files must capture). They were
identified during the Phase A read-through.

1. **mongo-pump-ignores-caller-context** (existing known-issue) applies to
   `mongo-standard`, `mongo-selective`, `graph-mongo`, and `mcp-mongo` —
   all four pass `context.Background()` to `m.store.Insert` instead of the
   caller `ctx`. The known-issue file currently names only
   `pumps/mongo.go:402-440`; the description should be amended to cover
   the other three files. This is the "E1" the task brief alludes to.
2. **Per-batch error swallowing** — `mongo-selective`, `sql-standard`,
   `graph-sql`, `mcp-sql`, `influx-v1`, `influx-v2`, `kinesis`,
   `graylog`, `syslog`, `logzio`, `segment` all log per-batch errors
   without returning them. The new reqs honestly say so; the existing
   family reqs falsely imply uniform propagation. Worth a Phase D
   spec-vs-code mismatch fix.
3. **`log.Fatal` on non-fatal record errors** — `graylog` (base64 + json),
   `moesif` (base64), `syslog.initWriter` (dial), `influx.Init` (config),
   `mongo.Init` (config) all call `log.Fatal` on what are arguably
   recoverable errors. New reqs honestly note `nominal` rather than
   `errors_propagated`. A separate Phase E known-issue for the `log.Fatal`
   anti-pattern is recommended.
4. **`graylog.WriteData` recursive on nil client** (line 111-113) and
   **`influx.connect` recursive on error** (line 95-96) — both lack
   recursion bounds; under sustained outage they can stack-overflow.
   Recommend a new known-issue.
5. **`moesif.WriteData` calls `rand.Seed(time.Now().UnixNano())` on every
   record** (line 472-473) — both a determinism quirk and a tiny perf
   regression. Worth flagging.
6. **`sqs.go` duplicate MessageGroupId assignment** (lines 151 and 159) —
   cosmetic but the new `aws-sqs` req can either ignore it or include a
   cleanup follow-up.
7. **`elasticsearch.connect` recursive on error** (line 412-415) — same
   pattern as `influx.connect`. Same known-issue applies.
