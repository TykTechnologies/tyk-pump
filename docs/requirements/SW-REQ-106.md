# SW-REQ-106: Kafka batch byte writer budget

## Intent
When `batch_bytes` is configured, Kafka Init must apply the value to
`kafka.WriterConfig.BatchBytes` exactly for non-negative integers.

The accepted behavior is:

- Positive values are copied into `WriterConfig.BatchBytes`.
- Zero and omitted values leave `WriterConfig.BatchBytes` at `0`, which is the
  kafka-go default sentinel.
- `TYK_PMP_PUMPS_KAFKA_META_BATCHBYTES` overrides file/config values through
  the standard pump env overlay.
- Non-integer config values fail `mapstructure.Decode` before writer-config
  application; the current `KafkaPump.Init` process-exit behavior for decode
  failures remains tracked by known issue `pumps-logfatal-on-config-decode`.
- Invalid env override values do not replace a valid file/config value.
- Negative values do not populate `WriterConfig.BatchBytes`.
- Other writer settings, including brokers, topic, balancer, compression, and
  dialer, remain intact when `batch_bytes` is applied.

## Motivation
TT-15560 added `batch_bytes` so operators can control Kafka request sizing
without changing topic, broker, compression, or timeout behavior. This child
requirement keeps that writer-budget contract explicit instead of burying it in
the broader Kafka pump requirement.

## Formalization
```
when kafka_batch_bytes_configured pumps_kafka shall always satisfy kafka_batch_bytes_applied
```

Variables are declared in `specs/software/variables/pumps-kafka.vars.yaml`.

## Code References
- `pumps/kafka.go:KafkaConf.BatchBytes` decodes the `batch_bytes` field.
- `pumps/kafka.go:KafkaPump.Init` applies non-negative values to
  `kafka.WriterConfig.BatchBytes` after other writer settings are initialized.

## Evidence
- `pumps/kafka_test.go:TestKafkaPump_Init_BatchBytesConfiguration` covers
  positive, zero, omitted, and large values.
- `pumps/kafka_test.go:TestKafkaPump_BatchBytesEnvironmentVariableOverride`
  covers env override.
- `pumps/kafka_test.go:TestKafkaPump_BatchBytesEnvironmentVariableInvalid`
  covers invalid env fallback to the file/config value.
- `pumps/kafka_test.go:TestKafkaPump_BatchBytesConfigAndEnvironmentVariableBothInvalid`
  covers decode rejection for non-integer config.
- `pumps/kafka_test.go:TestKafkaPump_Init_NegativeBatchBytes` covers negative
  value handling.
- `pumps/kafka_test.go:TestKafkaPump_WriterConfigIntegrity` covers sibling
  writer settings.
