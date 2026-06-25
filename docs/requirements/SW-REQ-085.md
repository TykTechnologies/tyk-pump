# SW-REQ-085: Hybrid init recoverable failures return errors

Documents: SW-REQ-085

## Contract

HybridPump.Init must return recoverable startup failures to its caller instead
of terminating the tyk-pump process. Recoverable failures include a missing
`connection_string`, an unreachable MDCB endpoint, and invalid RPC credentials.

This requirement is a child of SW-REQ-029. SW-REQ-029 keeps the broader Hybrid
MDCB forwarding contract; SW-REQ-085 pins the failure-isolation behavior fixed
by TT-8313.

## Evidence

- `pumps/hybrid_test.go:TestHybridPumpInit` covers missing connection string,
  invalid credentials, and successful initialization.
- `pumps/hybrid_test.go:TestConnectAndLogin` covers server-down connection
  attempts returning errors.

## Known Issues

This requirement does not close Hybrid's remaining connection-leak,
retry-deadline, SSRF, TLS-skip, or process-wide pump-timeout risks. Those remain
tracked under the KnownIssues linked from DEFECT-18 and SW-REQ-029.
