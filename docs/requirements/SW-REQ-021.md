# SW-REQ-021: Kafka pump — JSON producer with optional SASL and TLS

## Intent
The Kafka pump shall serialise each analytics record to a JSON message and
produce it to the configured topic using `segmentio/kafka-go`. The pump shall
support SASL/PLAIN and SASL/SCRAM (SHA-256 and SHA-512) authentication and
operator-configurable TLS via `UseSSL` + cert/key/CA/skip-verify. Derived
from SYS-REQ-004 (independent per-backend delivery).

## Motivation
Kafka is the canonical sink for streaming analytics into downstream
warehouses, lakehouses, and real-time consumers. The choice of JSON over a
binary format (Avro/Protobuf) trades schema discipline for trivial consumer
implementation and ad-hoc tooling friendliness — appropriate for analytics
events that are sampled and inspected far more than they are validated.
Embedding static metadata (`MetaData map[string]string`) per message lets
operators tag events with cluster/region/environment without changing the
analytics schema.

SASL/PLAIN and SASL/SCRAM cover essentially every managed-Kafka offering
(Confluent Cloud, AWS MSK, Aiven). Snappy compression is exposed via
`Compressed` for bandwidth-constrained links. The `BatchBytes` knob caps
request size — important because Kafka brokers reject oversized requests
with cryptic errors. The pump uses `LeastBytes` partition balancing by
default, distributing load evenly without operator intervention.

Batch byte writer-budget behavior is split into **SW-REQ-106** so positive,
zero, omitted, invalid, negative, and env-overridden `batch_bytes` values have
direct evidence against the concrete `kafka.WriterConfig.BatchBytes` field.

Failure mode addressed: per-write connection churn. The pump constructs a
fresh `kafka.Writer` per `WriteData` call (`kafka.go:252-255`) and closes it
immediately — this is wasteful but matches the per-purge-cycle batching
model and avoids cross-batch connection state.

## Code references
- `pumps/kafka.go:23-28` — `KafkaPump` struct.
- `pumps/kafka.go:36-77` — `KafkaConf` (broker list, SASL knobs, TLS knobs,
  metadata, batch-bytes).
- `pumps/kafka.go:112-122` — `UseSSL` path delegates to `NewTLSConfig`.
- `pumps/kafka.go:127-146` — SASL mechanism switch (PLAIN, SCRAM SHA-256,
  SCRAM SHA-512); warns on unknown mechanism.
- `pumps/kafka.go:148-187` — timeout parsing (string or float), dialer setup,
  `LeastBytes` balancer, optional Snappy compression, and `BatchBytes`
  application.
- `pumps/kafka.go:195-249` — `WriteData` builds JSON messages with static
  metadata merged in.
- `pumps/kafka.go:252-256` — `write` creates a writer per call and closes it.

## Evidence
- `pumps/kafka_test.go` covers config decode and message construction.
- `pumps/kafka_test.go:TestKafkaPump_Init_BatchBytesConfiguration`,
  `TestKafkaPump_BatchBytesEnvironmentVariableOverride`,
  `TestKafkaPump_BatchBytesEnvironmentVariableInvalid`,
  `TestKafkaPump_BatchBytesConfigAndEnvironmentVariableBothInvalid`,
  `TestKafkaPump_Init_NegativeBatchBytes`, and
  `TestKafkaPump_WriterConfigIntegrity` cover SW-REQ-106.
- A live broker is required for end-to-end coverage; those tests are excluded
  from the local audit MC/DC scope (recorded as a known issue).

## Open questions
- `WriteData` logs `kafkaError` but returns `nil` — the caller never learns
  of a partial or total write failure. The requirement is satisfied if "shall
  produce" means "shall attempt", but the production-readiness implication is
  worth a Phase-A explicit obligation.
- The SASL configuration warns (but does not fail) when `SASLMechanism` is
  set without `UseSSL` — credentials sent in cleartext. Phase A could capture
  the safety obligation explicitly.
- The pump always sets `WriteTimeout = ReadTimeout = timeout` — no separate
  knobs. The single-timeout assumption is not in the requirement text.
- Creating a fresh `kafka.Writer` per `WriteData` defeats kafka-go's internal
  connection pooling and batching; performance impact at high purge frequency
  is not captured.
