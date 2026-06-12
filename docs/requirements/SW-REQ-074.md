# SW-REQ-074: Prometheus API-key label masking (DEFECT-5 hardening)

> Numbering note: this requirement was first authored under a high
> placeholder id (in the 209 range) during the DEFECT-5 hardening loop and
> renumbered to SW-REQ-074 to close the 074–208 numbering gap (the ids are
> now contiguous 001–074). All trace links, code annotations, MC/DC
> witnesses, and the DEFECT-5 problem report were updated to SW-REQ-074;
> the prior identity is preserved in `lifecycle.change_history`.

## Intent
When `ObfuscateAPIKeys` is enabled, the Prometheus pump shall mask the values
it emits into the `api_key` and `key` metric labels so that at most the last
`ObfuscateAPIKeysLength` characters of the API key are present (prefixed by
`****` for keys longer than that length, otherwise fully masked to `--`), and
the raw API key is never present in any exported metric label. When
`ObfuscateAPIKeys` is disabled, the documented opt-in default applies and the
raw key is emitted.

## Motivation
DEFECT-5 identified that Prometheus metric labels can leak raw API keys into
any scrape target. This requirement is the contract surface for the masking
guarantee (`secrets_not_logged`): it pins the obfuscation behaviour so that the
secret value never reaches an exported label when masking is enabled, while
keeping the disabled path an explicit, regression-pinned operator decision.

## Code references
- `pumps/prometheus.go` `PrometheusMetric.obfuscateAPIKey` — masking
  implementation (`// reqproof:implements SW-REQ-074`).
- Tests: `pumps/prometheus_test.go` `TestPrometheusObfuscateAPIKey`
  (`// Verifies: SW-REQ-074`).

## Evidence
Verified by `TestPrometheusObfuscateAPIKey` with all three MC/DC witness rows
covered:
- `// MCDC SW-REQ-074: api_key_label_masked=F, obfuscate_api_keys_enabled=F => TRUE`
- `// MCDC SW-REQ-074: api_key_label_masked=F, obfuscate_api_keys_enabled=T => FALSE`
  (the violation row: masking enabled but label unmasked must be FALSE)
- `// MCDC SW-REQ-074: api_key_label_masked=T, obfuscate_api_keys_enabled=T => TRUE`

The requirement is assurance level B, approved, and trace-clean
(`approvals_current`, `suspect_clean`). The obligation checklist closes
`secrets_not_logged`.

## Open questions
- If the default of `ObfuscateAPIKeys` ever changes from the documented opt-in
  default, AC-003 and the regression test must be updated together with this
  requirement.
