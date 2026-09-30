// Package otel is the Pump's thin, metrics-only wrapper around the shared
// github.com/TykTechnologies/opentelemetry library. It mirrors the Gateway's,
// MDCB's and the Dashboard's internal/otel packages so operators configure
// every Tyk component the same way and every exported metric carries the same
// identity.
//
// It instruments the Pump process itself, not the analytics records the Pump
// moves. All metric names, label keys and attribute construction live in this
// package; callers pass plain values.
//
// This package must never import the root main package or pumps: the root
// package embeds otel.OpenTelemetry in its configuration, so an import here
// would create a cycle.
package otel

import (
	otelconfig "github.com/TykTechnologies/opentelemetry/config"
)

// Type aliases so consumers of this package never import the shared library's
// config package directly.
type (
	// BaseMetricsConfig is the shared library's OTLP metrics exporter
	// configuration; the MetricsConfig wrapper embeds it.
	BaseMetricsConfig = otelconfig.MetricsConfig
	// ExporterConfig holds the shared transport fields (exporter, endpoint,
	// headers, connection timeout, resource name, TLS).
	ExporterConfig = otelconfig.ExporterConfig
	// MetricsRetryConfig configures retry behaviour for metric export failures.
	MetricsRetryConfig = otelconfig.MetricsRetryConfig
	// TLS is the exporter TLS configuration.
	TLS = otelconfig.TLS
)

const (
	// DefaultResourceName is the default service.name resource attribute for
	// the Pump. The shared library would otherwise default it to "tyk".
	DefaultResourceName = "tyk-pump"
	// DefaultDeploymentEnvironment is used when deployment_environment is not
	// configured, matching MDCB and the Dashboard.
	DefaultDeploymentEnvironment = "unknown"
)

// MetricsConfig wraps the shared library's metrics config and adds
// Pump-specific fields, mirroring the Gateway's, MDCB's and the Dashboard's
// `opentelemetry.metrics` block. Per-family toggles are added here by the
// stories that introduce each family, and resolve through familyEnabled.
//
// Field names deliberately mirror the Gateway so that every derived
// environment variable is the Gateway's name with the TYK_GW_ prefix replaced
// by TYK_PMP_ (e.g. TYK_PMP_OPENTELEMETRY_METRICS_ENABLED). Do not rename
// fields: environment variable names derive from them.
//
//nolint:govet // field order mirrors the Gateway's block, not memory layout.
type MetricsConfig struct {
	// The shared OTLP metrics exporter settings: `enabled`, `exporter`,
	// `endpoint`, `headers`, `connection_timeout`, `resource_name`, `tls`,
	// `export_interval`, `temporality`, `shutdown_timeout`, `retry` and
	// `cardinality_limit`. They are the same keys the Gateway, MDCB and the
	// Dashboard use. `resource_name` defaults to `tyk-pump`.
	BaseMetricsConfig `json:",inline"`

	// DeploymentEnvironment is the deployment environment name (for example
	// `production` or `staging`), exported as the `deployment.environment`
	// resource attribute on every metric. Defaults to `unknown`.
	DeploymentEnvironment string `json:"deployment_environment"`
}

// OpenTelemetry is the Pump's `opentelemetry` configuration block. Like MDCB
// and the Dashboard there are no trace fields: the Pump is metrics-only.
type OpenTelemetry struct {
	// Metrics holds the OTLP metrics exporter configuration
	// (`opentelemetry.metrics`). Metrics are disabled unless
	// `opentelemetry.metrics.enabled` is explicitly set to `true`.
	// Exporter defaults to `grpc`, endpoint to `localhost:4317` and
	// export_interval to 60 seconds.
	Metrics MetricsConfig `json:"metrics"`
}

// SetDefaults fills zero-valued fields with Pump defaults. It never enables
// metrics: `enabled` has no default and stays off unless explicitly true.
// Negative durations and limits are treated as unset so the library defaults
// apply instead of producing an already-expired context or a zero export rate.
func (c *OpenTelemetry) SetDefaults() {
	if c.Metrics.DeploymentEnvironment == "" {
		c.Metrics.DeploymentEnvironment = DefaultDeploymentEnvironment
	}

	if c.Metrics.ResourceName == "" {
		c.Metrics.ResourceName = DefaultResourceName
	}

	for _, v := range []*int{
		&c.Metrics.ConnectionTimeout,
		&c.Metrics.ExportInterval,
		&c.Metrics.ShutdownTimeout,
		&c.Metrics.CardinalityLimit,
	} {
		if *v < 0 {
			*v = 0
		}
	}

	c.Metrics.SetDefaults()
}

// MetricsEnabled reports whether metrics export is explicitly enabled.
func (c *OpenTelemetry) MetricsEnabled() bool {
	return c.Metrics.Enabled != nil && *c.Metrics.Enabled
}

// familyEnabled resolves one per-family toggle. Every family follows the same
// three-state contract, stated once here: the family is on when metrics
// export is on and the toggle is not explicitly `false`. A nil toggle means
// "unset", which means on, unlike the top-level `enabled` which defaults to
// off; and a global metrics-off always wins over a family `true`.
//
// process.uptime has no toggle: it is always on while metrics are on. The
// first family toggle arrives with a later story; until then only the tests
// call this.
func (c *OpenTelemetry) familyEnabled(toggle *bool) bool {
	return c.MetricsEnabled() && (toggle == nil || *toggle)
}
