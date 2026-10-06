package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
	"github.com/TykTechnologies/opentelemetry/metric/metrictest"

	"github.com/TykTechnologies/tyk-pump/internal/otel"
	"github.com/TykTechnologies/tyk-pump/pumps"
)

func TestHealthProbes(t *testing.T) {
	originalConfig, originalUptime := SystemConfig, UptimePump
	t.Cleanup(func() { SystemConfig, UptimePump = originalConfig, originalUptime })

	// labels maps each probe's component to its store label.
	labels := func() map[string]string {
		out := map[string]string{}
		for _, p := range healthProbes() {
			require.NotNil(t, p.Ping, "%s has no ping", p.Component)
			out[p.Component] = p.Store
		}
		return out
	}

	t.Run("uptime purging off: only temporal storage", func(t *testing.T) {
		SystemConfig = TykPumpConfiguration{DontPurgeUptimeData: true}
		UptimePump = &pumps.MongoPump{IsUptime: true}
		assert.Equal(t, map[string]string{"temporal_storage": "redis"}, labels(), "uptime must be absent, not 0")
	})

	t.Run("mongo uptime pump", func(t *testing.T) {
		SystemConfig = TykPumpConfiguration{}
		UptimePump = &pumps.MongoPump{IsUptime: true}
		assert.Equal(t, map[string]string{"temporal_storage": "redis", "uptime": "mongo"}, labels())
	})

	t.Run("sql uptime pump that failed to initialise", func(t *testing.T) {
		SystemConfig = TykPumpConfiguration{}
		sqlPump := &pumps.SQLPump{IsUptime: true}
		require.Error(t, sqlPump.Init(map[string]interface{}{"type": "mysql", "connection_string": "nobody@tcp(127.0.0.1:1)/none?timeout=1s"}),
			"nothing listens on port 1")
		UptimePump = sqlPump

		assert.Equal(t, map[string]string{"temporal_storage": "redis", "uptime": "mysql"}, labels(),
			"the store label falls back to the configured type")
		probes := healthProbes()
		assert.Error(t, probes[1].Ping(context.Background()), "a pump without a connection is never healthy")
	})
}

func TestSetupHealthMetrics(t *testing.T) {
	originalConfig := SystemConfig
	t.Cleanup(func() {
		SystemConfig = originalConfig
		otel.SetActive(nil)
	})

	install := func(t *testing.T, health *bool) *metrictest.Recorder {
		t.Helper()
		enabled := true
		cfg := otel.OpenTelemetry{}
		cfg.Metrics.Enabled = &enabled
		cfg.Metrics.HealthMetrics = health
		rec := metrictest.NewRecorder(t)
		otel.SetActive(otel.InitMetrics(context.Background(), log, cfg, otel.Identity{InstanceID: otel.InstanceID()}, rec.Option()))
		return rec
	}

	t.Run("family on registers the gauge", func(t *testing.T) {
		SystemConfig = TykPumpConfiguration{DontPurgeUptimeData: true}
		rec := install(t, nil)

		setupHealthMetrics()

		m := rec.FindMetric(t, otel.HealthMetricName)
		metrictest.AssertDataPointCount(t, m, 1)
		metrictest.AssertHasAttributes(t, m,
			tykmetric.StringAttribute(otel.AttrComponent, "temporal_storage"),
			tykmetric.StringAttribute(otel.AttrStore, "redis"))
	})

	t.Run("family off registers nothing", func(t *testing.T) {
		SystemConfig = TykPumpConfiguration{DontPurgeUptimeData: true}
		off := false
		rec := install(t, &off)

		setupHealthMetrics()

		assert.NotContains(t, rec.MetricNames(), otel.HealthMetricName)
		assert.Contains(t, rec.MetricNames(), otel.UptimeMetricName)
	})

	t.Run("metrics off is a no-op", func(t *testing.T) {
		otel.SetActive(nil)
		assert.NotPanics(t, setupHealthMetrics)
	})
}
