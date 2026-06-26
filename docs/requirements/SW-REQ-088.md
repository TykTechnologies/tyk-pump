# SW-REQ-088: Per-pump raw payload decode in filterData

Documents: SW-REQ-088

## Contract

When a pump's per-backend `raw_request_decoded` or `raw_response_decoded`
setting is enabled, `main.filterData` attempts to base64-decode the matching
`AnalyticsRecord.RawRequest` or `AnalyticsRecord.RawResponse` field before
dispatching the record to that pump. Disabled fields remain unchanged.

This is the software decomposition of SYS-REQ-011. SYS-REQ-011 owns the system
privacy behavior; SW-REQ-088 pins the `pump_core` implementation point and the
independence of the request and response toggles.

## Evidence

- `main_test.go:TestDecodedKey` carries the positive MC/DC witness rows for
  enabled and disabled decode settings.
- `main_test.go:TestDecodedKeyInvalidBase64LeavesPayloadsUnchanged` and
  `main_test.go:TestFilterDataBase64DecodeFailurePreservesField` cover
  malformed-base64 reachability as `encoding_aware` / KI evidence. They prove
  the current preserved-original behavior, but they are not satisfied MC/DC
  witnesses for the successful decode formula.

## Known Issues

Malformed base64 decode is still silently ignored. That current behavior is
tracked by `filterdata-base64-decode-silent-noop`; remediation should add an
operator-visible diagnostic or explicit marker rather than silently forwarding
the still-encoded payload.
