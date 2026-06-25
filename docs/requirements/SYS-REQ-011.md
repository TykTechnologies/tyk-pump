# SYS-REQ-011: Optional base64 payload decoding per backend

## Intent
When the operator enables `raw_request_decoded` / `raw_response_decoded` for a backend, the pump base64-decodes the raw request and response payloads before forwarding them to that backend. This malformed-input obligation satisfies parent **STK-REQ-003** (operator-configurable behaviour): some downstream sinks (Splunk, SIEM, ad-hoc viewers) need plaintext payloads while others (Kafka producers feeding binary consumers) prefer the original encoded form.

## Motivation
The gateway always emits raw payloads base64-encoded, but downstream readability varies wildly. Capturing per-backend decode as a SYS-level toggle makes the contract explicit: opt-in, per-pump, and decode failure is non-fatal. This also documents that the older global `DecodeRawRequest`/`DecodeRawResponse` flags are now deprecated (see `showDecodeDeprecationWarnings` at `main.go:56`).

## Formalization
```
when decode_request_enabled | decode_response_enabled privacy shall always satisfy enabled_payloads_decoded
```
The inputs `decode_request_enabled` and `decode_response_enabled` are true when
the receiving pump's per-backend request or response decode toggle is enabled;
the output `enabled_payloads_decoded` becomes true once each enabled raw field
has been rewritten by `base64.StdEncoding.DecodeString` for that record-pump
pair. Variables: `specs/system/variables/privacy.vars.yaml`.

## Code references
- `main.go:415-426` — the decode branches inside `filterData`, with `if err == nil` guard so undecodable strings pass through unchanged.
- `pumps/common.go:95 SetDecodingResponse`, `:100 SetDecodingRequest`, `:105 GetDecodedRequest`, `:110 GetDecodedResponse`.
- `main.go:213-214` — `initialisePumps` wires `pmp.DecodeRawRequest` / `pmp.DecodeRawResponse` from the JSON config into each pump.
- `main.go:56 showDecodeDeprecationWarnings` — warns when the deprecated global flags are set.

## Evidence
- `main_test.go:TestDecodedKey` verifies the decode-enabled flow on the
  `raw_request` / `raw_response` fields.
- SW-REQ-088 decomposes the `main.filterData` software behavior and carries
  the per-pump request/response toggle evidence.
- SW-REQ-016 covers the common-pump setters/getters used to store those
  per-pump flags.

## Open questions
- Decoding failure is silently ignored (`if err == nil { ... }`): undecodable
  payloads are forwarded raw. This is tracked by KnownIssue
  `filterdata-base64-decode-silent-noop`.
