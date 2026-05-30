# SYS-REQ-015: Omit detailed recording drops raw payloads

## Intent
When the operator enables `omit_detailed_recording` for a backend, the pump drops the raw request and response payloads from each analytics record before forwarding to that backend. This satisfies parent **STK-REQ-003** (operator-configurable behaviour) as a privacy / data-minimization control: some backends should never see request bodies.

## Motivation
Compliance regimes (PCI, HIPAA, internal data-classification policies) frequently require that analytics destined for long-term storage or third-party SaaS not include raw payloads — those should only flow to a single hardened sink, if anywhere. Capturing omit-detailed-recording at the SYS layer formalizes this as a primary privacy lever, separate from `ignore_fields` (which is a more general field-stripping mechanism) and from `max_record_size` (which is a quantitative cap, not a categorical drop).

## Formalization
```
when omit_detailed_recording_enabled privacy shall always satisfy detailed_payloads_omitted
```
The input `omit_detailed_recording_enabled` is true when `pmp.GetOmitDetailedRecording()` returns true; the output `detailed_payloads_omitted` becomes true once `decoded.RawRequest = ""` and `decoded.RawResponse = ""` have run inside `filterData`. Variables: `specs/system/variables/privacy.vars.yaml`.

## Code references
- `main.go:396-398` — `if pump.GetOmitDetailedRecording() { decoded.RawRequest = ""; decoded.RawResponse = "" }` (inside `filterData`).
- `pumps/common.go:50 SetOmitDetailedRecording` / `:55 GetOmitDetailedRecording` — per-pump accessor.
- `main.go:210 thisPmp.SetOmitDetailedRecording(pmp.OmitDetailedRecording)` — wired from JSON config.
- `analytics/analytics.go:67-68` — `RawRequest` and `RawResponse` fields cleared.

## Evidence
- `main_test.go:145 TestOmitDetailsFilterData` — verifies that raw fields are emptied per pump.
- Satisfying SW child: **SW-REQ-016** (common-pump setters/getters); the privacy obligation rides on the per-pump filter pipeline.

## Open questions
- Omit-detailed-recording short-circuits `max_record_size` for this record path: when omit is on, the `shouldTrim` branch is skipped (`main.go:399-407` `else` clause), which is correct (no payload left to trim) but not explicit in the SYS req.
- A global `SystemConfig.OmitDetailedRecording` exists and is passed to `StartPurgeLoop` (`main.go:536`), but it is *not* applied in `filterData`; only the per-pump flag is honoured. The SYS req says "per backend" which matches the implementation, but operators reading the global config field may expect a global drop — this is a latent surprise.
