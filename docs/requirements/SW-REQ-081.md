# SW-REQ-081: Kafka timeout config units preserved

The Kafka pump accepts timeout configuration through generic pump config and
the `TYK_PMP_PUMPS_KAFKA_META_TIMEOUT` environment override. TT-9360 fixed a
historical unit bug where the timeout field crossed `mapstructure`/env parsing
as a `time.Duration`-shaped value and then got multiplied by `time.Second` at
the Kafka client boundary.

## Contract

When a Kafka timeout is configured, `KafkaPump.Init` shall parse the supported
operator representations and apply the same duration to all Kafka client
deadline fields:

- `kafka.Dialer.Timeout`
- `kafka.WriterConfig.WriteTimeout`
- `kafka.WriterConfig.ReadTimeout`

Supported representations currently covered by tests are duration strings such
as `"10s"`, numeric strings such as `"5"` interpreted as seconds, numeric
values such as `7.0` interpreted as seconds, and the
`TYK_PMP_PUMPS_KAFKA_META_TIMEOUT` override.

## Evidence

- `pumps/kafka_mcdc_test.go:TestKafkaPump_Init_TimeoutVariants` covers duration
  strings, numeric strings, and numeric values.
- `pumps/kafka_mcdc_test.go:TestKafkaPump_Init_TimeoutEnvOverride` covers the
  Kafka-specific environment override path.
- `.proof/catalog/domain/timeout_config_units_preserved.yaml` captures the
  reusable obligation class.

## Known Issue

Malformed Kafka timeout input still calls `log.Fatal` instead of returning an
error to the caller. That behavior is tracked separately by
`kafka-logfatal-on-init-mech-and-timeout` and is not claimed fixed by this
requirement.
