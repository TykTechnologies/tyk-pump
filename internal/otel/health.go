package otel

import (
	"context"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
)

// Self-health metrics family. One observable gauge reporting the last result
// of a real probe (Ping) per dependency of the Pump itself: the Redis temporal
// storage and the uptime pump's datastore, never the analytics sinks. The
// label schema is shared with MDCB and the Dashboard: `component` is the
// stable logical role, `store` the engine backing it, so one panel and one
// alert template cover every Tyk component.
const (
	// HealthMetricName is the dependency health gauge; the Collector's
	// Prometheus exporter renders it as tyk_pump_health.
	HealthMetricName = "tyk.pump.health"
	// HealthMetricUnit is empty on purpose. The gauge is dimensionless, but
	// the OTel → Prometheus naming rules turn a gauge with unit "1" into
	// <name>_ratio, which would export it as tyk_pump_health_ratio. No unit
	// keeps the Prometheus name tyk_pump_health, as on MDCB and the Dashboard.
	HealthMetricUnit = ""
	// HealthMetricDescription describes the health gauge.
	HealthMetricDescription = "1 when the dependency's live health probe passes, 0 when it fails"

	// AttrComponent is the label for the dependency role being probed:
	// `temporal_storage` or `uptime`.
	AttrComponent = "component"
	// AttrStore is the label for the engine backing that role: `redis`,
	// `mongo`, `postgres` or `mysql`. Never a host, DSN or database name.
	AttrStore = "store"
)

// HealthSample is one dependency's current health as last observed by the
// health checker. Healthy is the real probe result, not a configuration
// presence proxy.
type HealthSample struct {
	Component string
	Store     string
	Healthy   bool
}

// value is the gauge reading for the sample: 1 healthy, 0 unhealthy.
func (s HealthSample) value() float64 {
	if s.Healthy {
		return 1
	}
	return 0
}

// HealthSnapshot returns the dependencies' latest probe results. It runs once
// per export cycle inside the gauge callback, so it must be bounded by ctx and
// must never block on a hung dependency (internal/healthcheck serves its last
// snapshot instead). An empty result emits no series at all: the gauge is
// absent until a real result exists, never a placeholder.
type HealthSnapshot func(ctx context.Context) []HealthSample

// HealthEnabled reports whether the self-health family is actively exported:
// metrics are exported and health_metrics is not off. RegisterHealthObserver
// is a no-op otherwise, so a disabled family never probes anything.
func (m *MetricInstruments) HealthEnabled() bool {
	return m.Enabled() && m.healthEnabled
}

// RegisterHealthObserver registers the tyk.pump.health gauge; snapshot runs
// once per export cycle and its samples become one data point each, labelled
// with component and store. No-op (returning nil) on a nil receiver or when
// the family is disabled.
func (m *MetricInstruments) RegisterHealthObserver(snapshot HealthSnapshot) error {
	if !m.HealthEnabled() {
		return nil
	}
	_, err := m.provider.NewObservableGauge(HealthMetricName, HealthMetricDescription, HealthMetricUnit,
		func(ctx context.Context, observe tykmetric.Float64Observer) error {
			for _, s := range snapshot(ctx) {
				observe(s.value(),
					tykmetric.StringAttribute(AttrComponent, s.Component),
					tykmetric.StringAttribute(AttrStore, s.Store),
				)
			}
			return nil
		})
	return err
}
