# SW-REQ-029: Hybrid pump — forward records to Tyk MDCB over gorpc

## Intent
The hybrid pump shall forward analytics records to a remote Tyk MDCB
(Multi-Data-Centre-Bridge) endpoint using `gorpc` (the TykTechnologies RPC
library), with optional TLS. Records may be sent raw (single
`PurgeAnalyticsData` RPC) or pre-aggregated (`PurgeAnalyticsDataAggregated`,
optionally with a parallel `PurgeAnalyticsDataMCPAggregated` for MCP
records). Derived from SYS-REQ-004 (independent per-backend delivery).

## Motivation
The hybrid pump exists to support Tyk's MDCB topology, where a fleet of
edge gateways collect analytics locally and a central control plane
(MDCB) aggregates them. Without this pump, MDCB sites would have to
double-write analytics to both their local backend and the central
control plane.

The aggregation knobs (`Aggregated`, `StoreAnalyticsPerMinute`,
`TrackAllPaths`, `IgnoreTagPrefixList`) let operators control how much
data crosses the WAN: raw mode forwards every record (bandwidth-heavy,
maximum fidelity); aggregated mode pre-bucketises by orgID/timestamp
locally and sends the aggregates only. The 60-second default aggregation
window with a 1-minute override addresses two common operator preferences
(hourly batches vs near-real-time).

MCP aggregation (`EnableMCPAggregation`) is a separate flag because MCP
records have their own aggregation schema (`AggregateMCPData`); if the
operator hasn't explicitly enabled it, MCP records in aggregated mode are
dropped entirely (documented inline at `hybrid.go:97`).

Failure modes addressed:
- RPC connection loss: `connectAndLogin` retries with exponential backoff
  (`backoff.WithMaxRetries(..., 3)`); `WriteData` re-attempts login on
  pre-write failure and falls through to a full reconnect on persistent
  failure.
- TLS configuration: `UseSSL` + `SSLInsecureSkipVerify` operator-
  configurable (note the `// #nosec G402` annotations acknowledge this is
  intentional).
- Per-RPC timeout: every `callRPCFn` uses `CallTimeout` (default 10s).

## Code references
- `pumps/hybrid.go:54-64` — `HybridPump` struct with `gorpc.Client`,
  `gorpc.Dispatcher`, atomic connection flag.
- `pumps/hybrid.go:67-103` — `HybridPumpConf` (connection_string, RPC key,
  API key, aggregation flags, MCP aggregation, TLS).
- `pumps/hybrid.go:25-41` — `dispatcherFuncs` declares the four RPC
  signatures (`Login`, `PurgeAnalyticsData`,
  `PurgeAnalyticsDataAggregated`, `PurgeAnalyticsDataMCPAggregated`, plus
  `Ping`).
- `pumps/hybrid.go:174-216` — `connectRPC` uses `gorpc.NewTLSClient` when
  `UseSSL` or `gorpc.NewTCPClient` otherwise.
- `pumps/hybrid.go:233-267` — `getDialFn` constructs the dial function
  with TLS + connID handshake.
- `pumps/hybrid.go:270-336` — `WriteData`: raw vs aggregated path; calls
  `sendMCPAggregates` when both `Aggregated` and `EnableMCPAggregation`.
- `pumps/hybrid.go:352-370` — `RPCLogin` returns `ErrRPCLogin` on
  authentication failure.
- `pumps/hybrid.go:375-393` — `sendMCPAggregates` skips when no MCP
  records present.
- `pumps/hybrid.go:397-422` — `connectAndLogin` wraps retry logic.

## Evidence
- `pumps/hybrid_test.go` covers config defaults, dispatcher registration,
  aggregation paths, and reconnect logic with a fake gorpc server.

## Open questions
- The hybrid pump is the only pump that distinguishes a "login" failure
  (`ErrRPCLogin`) — a hard authentication error that should not be
  retried indefinitely — from a transient connection error. The
  distinction isn't captured in any requirement.
- `EnableMCPAggregation` interaction with `Aggregated` is non-obvious:
  when `Aggregated` is false the MCP flag is *ignored entirely* and MCP
  records flow as raw via `PurgeAnalyticsData`. The inline comment
  documents this; the requirement does not.
- TLS verification is operator-configurable via `SSLInsecureSkipVerify`;
  same TLS-policy concern as SW-REQ-027.
- The retry policy uses `backoff.WithMaxRetries(..., 3)` only inside
  `retryAndLog` — `WriteData`'s own login retry path is hand-rolled and
  not bounded by the same mechanism. Phase A should consolidate.
- Shutdown stops the `gorpc.Client` but does not flush any in-flight
  RPCs; on a fast shutdown a record-batch could be lost.
