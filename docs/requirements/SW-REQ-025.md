# SW-REQ-025: CSV pump — append analytics records to hourly CSV files

## Intent
The CSV pump shall append analytics records to a CSV file named
`<year>-<month>-<day>-<hour>.csv` under the configured `CSVDir`, creating a
new file (with header row) when no file exists for the current
year/month/day/hour combination and appending otherwise. This produces one
file per wall-clock hour. Derived from SYS-REQ-004 (independent per-backend
delivery).

## Motivation
CSV is the lowest-common-denominator analytics sink: trivial for spreadsheet
or pandas-based ad-hoc analysis, no infrastructure needed, no client library
dependency. The hourly file boundary is a balance: smaller granularity
(per-minute) creates filesystem inode pressure; larger granularity (per-day)
produces files too large for naive tooling.

The filename format `%d-%s-%d-%d.csv` (year, month name, day, hour) is
human-sortable within a day but not across days because the month is
formatted as a name (`January`, `February`) — operators relying on
`ls`-order to navigate by time will need a sort wrapper. This is a
known ergonomic wart, but changing the format would break any
downstream that hard-codes file globs.

The first write per file injects a header row derived from
`AnalyticsRecord.GetFieldNames()`. Subsequent writes within the same hour
append without rewriting the header. There is no rotation, compression, or
cleanup — operators are expected to manage retention out-of-band.

Failure modes addressed:
- Missing directory: `Init` calls `os.MkdirAll(..., 0777)` to create the
  directory tree.
- Concurrent writes: each batch opens-appends-closes within a single
  `WriteData` call; the pump assumes the core serialises writes per pump.
- Non-`AnalyticsRecord` items: returns an explicit error rather than
  silently skipping.

## Code references
- `pumps/csv.go:16-20` — `CSVPump` struct.
- `pumps/csv.go:23-29` — `CSVConf` (only `CSVDir`).
- `pumps/csv.go:51-69` — `Init` creates the directory (0777, mode chosen
  intentionally to allow shared multi-user access).
- `pumps/csv.go:72-141` — `WriteData`. Note `fname` format on line 76:
  `fmt.Sprintf("%d-%s-%d-%d.csv", curtime.Year(), curtime.Month().String(),
  curtime.Day(), curtime.Hour())` — this produces genuinely hourly files
  (Phase 0 verification confirmed this against the original "per-day"
  description).
- `pumps/csv.go:82-95` — file create-vs-append decision based on
  `os.Stat` result.
- `pumps/csv.go:100-110` — header row written only on file create.

## Evidence
- `pumps/csv_test.go` covers file creation, header injection, and append
  behaviour using a temp directory.

## Open questions
- The filename format embeds the month *name* (`time.Month().String()` —
  "January", "February"), so files don't sort chronologically across months
  with a plain `ls`. Worth flagging but probably not worth breaking
  back-compat to change.
- File mode `0777` on `os.MkdirAll` is permissive; if tyk-pump runs as a
  service user this is fine, but on shared hosts it exposes data via
  umask leakage.
- `outfile.Close()` is deferred even when the open failed (`outfile` is
  nil) — the deferred call on a nil `*os.File` panics. The existing code
  logs errors but does not return, so this is reachable on a permission
  failure. Phase A should capture an error-handling obligation.
- There's no rotation, compression, or retention policy; pump assumes the
  operator runs `find … -delete` or similar.
- Concurrent writers (e.g. two pump processes pointed at the same dir)
  would corrupt the header logic because the per-file create check is not
  atomic with the open.
