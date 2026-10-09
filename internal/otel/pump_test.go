package otel_test

import (
	"context"
	"strings"
	"testing"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
	"github.com/TykTechnologies/opentelemetry/metric/metrictest"

	"github.com/TykTechnologies/tyk-pump/internal/otel"
)

// pumpFamily lists every metric of the pump write metrics family.
var pumpFamily = []string{
	otel.PumpInitializedMetricName,
	otel.PumpWritesMetricName,
	otel.PumpWriteRecordsMetricName,
	otel.PurgeRecordsMetricName,
}

// newPumpMetrics returns enabled instruments backed by a manual reader, with
// pump_metrics set to pumpMetrics (nil leaves it unset).
func newPumpMetrics(t *testing.T, pumpMetrics *bool) (*otel.MetricInstruments, *metrictest.Recorder) {
	t.Helper()
	conf := enabledConf()
	conf.Metrics.PumpMetrics = pumpMetrics
	logger, _ := logrustest.NewNullLogger()
	rec := metrictest.NewRecorder(t)
	m := otel.InitMetrics(context.Background(), logger, conf, testIdentity(), rec.Option())
	require.True(t, m.Enabled())
	return m, rec
}

var (
	splunkA = otel.PumpIdentity{Name: "splunk-a", Type: "splunk"}
	splunkB = otel.PumpIdentity{Name: "splunk-b", Type: "splunk"}
)

// assertSums asserts the named counter holds exactly the points in want, each
// keyed by the values of keys joined with "/".
func assertSums(t *testing.T, rec *metrictest.Recorder, name string, keys []string, want map[string]int64) {
	t.Helper()
	m := rec.FindMetric(t, name)
	metrictest.AssertDataPointCount(t, m, len(want))
	for k, v := range want {
		values := strings.Split(k, "/")
		attrs := make([]tykmetric.Attribute, len(keys))
		for i, key := range keys {
			attrs[i] = tykmetric.StringAttribute(key, values[i])
		}
		metrictest.AssertSumWithAttrs(t, m, v, attrs...)
	}
}

var (
	byPumpTypeOutcome = []string{otel.AttrPump, otel.AttrPumpType, otel.AttrOutcome}
	byPumpOutcome     = []string{otel.AttrPump, otel.AttrOutcome}
	byResult          = []string{otel.AttrResult}
)

func TestRecordPumpWrite(t *testing.T) {
	m, rec := newPumpMetrics(t, nil)
	ctx := context.Background()

	m.RecordPumpWrite(ctx, splunkA, otel.OutcomeSuccess, 10, 0)
	m.RecordPumpWrite(ctx, splunkA, otel.OutcomeSuccess, 5, 0)
	m.RecordPumpWrite(ctx, splunkA, otel.OutcomeError, 7, 3)
	m.RecordPumpWrite(ctx, splunkB, otel.OutcomeTimeout, 4, 0)

	// One write per call, labelled with the configured name and the registry type.
	assertSums(t, rec, otel.PumpWritesMetricName, byPumpTypeOutcome, map[string]int64{
		"splunk-a/splunk/success": 2,
		"splunk-a/splunk/error":   1,
		"splunk-b/splunk/timeout": 1,
	})
	// Records are added under the write's outcome, filtered ones under filtered.
	assertSums(t, rec, otel.PumpWriteRecordsMetricName, byPumpTypeOutcome, map[string]int64{
		"splunk-a/splunk/success":  15,
		"splunk-a/splunk/error":    7,
		"splunk-a/splunk/filtered": 3,
		"splunk-b/splunk/timeout":  4,
	})

	assert.Equal(t, "{write}", rec.FindMetric(t, otel.PumpWritesMetricName).Unit)
	assert.Equal(t, "{record}", rec.FindMetric(t, otel.PumpWriteRecordsMetricName).Unit)
}

func TestRecordPumpWrite_ZeroAddsAreSkipped(t *testing.T) {
	m, rec := newPumpMetrics(t, nil)

	// Everything filtered: the write is counted, no handed-records series.
	m.RecordPumpWrite(context.Background(), splunkA, otel.OutcomeSuccess, 0, 4)
	// Nothing filtered: no filtered series.
	m.RecordPumpWrite(context.Background(), splunkB, otel.OutcomeSuccess, 4, 0)

	assertSums(t, rec, otel.PumpWritesMetricName, byPumpOutcome, map[string]int64{
		"splunk-a/success": 1,
		"splunk-b/success": 1,
	})
	assertSums(t, rec, otel.PumpWriteRecordsMetricName, byPumpOutcome, map[string]int64{
		"splunk-a/filtered": 4,
		"splunk-b/success":  4,
	})
}

func TestRecordPurgeRecords(t *testing.T) {
	m, rec := newPumpMetrics(t, nil)
	ctx := context.Background()

	m.RecordPurgeRecords(ctx, 10, 0)
	m.RecordPurgeRecords(ctx, 3, 2)

	assertSums(t, rec, otel.PurgeRecordsMetricName, byResult, map[string]int64{
		otel.PurgeResultDecoded:      13,
		otel.PurgeResultDecodeFailed: 2,
	})
	assert.Equal(t, "{record}", rec.FindMetric(t, otel.PurgeRecordsMetricName).Unit)

	t.Run("zero adds are skipped", func(t *testing.T) {
		m, rec := newPumpMetrics(t, nil)
		m.RecordPurgeRecords(ctx, 0, 0)
		assert.NotContains(t, rec.MetricNames(), otel.PurgeRecordsMetricName)

		m.RecordPurgeRecords(ctx, 2, 0)
		assertSums(t, rec, otel.PurgeRecordsMetricName, byResult, map[string]int64{otel.PurgeResultDecoded: 2})
	})
}

func TestRegisterPumpInitObserver(t *testing.T) {
	// One pump per case, so each value is tied to its identity.
	for _, tc := range []struct {
		state otel.PumpInitState
		want  float64
	}{
		{otel.PumpInitState{PumpIdentity: otel.PumpIdentity{Name: "mongo", Type: "mongo"}, Initialized: true}, 1},
		{otel.PumpInitState{PumpIdentity: otel.PumpIdentity{Name: "splunk-noinit", Type: "splunk"}}, 0},
		{otel.PumpInitState{PumpIdentity: otel.PumpIdentity{Name: "typo", Type: otel.UnknownPump}}, 0},
	} {
		t.Run(tc.state.Name, func(t *testing.T) {
			m, rec := newPumpMetrics(t, nil)
			require.NoError(t, m.RegisterPumpInitObserver([]otel.PumpInitState{tc.state}))

			g := rec.FindMetric(t, otel.PumpInitializedMetricName)
			assert.Equal(t, tc.want, metrictest.GaugeValue[float64](t, g))
			metrictest.AssertHasAttributes(t, g,
				tykmetric.StringAttribute(otel.AttrPump, tc.state.Name),
				tykmetric.StringAttribute(otel.AttrPumpType, tc.state.Type))
			assert.Empty(t, g.Unit, "registered without a unit, so Prometheus adds no suffix")
		})
	}

	t.Run("one series per configured pump", func(t *testing.T) {
		m, rec := newPumpMetrics(t, nil)
		require.NoError(t, m.RegisterPumpInitObserver([]otel.PumpInitState{
			{PumpIdentity: splunkA, Initialized: true},
			{PumpIdentity: splunkB},
		}))
		metrictest.AssertDataPointCount(t, rec.FindMetric(t, otel.PumpInitializedMetricName), 2)
	})
}

func TestPumpMetrics_FamilyOff(t *testing.T) {
	off := false
	ctx := context.Background()
	record := func(m *otel.MetricInstruments) {
		m.RecordPumpWrite(ctx, splunkA, otel.OutcomeSuccess, 3, 1)
		m.RecordPurgeRecords(ctx, 3, 1)
		assert.NoError(t, m.RegisterPumpInitObserver([]otel.PumpInitState{{PumpIdentity: splunkA, Initialized: true}}))
	}

	t.Run("pump_metrics false", func(t *testing.T) {
		m, rec := newPumpMetrics(t, &off)

		record(m)

		names := rec.MetricNames()
		assert.Contains(t, names, otel.UptimeMetricName, "process.uptime keeps flowing with the family off")
		for _, name := range pumpFamily {
			assert.NotContains(t, names, name)
		}
	})

	t.Run("pump_metrics unset is on", func(t *testing.T) {
		m, rec := newPumpMetrics(t, nil)

		record(m)

		names := rec.MetricNames()
		for _, name := range pumpFamily {
			assert.Contains(t, names, name)
		}
	})

	t.Run("metrics disabled", func(t *testing.T) {
		on := true
		logger, _ := logrustest.NewNullLogger()
		conf := otel.OpenTelemetry{}
		conf.Metrics.PumpMetrics = &on // global off wins over a family true

		m := otel.InitMetrics(context.Background(), logger, conf, testIdentity())

		assert.NotPanics(t, func() { record(m) })
	})

	t.Run("nil and zero containers", func(t *testing.T) {
		var nilM *otel.MetricInstruments
		assert.NotPanics(t, func() { record(nilM) })
		assert.NotPanics(t, func() { record(&otel.MetricInstruments{}) })
		assert.NotPanics(t, func() { record(otel.Metrics()) })
	})
}

func TestPumpMetricsEnabled(t *testing.T) {
	on, off := true, false
	for _, tc := range []struct {
		metrics, toggle *bool
		name            string
		want            bool
	}{
		{nil, nil, "metrics unset", false},
		{&off, &on, "global off wins", false},
		{&on, nil, "unset toggle is on", true},
		{&on, &on, "explicit true", true},
		{&on, &off, "explicit false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := otel.OpenTelemetry{}
			c.Metrics.Enabled = tc.metrics
			c.Metrics.PumpMetrics = tc.toggle
			assert.Equal(t, tc.want, c.PumpMetricsEnabled())
		})
	}
}
