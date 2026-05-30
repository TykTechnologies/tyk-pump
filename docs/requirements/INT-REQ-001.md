# INT-REQ-001: Gateway/pump analytics-key publish-consume contract

## Intent
This is the cross-component wire contract for the request-analytics stream
between Tyk Gateway (producer) and tyk-pump (consumer): the gateway pushes
serialized analytics records onto the temporal store under a well-known set
of list keys, and the pump pops them off, decodes them, and forwards them
downstream. The requirement also asserts that untrusted input must be
bounded — the pump must not let an unbounded list, an oversized record or a
malformed payload destabilise its purge loop. It satisfies SYS-REQ-001.

## Motivation
The pump and the gateway are different processes (often different repos,
different release cadences). Without an explicit naming and bounding
contract, the gateway could rename a key in a minor release and the pump
would silently stop draining — exactly the dashboard-goes-blank failure
mode the stakeholder reqs flag. Naming the keys (and the convention for
multiple keys when `enable_multiple_analytics_keys` is on at the gateway)
in this INT req keeps the contract explicit and reviewable.

Bounding untrusted input is the second half of the contract: the pump runs
with elevated trust in its target databases, and a malicious or buggy
producer could otherwise weaponise the pump as an amplifier. Today this is
realised as chunked reads, per-pump max record size, and a
decode-error-skip path that does not abort the purge cycle.

## Code references
Wire-format constants and call sites:
- `storage/store.go:18` `ANALYTICS_KEYNAME = "tyk-system-analytics"` — the
  base key.
- `main.go:267-274` — the consumer iterates `i = -1..9`, producing
  `tyk-system-analytics` (i == -1, for the single-key backwards-compatible
  case) and `tyk-system-analytics_0` .. `tyk-system-analytics_9` (when the
  gateway is sharding analytics keys).
- `main.go:276-277` — for each base key, the consumer appends the
  serializer suffix (e.g. `_protobuf`) before reading.
- `storage/temporal_storage.go:262` `GetAndDeleteSet` — the actual `LPOP n`
  + `EXPIRE` pop.
- Bounding: chunk size is the `chunkSize int64` parameter into
  `GetAndDeleteSet`; per-record max size is enforced in `filterData` at
  `main.go:400-406` via `decoded.TrimRawData(...)`; malformed payloads are
  skipped per-record at `main.go:320-326` (decode error logged, record
  dropped, loop continues).

## Evidence
- `storage/temporal_storage_test.go` and
  `storage/temporal_storage_negative_test.go` exercise `GetAndDeleteSet`
  including the empty-key path.
- `serializer/serializer_test.go` exercises malformed-input decode error
  surfaces.
- `main_test.go` integration tests cover the multi-key iteration.

## Open questions
- The 0..9 shard count is hard-coded at `main.go:267`. If the gateway is
  reconfigured to use more than 10 analytics keys (`enable_multiple_analytics_keys`
  with a custom modulus), the pump will silently fail to drain the higher
  shards. The contract does not name this constant.
- The contract is silent on a maximum-list-length safeguard: if the gateway
  produces records faster than the pump can drain, the list grows until
  Redis OOMs. `chunkSize` bounds *per-pop*, not *per-list*.
