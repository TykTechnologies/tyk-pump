# SYS-REQ-010: Records exceeding max size are truncated

## Intent
Any record whose serialized size exceeds the operator-configured maximum is truncated (raw request/response payloads are clipped) before forwarding. This boundary obligation satisfies parent **STK-REQ-003**: operators must be able to enforce per-backend size caps to protect downstream sinks with payload limits (e.g. Kafka message size, syslog UDP, SQL `TEXT` columns).

## Motivation
A single mis-sized record can break an entire backend write (Kafka rejects, syslog truncates silently, SQL inserts fail). Capturing the truncation obligation at the SYS layer makes it a black-box property of "the pump" rather than each backend reinventing the size check. The two-level configuration (global `MaxRecordSize` plus per-pump override) gives operators flexibility while keeping the enforcement point centralized in `filterData`.

## Formalization
```
when record_exceeds_max_size privacy shall always satisfy record_truncated
```
The input `record_exceeds_max_size` fires when the record's serialized size > the effective `max_record_size`; the output `record_truncated` becomes true once `TrimRawData(size)` has clipped both `RawRequest` and `RawResponse`. Variables: `specs/system/variables/privacy.vars.yaml`.

## Code references
- `analytics/analytics.go:296 TrimRawData` — trims `RawResponse` then `RawRequest` via `trimString(size, ...)`.
- `analytics/analytics.go:333 trimString` — bounded `bytes.Buffer.Truncate(size)`; returns substring.
- `main.go:379 shouldTrim := SystemConfig.MaxRecordSize != 0 || pump.GetMaxRecordSize() != 0` — gate.
- `main.go:399-406` — per-pump precedence: `pump.GetMaxRecordSize()` first, then global `SystemConfig.MaxRecordSize`.
- `pumps/common.go:70 SetMaxRecordSize` / `:75 GetMaxRecordSize` — per-pump accessor.

## Evidence
- `main_test.go:78 TestTrimData` — exercises both pump-level and global trim configs.
- Satisfying SW child: **SW-REQ-001** (purge loop integration), filter-side trimming is shared common code (no separate SW req).

## Open questions
- "Serialized size" in the FRETish is interpreted in the code as the byte length of `RawRequest` / `RawResponse` strings, not the wire-format size of the whole record. A record with very large `Tags` or `Geo` blocks still passes the gate. The SYS req does not pin the definition.
- Truncation is destructive and silent: there is no log entry per-record. Operators only see the global "trimmed at N" via debug logging in the trim function call path, not in `filterData` itself. This may surprise during incident triage.
- When `pump.GetOmitDetailedRecording()` is true, raw fields are unconditionally cleared and the trim branch is skipped (`main.go:396-407`); the SYS req treats trim and omit as independent but in code they are mutually exclusive in the same iteration.
