# SW-REQ-015: Uptime report data model

## Intent
Realises parent **SYS-REQ-014** at the data-model layer. `UptimeReportData` is the single struct carrying the fields needed for availability reporting: `URL`, `RequestTime`, `ResponseCode`, `TCPError`, `ServerError`, `Day/Month/Year/Hour/Minute/TimeStamp`, `ExpireAt`, `APIID`, `OrgID`, plus the persistent-model `ID` for ObjectID round-trip. The accompanying `UptimeReportAggregate` (built by `AggregateUptimeData`) holds per-URL and per-error-code `Counter` maps plus a `Total` counter, and `OnConflictUptimeAssignments` returns the GORM `ON CONFLICT` assignment map for upsert on `UptimeReportAggregateSQL`.

## Motivation
Pinning the uptime data shape at the SW layer guarantees that the Mongo and SQL uptime pumps both see the same record schema — the SQL pump uses `UptimeSQLTable = "tyk_uptime_analytics"` for both `UptimeReportData.TableName()` and `UptimeReportAggregateSQL.TableName()`. The struct carries both granular fields (Day, Month, Year, Hour, Minute as separate ints, mirroring `AnalyticsRecord`) and the underlying `TimeStamp`/`ExpireAt` time.Time — granular fields drive dimension/group-by in dashboards; the time.Time drives TTL on Mongo. Trade-off: `ResponseCode == -1` is overloaded as the "host-checker registered the URL but didn't make a request" sentinel (see the special-case in `AggregateUptimeData` at line 175-187), which is undocumented in the struct itself.

## Code references
- `analytics/uptime_data.go:15 UptimeReportData` — the core struct with all required reporting fields.
- `analytics/uptime_data.go:52-63` — `GetObjectID`, `SetObjectID`, `TableName` (returns `UptimeSQLTable`).
- `analytics/uptime_data.go:33 UptimeReportAggregateSQL` — SQL row with embedded `Counter` and `Code`, GORM composite index on timestamp/org/dimension.
- `analytics/uptime_data.go:47 (*UptimeReportAggregateSQL).TableName` — returns `UptimeSQLTable`.
- `analytics/uptime_data.go:111 UptimeReportAggregate`, `:131 New`, `:97 Dimensions`, `:141 AggregateUptimeData` — the in-memory aggregate plus the per-record fold (status-bucketed counter increments at lines 199-217).
- `analytics/uptime_data.go:67 OnConflictUptimeAssignments` — GORM upsert assignment generator; tracks special-case fields `hits/error/success/total_request_time` (sum), `request_time` (recomputed weighted average), `last_time` (last-write-wins).

## Evidence
- `analytics/uptime_data_test.go:15 TestUptimeReportData_GetObjectID`, `:26 TestUptimeReportData_SetObjectID`, `:36 TestUptimeReportData_TableName`, `:44 TestUptimeReportAggregateSQL_TableName` — struct-API surface, all tagged `// Verifies: SW-REQ-015`.
- `analytics/uptime_data_test.go:52 TestUptimeReportAggregate_New`, `:65 TestUptimeReportAggregate_Dimensions` — aggregate constructors and dimension projection.
- `analytics/uptime_data_test.go:127 TestAggregateUptimeData`, `:343 TestOnConflictUptimeAssignments` — full aggregation and SQL-upsert assignment coverage.
- `analytics/aggregate_mcdc_test.go:86 TestAggregateUptimeData_MCDCBranches` — tagged `// SW-REQ-015:nominal:negative`; MC/DC branch coverage of the aggregation.
- `analytics/coverage2_test.go:59 TestAggregateUptimeData_URLAndErrorBranches` — URL/error branch coverage.

## Open questions
- `ResponseCode == -1` as "registration only, no measurement" sentinel is implicit in `AggregateUptimeData` and not encoded in the struct or req description; a consumer building a custom aggregator will easily miss it and treat -1 as a real status code.
- The `Counter` recomputes `RequestTime = TotalRequestTime / Hits` on every fold (see also SW-REQ-011); for uptime data this means a single oversize request can shift the historical average non-monotonically.
- `Minute` is part of the struct but only `Hour` participates in the aggregate's `TimeID` (see `AggregateUptimeData` line 158); minute-granularity uptime reporting would require a schema change.
