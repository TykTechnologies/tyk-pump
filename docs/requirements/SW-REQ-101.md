# SW-REQ-101: Influx v1 writes one batch per purge

Documents: SW-REQ-101

## Contract

`InfluxPump.WriteData` must add all accepted analytics records from the current
purge to one InfluxDB v1 `BatchPoints` value and call the v1 client's `Write`
method exactly once after the record loop. For an N-record purge, the backend
must see one write request with N line-protocol points, not N cumulative write
requests containing 1, 2, ..., N points.

This is a child of SW-REQ-046. SW-REQ-046 owns the broader Influx v1 client,
field/tag projection, and write-path behavior; SW-REQ-101 pins the historical
output-cardinality failure fixed by commit `51af27d`.

## Evidence

- `pumps/http_pumps_mcdc_test.go:TestInfluxPump_WriteData_RoundTrip` sends a
  three-record purge to an `httptest` Influx endpoint, asserts exactly one
  `/write` request, and asserts the request body contains exactly three
  line-protocol rows.

## Known Issues

This requirement does not claim Influx v1 write errors are propagated, nor that
reconnects are bounded. Those behaviors remain tracked by
`pump-writedata-swallows-per-batch-errors`,
`influx-v1-unbounded-reconnect-recursion`, and related SW-REQ-046 deferrals.
