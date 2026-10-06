package otel_test

import (
	"context"
	"sync/atomic"
	"testing"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
	"github.com/TykTechnologies/opentelemetry/metric/metrictest"

	"github.com/TykTechnologies/tyk-pump/internal/otel"
)

func boolPtr(b bool) *bool { return &b }

// confWithHealth returns an enabled metrics config with the health_metrics
// toggle in the given state (nil = unset).
func confWithHealth(health *bool) otel.OpenTelemetry {
	c := enabledConf()
	c.Metrics.HealthMetrics = health
	return c
}

// bothSamples is the full set of series a Pump with an SQL uptime pump emits,
// with Redis down.
var bothSamples = []otel.HealthSample{
	{Component: "temporal_storage", Store: "redis", Healthy: false},
	{Component: "uptime", Store: "postgres", Healthy: true},
}

func TestHealthMetricsEnabled_ThreeState(t *testing.T) {
	c := confWithHealth(nil)
	assert.True(t, c.HealthMetricsEnabled(), "health_metrics unset must default to enabled")

	c = confWithHealth(boolPtr(true))
	assert.True(t, c.HealthMetricsEnabled())

	c = confWithHealth(boolPtr(false))
	assert.False(t, c.HealthMetricsEnabled(), "explicit false must disable the family")

	var off otel.OpenTelemetry // metrics export disabled entirely
	assert.False(t, off.HealthMetricsEnabled(), "family must be off when metrics export is off")
	off.Metrics.HealthMetrics = boolPtr(true)
	assert.False(t, off.HealthMetricsEnabled(), "health_metrics=true must not override a disabled exporter")
}

// TestRegisterHealthObserver_SeriesAndLabels asserts one data point per
// sample, the 1/0 mapping, the component/store label pair, the unit-less
// registration and that every export re-reads the snapshot. That no other
// label is added is asserted end to end by the e2e TestHealth_Healthy.
func TestRegisterHealthObserver_SeriesAndLabels(t *testing.T) {
	logger, _ := logrustest.NewNullLogger()
	rec := metrictest.NewRecorder(t)
	m := otel.InitMetrics(context.Background(), logger, confWithHealth(nil), testIdentity(), rec.Option())
	require.True(t, m.Enabled())
	assert.True(t, m.HealthEnabled(), "health family must be on by default when metrics are enabled")

	samples := bothSamples
	var calls atomic.Int32
	require.NoError(t, m.RegisterHealthObserver(func(context.Context) []otel.HealthSample {
		calls.Add(1)
		return samples
	}))

	metric := rec.FindMetric(t, otel.HealthMetricName)
	assert.Empty(t, metric.Unit, "a unit would make Prometheus append _ratio")
	assert.Equal(t, otel.HealthMetricDescription, metric.Description)
	metrictest.AssertDataPointCount(t, metric, len(bothSamples))
	for _, s := range bothSamples {
		metrictest.AssertHasAttributes(t, metric,
			tykmetric.StringAttribute(otel.AttrComponent, s.Component),
			tykmetric.StringAttribute(otel.AttrStore, s.Store),
		)
	}
	assert.ElementsMatch(t, []float64{0, 1}, metrictest.DataPointValues[float64](t, metric), "healthy → 1, unhealthy → 0")

	// A recovered Redis and a lost uptime store show on the next export.
	samples = []otel.HealthSample{
		{Component: "temporal_storage", Store: "redis", Healthy: true},
		{Component: "uptime", Store: "mongo", Healthy: false},
	}
	metric = rec.FindMetric(t, otel.HealthMetricName)
	metrictest.AssertDataPointCount(t, metric, 2)
	assert.ElementsMatch(t, []float64{1, 0}, metrictest.DataPointValues[float64](t, metric))
	assert.EqualValues(t, 2, calls.Load(), "the snapshot is read once per collection")
}

// TestRegisterHealthObserver_EmptySnapshot asserts no series is exported while
// the snapshot has no result: absent, never a placeholder.
func TestRegisterHealthObserver_EmptySnapshot(t *testing.T) {
	logger, _ := logrustest.NewNullLogger()
	rec := metrictest.NewRecorder(t)
	m := otel.InitMetrics(context.Background(), logger, confWithHealth(nil), testIdentity(), rec.Option())

	require.NoError(t, m.RegisterHealthObserver(func(context.Context) []otel.HealthSample { return nil }))
	assert.NotContains(t, rec.MetricNames(), otel.HealthMetricName, "no data points ⇒ no series")
	assert.Contains(t, rec.MetricNames(), otel.UptimeMetricName, "other families keep exporting")
}

// TestRegisterHealthObserver_FamilyOff asserts health_metrics=false registers
// nothing and never reads the snapshot, while process.uptime keeps working.
func TestRegisterHealthObserver_FamilyOff(t *testing.T) {
	logger, _ := logrustest.NewNullLogger()
	rec := metrictest.NewRecorder(t)
	m := otel.InitMetrics(context.Background(), logger, confWithHealth(boolPtr(false)), testIdentity(), rec.Option())
	require.True(t, m.Enabled())
	assert.False(t, m.HealthEnabled())

	called := false
	require.NoError(t, m.RegisterHealthObserver(func(context.Context) []otel.HealthSample {
		called = true
		return bothSamples
	}))
	assert.NotContains(t, rec.MetricNames(), otel.HealthMetricName)
	assert.Contains(t, rec.MetricNames(), otel.UptimeMetricName)
	assert.False(t, called, "the snapshot must never be read when the family is off")
}

// TestRegisterHealthObserver_Disabled asserts no-op behaviour on a disabled
// provider, a zero container and a nil receiver.
func TestRegisterHealthObserver_Disabled(t *testing.T) {
	logger, _ := logrustest.NewNullLogger()
	snapshot := func(context.Context) []otel.HealthSample { return bothSamples }

	disabled := otel.InitMetrics(context.Background(), logger, otel.OpenTelemetry{}, testIdentity())
	assert.False(t, disabled.HealthEnabled())
	assert.NoError(t, disabled.RegisterHealthObserver(snapshot))

	var zero otel.MetricInstruments
	assert.False(t, zero.HealthEnabled())
	assert.NoError(t, zero.RegisterHealthObserver(snapshot))

	var nilInst *otel.MetricInstruments
	assert.False(t, nilInst.HealthEnabled())
	assert.NoError(t, nilInst.RegisterHealthObserver(snapshot))
}
