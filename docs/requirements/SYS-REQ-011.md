# SYS-REQ-011: Optional base64 payload decoding per backend

## Intent
When the operator enables `raw_request_decoded` / `raw_response_decoded` for a backend, the pump base64-decodes the raw request and response payloads before forwarding them to that backend. This malformed-input obligation satisfies parent **STK-REQ-003** (operator-configurable behaviour): some downstream sinks (Splunk, SIEM, ad-hoc viewers) need plaintext payloads while others (Kafka producers feeding binary consumers) prefer the original encoded form.

## Motivation
The gateway always emits raw payloads base64-encoded, but downstream readability varies wildly. Capturing per-backend decode as a SYS-level toggle makes the contract explicit: opt-in, per-pump, and decode failure is non-fatal. This also documents that the older global `DecodeRawRequest`/`DecodeRawResponse` flags are now deprecated (see `showDecodeDeprecationWarnings` at `main.go:56`).

## Formalization
```
when decode_enabled privacy shall always satisfy payload_decoded
```
The input `decode_enabled` is true when either `pmp.GetDecodedRequest()` or `pmp.GetDecodedResponse()` returns true for the receiving pump; the output `payload_decoded` becomes true once `base64.StdEncoding.DecodeString` has rewritten the raw field for that record-pump pair. Variables: `specs/system/variables/privacy.vars.yaml`.

## Code references
- `main.go:415-426` — the decode branches inside `filterData`, with `if err == nil` guard so undecodable strings pass through unchanged.
- `pumps/common.go:95 SetDecodingResponse`, `:100 SetDecodingRequest`, `:105 GetDecodedRequest`, `:110 GetDecodedResponse`.
- `main.go:213-214` — `initialisePumps` wires `pmp.DecodeRawRequest` / `pmp.DecodeRawResponse` from the JSON config into each pump.
- `main.go:56 showDecodeDeprecationWarnings` — warns when the deprecated global flags are set.

## Evidence
- `main_test.go:350 TestDecodedKey` — verifies the decode-enabled flow on the `raw_request` / `raw_response` fields.
- Satisfying SW children: none with a dedicated decoding SW req; the SW realization is **SW-REQ-016** (common-pump base setters/getters) plus **SW-REQ-001** (purge loop including `filterData`).

## Open questions
- Decoding failure is silently ignored (`if err == nil { ... }`): undecodable payloads are forwarded raw. The SYS req does not state whether this is intended fallback or a violation. In practice this is intentional (decode is best-effort) but the SYS layer doesn't say so.
- The req is keyed on "decode_enabled" as a single variable but the implementation has two independent toggles (request, response). FRETish coarsens them into one; conformance therefore holds when either toggle drives a decode, but the req does not constrain the *other* toggle.
