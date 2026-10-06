package otel

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
)

// Aliases so consumers of this package never import the shared library.
type (
	// MetricsProvider is the shared library's meter provider interface.
	MetricsProvider = tykmetric.Provider
	// Attribute is an OpenTelemetry attribute key/value pair.
	Attribute = tykmetric.Attribute
)

const (
	// UptimeMetricName is the liveness gauge registered at provider init. It
	// doubles as an end-to-end proof that the OTLP pipeline works: every
	// export cycle carries at least one data point.
	UptimeMetricName = "process.uptime"
	// UptimeMetricUnit is seconds.
	UptimeMetricUnit = "s"
	// UptimeMetricDescription describes the liveness gauge.
	UptimeMetricDescription = "Time the process has been running"
)

// MetricInstruments encapsulates the OTel metrics provider and every Pump
// instrument, mirroring the Gateway's and the Dashboard's
// internal/otel.MetricInstruments. Signal stories add their instruments as
// fields created in NewMetricInstruments and expose them through Record*
// methods; call sites never see the library.
//
// Every method is safe to call on a nil receiver, on the zero value (no
// provider) and with a disabled provider: the library's instruments are
// nil-safe no-ops, so recording never needs an Enabled() check at the call
// site.
type MetricInstruments struct {
	provider MetricsProvider
	logger   logrus.FieldLogger

	// uptime is process.uptime: an observable gauge sampled by callback on
	// every export cycle, so it costs nothing between exports.
	uptime *tykmetric.ObservableGauge

	// healthEnabled records whether the self-health family is on
	// (opentelemetry.metrics.health_metrics not explicitly false). The gauge
	// itself is registered by RegisterHealthObserver once the dependencies
	// it probes exist.
	healthEnabled bool
}

// NewMetricInstruments creates the Pump instruments from an initialized
// provider. cfg carries the per-family toggles deciding which instruments are
// registered. The provider may be a noop provider; instruments created from it
// are safe no-ops. Instrument creation errors are logged, never returned: the
// affected instrument becomes a no-op and boot continues.
//
//nolint:gocritic // cfg mirrors InitMetrics, which takes it by value.
func NewMetricInstruments(provider MetricsProvider, logger logrus.FieldLogger, cfg OpenTelemetry) *MetricInstruments {
	m := &MetricInstruments{provider: provider, logger: logger, healthEnabled: cfg.HealthMetricsEnabled()}
	if provider == nil {
		return m
	}

	start := time.Now()
	uptime, err := provider.NewObservableGauge(UptimeMetricName, UptimeMetricDescription, UptimeMetricUnit,
		func(_ context.Context, observe tykmetric.Float64Observer) error {
			observe(time.Since(start).Seconds())
			return nil
		})
	if err != nil {
		logger.WithError(err).Errorf("Creating %s gauge; it will be a no-op", UptimeMetricName)
	}
	m.uptime = uptime

	return m
}

// Enabled reports whether metrics are actively recorded and exported.
func (m *MetricInstruments) Enabled() bool {
	return m != nil && m.provider != nil && m.provider.Enabled()
}

// Provider returns the underlying provider, or nil on a nil/empty container.
func (m *MetricInstruments) Provider() MetricsProvider {
	if m == nil {
		return nil
	}
	return m.provider
}

// Shutdown flushes pending metrics (ForceFlush) then stops the provider.
// Safe on a nil receiver or a nil/noop provider.
func (m *MetricInstruments) Shutdown(ctx context.Context) error {
	if m == nil || m.provider == nil {
		return nil
	}
	// Always attempt Shutdown even if ForceFlush fails (e.g. an unreachable
	// collector), otherwise the provider's periodic-reader goroutine leaks.
	flushErr := m.provider.ForceFlush(ctx)
	shutdownErr := m.provider.Shutdown(ctx)
	return errors.Join(flushErr, shutdownErr)
}
