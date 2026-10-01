package otel_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
	"github.com/TykTechnologies/opentelemetry/metric/metrictest"

	"github.com/TykTechnologies/tyk-pump/internal/otel"
)

// enabledConf returns an enabled metrics config. Tests pair it with a
// metrictest.Recorder so no exporter (and no network) is ever created.
func enabledConf() otel.OpenTelemetry {
	enabled := true
	c := otel.OpenTelemetry{}
	c.Metrics.Enabled = &enabled
	return c
}

func testIdentity() otel.Identity {
	return otel.Identity{InstanceID: otel.InstanceID(), Version: "v1.2.3-test"}
}

// warnings returns the messages logged at warning level or more severe.
func warnings(hook *logrustest.Hook) []string {
	var out []string
	for _, e := range hook.AllEntries() {
		if e.Level <= logrus.WarnLevel { // logrus levels: lower is more severe
			out = append(out, e.Message)
		}
	}
	return out
}

func TestOpenTelemetry_SetDefaults(t *testing.T) {
	var c otel.OpenTelemetry
	c.SetDefaults()

	assert.False(t, c.MetricsEnabled(), "SetDefaults must never enable metrics")
	assert.Nil(t, c.Metrics.Enabled)
	assert.Equal(t, "tyk-pump", c.Metrics.ResourceName)
	assert.Equal(t, "unknown", c.Metrics.DeploymentEnvironment)
	assert.Equal(t, "grpc", c.Metrics.Exporter)
	assert.Equal(t, "localhost:4317", c.Metrics.Endpoint)
	assert.Equal(t, 60, c.Metrics.ExportInterval)
	assert.Equal(t, 30, c.Metrics.ShutdownTimeout)

	c.Metrics.ResourceName = "custom"
	c.Metrics.DeploymentEnvironment = "prod"
	c.SetDefaults()
	assert.Equal(t, "custom", c.Metrics.ResourceName, "SetDefaults must not override operator values")
	assert.Equal(t, "prod", c.Metrics.DeploymentEnvironment)

	var neg otel.OpenTelemetry
	neg.Metrics.ShutdownTimeout = -5
	neg.Metrics.ExportInterval = -1
	neg.Metrics.ConnectionTimeout = -1
	neg.Metrics.CardinalityLimit = -1
	neg.SetDefaults()
	assert.Equal(t, 30, neg.Metrics.ShutdownTimeout, "negative shutdown_timeout must fall back to the default")
	assert.Equal(t, 60, neg.Metrics.ExportInterval, "negative export_interval must fall back to the default")
	assert.Equal(t, c.Metrics.ConnectionTimeout, neg.Metrics.ConnectionTimeout)
	assert.Equal(t, -1, neg.Metrics.CardinalityLimit, "cardinality_limit must reach the library unchanged")
}

func TestShutdownTimeout(t *testing.T) {
	assert.Equal(t, 30*time.Second, otel.ShutdownTimeout(otel.OpenTelemetry{}))

	c := otel.OpenTelemetry{}
	c.Metrics.ShutdownTimeout = 7
	assert.Equal(t, 7*time.Second, otel.ShutdownTimeout(c))
	assert.Equal(t, 7, c.Metrics.ShutdownTimeout, "the caller's config must not be mutated")
}

func TestInstanceID(t *testing.T) {
	id := otel.InstanceID()
	require.NotEmpty(t, id)
	_, err := uuid.FromString(id)
	assert.NoError(t, err, "instance id must be a UUID, got %q", id)
	assert.Equal(t, id, otel.InstanceID(), "instance id must be stable for the lifetime of the process")
}

func TestInitMetrics_DisabledByDefault(t *testing.T) {
	logger, hook := logrustest.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	m := otel.InitMetrics(context.Background(), logger, otel.OpenTelemetry{}, testIdentity())

	require.NotNil(t, m)
	require.NotNil(t, m.Provider(), "callers must never nil-check the provider")
	assert.False(t, m.Enabled())
	assert.NoError(t, m.Shutdown(context.Background()), "shutdown of a never-enabled provider is a safe no-op")

	// Off by default: at most one debug line, nothing louder.
	entries := hook.AllEntries()
	assert.LessOrEqual(t, len(entries), 1, "a disabled config logs at most one line")
	for _, e := range entries {
		assert.Equal(t, logrus.DebugLevel, e.Level, "disabled config must only log at debug, got %s: %s", e.Level, e.Message)
	}
}

func TestInitMetrics_ResourceAttributes(t *testing.T) {
	t.Run("defaults and identity", func(t *testing.T) {
		logger, hook := logrustest.NewNullLogger()
		rec := metrictest.NewRecorder(t)
		id := testIdentity()

		m := otel.InitMetrics(context.Background(), logger, enabledConf(), id, rec.Option())
		require.True(t, m.Enabled())
		attrs := rec.ResourceAttributes()

		assert.Equal(t, "tyk-pump", attrs["service.name"])
		assert.Equal(t, id.InstanceID, attrs["service.instance.id"])
		assert.Equal(t, id.Version, attrs["service.version"])
		assert.Equal(t, "unknown", attrs["deployment.environment"])
		assert.NotContains(t, attrs, "control_plane_id", "the Pump never exports a control plane id")

		// Detectors are on, as in the Gateway, MDCB and the Dashboard.
		assert.NotEmpty(t, attrs["host.name"])
		assert.NotEmpty(t, attrs["process.pid"])

		// Everything else must come from a detector or the SDK itself.
		identity := map[string]bool{
			"service.name": true, "service.instance.id": true,
			"service.version": true, "deployment.environment": true,
		}
		for k := range attrs {
			if identity[k] {
				continue
			}
			assert.True(t, hasAnyPrefix(k, "host.", "process.", "container.", "os.", "telemetry.sdk."),
				"unexpected resource attribute %q=%q", k, attrs[k])
		}

		assert.Empty(t, warnings(hook), "a valid config must not warn")
	})

	t.Run("overrides", func(t *testing.T) {
		logger, _ := logrustest.NewNullLogger()
		rec := metrictest.NewRecorder(t)
		conf := enabledConf()
		conf.Metrics.ResourceName = "my-pump"
		conf.Metrics.DeploymentEnvironment = "prod"

		otel.InitMetrics(context.Background(), logger, conf, testIdentity(), rec.Option())
		attrs := rec.ResourceAttributes()

		assert.Equal(t, "my-pump", attrs["service.name"])
		assert.Equal(t, "prod", attrs["deployment.environment"])
	})

	t.Run("caller config is not mutated", func(t *testing.T) {
		logger, _ := logrustest.NewNullLogger()
		conf := enabledConf()
		otel.InitMetrics(context.Background(), logger, conf, testIdentity(), metrictest.NewRecorder(t).Option())
		assert.Empty(t, conf.Metrics.ResourceName)
		assert.Empty(t, conf.Metrics.Exporter)
	})
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func TestInitMetrics_UptimeGauge(t *testing.T) {
	logger, _ := logrustest.NewNullLogger()
	rec := metrictest.NewRecorder(t)

	otel.InitMetrics(context.Background(), logger, enabledConf(), testIdentity(), rec.Option())

	first := rec.FindMetric(t, otel.UptimeMetricName)
	assert.Equal(t, "process.uptime", first.Name)
	assert.Equal(t, "s", first.Unit)
	assert.Equal(t, "Time the process has been running", first.Description)

	v1 := metrictest.GaugeValue[float64](t, first)
	assert.Greater(t, v1, 0.0)

	time.Sleep(20 * time.Millisecond)

	v2 := metrictest.GaugeValue[float64](t, rec.FindMetric(t, otel.UptimeMetricName))
	assert.Greater(t, v2, v1, "uptime must strictly increase between collections")
}

func TestInitMetrics_MisconfiguredDegradesToDisabled(t *testing.T) {
	for name, mutate := range map[string]func(*otel.OpenTelemetry){
		"unknown exporter": func(c *otel.OpenTelemetry) { c.Metrics.Exporter = "bogus" },
		"invalid TLS":      func(c *otel.OpenTelemetry) { c.Metrics.TLS = otel.TLS{Enable: true, MinVersion: "0.9"} },
	} {
		t.Run(name, func(t *testing.T) {
			logger, hook := logrustest.NewNullLogger()
			conf := enabledConf()
			mutate(&conf)

			m := otel.InitMetrics(context.Background(), logger, conf, testIdentity()) // no reader: real init path

			require.NotNil(t, m)
			require.NotNil(t, m.Provider())
			assert.False(t, m.Enabled(), "misconfiguration must degrade to disabled, never fail boot")
			assert.NoError(t, m.Shutdown(context.Background()))

			problems := warnings(hook)
			require.Len(t, problems, 1, "exactly one warning line identifying the problem, got %v", problems)
			entry := hook.LastEntry()
			require.NotNil(t, entry)
			assert.NotNil(t, entry.Data[logrus.ErrorKey], "the warning must carry the underlying error")
		})
	}
}

func TestMetricInstruments_Shutdown(t *testing.T) {
	t.Run("manual reader", func(t *testing.T) {
		logger, _ := logrustest.NewNullLogger()
		m := otel.InitMetrics(context.Background(), logger, enabledConf(), testIdentity(), metrictest.NewRecorder(t).Option())

		assert.NoError(t, shutdownWithin(t, m, 5*time.Second))
	})

	t.Run("unreachable endpoint completes within the deadline", func(t *testing.T) {
		logger, _ := logrustest.NewNullLogger()
		conf := enabledConf()
		conf.Metrics.Endpoint = "127.0.0.1:1" // nothing listens there
		conf.Metrics.ConnectionTimeout = 1

		m := otel.InitMetrics(context.Background(), logger, conf, testIdentity())
		require.True(t, m.Enabled(), "an unreachable endpoint is only detected on export")

		// The flush fails (nothing is listening), but it must not hang.
		if err := shutdownWithin(t, m, 3*time.Second); err != nil {
			t.Logf("flush to an unreachable collector failed as expected: %v", err)
		}
	})

	t.Run("shutdown still runs when the flush fails", func(t *testing.T) {
		logger, _ := logrustest.NewNullLogger()
		p := &fakeProvider{Provider: noopProvider(t), flushErr: errors.New("collector unreachable")}
		m := otel.NewMetricInstruments(p, logger, enabledConf())

		err := m.Shutdown(context.Background())

		assert.ErrorIs(t, err, p.flushErr)
		assert.True(t, p.flushed)
		assert.True(t, p.shutdown, "Shutdown must run even after ForceFlush failed, or the reader goroutine leaks")
	})
}

// shutdownWithin runs m.Shutdown with a context bounded by d and fails the
// test if it does not return in time.
func shutdownWithin(t *testing.T, m *otel.MetricInstruments, d time.Duration) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- m.Shutdown(ctx) }()
	select {
	case err := <-done:
		return err
	case <-time.After(d + time.Second):
		t.Fatalf("Shutdown did not complete within %s", d)
		return nil
	}
}

// fakeProvider is a noop provider that records ForceFlush/Shutdown calls.
type fakeProvider struct {
	tykmetric.Provider
	flushErr error
	flushed  bool
	shutdown bool
}

func (p *fakeProvider) ForceFlush(context.Context) error {
	p.flushed = true
	return p.flushErr
}

func (p *fakeProvider) Shutdown(context.Context) error {
	p.shutdown = true
	return nil
}

func noopProvider(t *testing.T) tykmetric.Provider {
	t.Helper()
	p, err := tykmetric.NewProvider()
	require.NoError(t, err)
	require.False(t, p.Enabled())
	return p
}

func TestMetricInstruments_NilSafety(t *testing.T) {
	var m *otel.MetricInstruments

	assert.False(t, m.Enabled())
	assert.Nil(t, m.Provider())
	assert.NoError(t, m.Shutdown(context.Background()))

	// The zero value (what Metrics() returns before SetActive) is equally safe.
	zero := &otel.MetricInstruments{}
	assert.False(t, zero.Enabled())
	assert.Nil(t, zero.Provider())
	assert.NoError(t, zero.Shutdown(context.Background()))

	// A nil provider is tolerated too.
	logger, _ := logrustest.NewNullLogger()
	nilProvider := otel.NewMetricInstruments(nil, logger, enabledConf())
	assert.False(t, nilProvider.Enabled())
	assert.NoError(t, nilProvider.Shutdown(context.Background()))
}

func TestActiveMetrics(t *testing.T) {
	t.Cleanup(func() { otel.SetActive(nil) })

	require.NotNil(t, otel.Metrics(), "Metrics() must never be nil, even before SetActive")
	assert.False(t, otel.Metrics().Enabled())
	assert.NoError(t, otel.Metrics().Shutdown(context.Background()))

	logger, _ := logrustest.NewNullLogger()
	m := otel.InitMetrics(context.Background(), logger, enabledConf(), testIdentity(), metrictest.NewRecorder(t).Option())

	otel.SetActive(m)
	assert.Same(t, m, otel.Metrics())
	assert.True(t, otel.Metrics().Enabled())

	otel.SetActive(nil)
	require.NotNil(t, otel.Metrics(), "SetActive(nil) installs an empty no-op container")
	assert.False(t, otel.Metrics().Enabled())
	assert.NoError(t, otel.Metrics().Shutdown(context.Background()))
}
