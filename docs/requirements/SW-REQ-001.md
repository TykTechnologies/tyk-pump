# SW-REQ-001: Purge loop and main startup pipeline

## Intent
Realises the parent **SYS-REQ-001** ingestion guarantee inside the `main` package. On startup, `main.Init` loads the JSON config and applies log-level/format overrides on top of the defaults that `logger.init` already installed; then on every tick the purge loop iterates the legacy `tyk-system-analytics` key plus shards `_0..9`, pops each with both serializer suffixes (`""` for msgpack, `"_protobuf"` for protobuf), decodes the records, hands them through `filterData` (per-pump filters and the configured privacy ops), and dispatches the survivors to every configured pump in parallel.

## Motivation
This is the orchestration layer of the pump. Capturing it at the SW level locks in three design choices that the system-layer requirement glosses over: (1) a fixed-cadence `time.Tick` loop (no jitter, no adaptive scheduling); (2) decoding happens once in `main` and the decoded `AnalyticsRecord` is fanned out to all pumps so each pump pays only filter/copy cost, not deserialize cost; (3) privacy ops (`OmitDetailedRecording`, `TrimRawData`, `RemoveIgnoredFields`, base64 decode of raw req/resp) are applied per-pump in `filterData` so two pumps with different privacy configs see different views of the same underlying record. The trade-off is memory: `filteredKeys := make(...); copy(...)` allocates a per-pump slice on every cycle rather than reusing one.

## Code references
- `main.go:498 main` — boots `Init`, instrumentation, health server, store, pumps, then `StartPurgeLoop` under a `WaitGroup` cancelled by SIGINT/SIGTERM.
- `main.go:71 Init` — `LoadConfig`, JSON-driven `log.Formatter`/`log.Level` override (lines 79-118), serializer slice construction at line 90.
- `main.go:261 StartPurgeLoop` — `time.Tick`, shard loop `i := -1; i < 10`, per-serializer `analyticsKeyName += serializerMethod.GetSuffix()`.
- `main.go:310 PreprocessAnalyticsValues` — per-record `serializerMethod.Decode` + error-skip + `writeToPumps`.
- `main.go:361 writeToPumps` — `sync.WaitGroup` fanout, one goroutine per pump.
- `main.go:378 filterData` — fast-path return when no privacy/filter/decoding settings; otherwise `OmitDetailedRecording`, `TrimRawData(MaxRecordSize)`, `ShouldFilter`, `RemoveIgnoredFields`, base64 decode of `RawRequest`/`RawResponse`.
- `main.go:435 execPumpWriting` — per-pump goroutine with timeout context and purge-delay watchdog timer.
- `api.go` — placeholder (single `package main` line); no behaviour.

## Evidence
- `main_test.go:55 TestFilterData`, `main_test.go:78 TestTrimData`, `main_test.go:145 TestOmitDetailsFilterData`, `main_test.go:298 TestIgnoreFieldsFilterData`, `main_test.go:350 TestDecodedKey` — each tagged `// Verifies: SW-REQ-001` and exercise individual privacy/filter branches of `filterData`.
- `main_test.go:168 TestWriteDataWithFilters` — tagged `// Verifies: SW-REQ-001` and `SW-REQ-003`; drives `writeToPumps` end-to-end across multiple pumps.

## Related requirements
`SW-REQ-095` decomposes the TT-5776 fan-out isolation invariant from this
startup/purge-loop requirement: per-backend `filterData` filtering and privacy
transforms must operate on an isolated view rather than mutating the shared
decoded batch.

## Open questions
- Privacy ops are applied in `main.filterData` not in `pumps/common.go`; the SYS-layer "per-pump privacy" obligation has no symmetric pump-side enforcement, so a pump implementation that bypasses `filterData` (e.g., by being called from a code path other than `execPumpWriting`) would silently leak raw fields. Demo mode (`main.go:516`) calls `writeToPumps` directly with hand-crafted keys.
- The shard ceiling `i < 10` at `main.go:267` is hard-coded and shared with SYS-REQ-001; if a gateway shards beyond 10 keys the trailing data is dropped silently.
