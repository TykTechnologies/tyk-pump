package main

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
	"github.com/TykTechnologies/opentelemetry/metric/metrictest"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/internal/otel"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/serializer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePump is a pump whose write result and timing the test controls.
type fakePump struct {
	err error
	// release, when set, blocks WriteData until it is closed.
	release chan struct{}
	// filtering, when set, blocks filterData (through GetIgnoreFields) until
	// it is closed, so a timeout can fire before the pump is handed anything.
	filtering chan struct{}
	// returned is closed once WriteData returns.
	returned chan struct{}
	pumps.CommonPumpConfig
	written atomic.Int64
}

func newFakePump(err error) *fakePump {
	return &fakePump{err: err, returned: make(chan struct{})}
}

func (p *fakePump) GetName() string        { return "Fake Pump" }
func (p *fakePump) New() pumps.Pump        { return newFakePump(nil) }
func (p *fakePump) Init(interface{}) error { return nil }

func (p *fakePump) GetIgnoreFields() []string {
	if p.filtering != nil {
		<-p.filtering
	}
	return p.CommonPumpConfig.GetIgnoreFields()
}

func (p *fakePump) WriteData(_ context.Context, keys []interface{}) error {
	defer close(p.returned)
	if p.release != nil {
		<-p.release
	}
	p.written.Add(int64(len(keys)))
	return p.err
}

// failingInitPump is a registered pump type whose Init always fails.
type failingInitPump struct{ pumps.CommonPumpConfig }

func (p *failingInitPump) GetName() string                                { return "Failing Pump" }
func (p *failingInitPump) New() pumps.Pump                                { return &failingInitPump{} }
func (p *failingInitPump) Init(interface{}) error                         { return errors.New("no collector_token") }
func (p *failingInitPump) WriteData(context.Context, []interface{}) error { return nil }

// setupPumpMetrics installs enabled process-wide instruments backed by a
// manual reader, with pump_metrics set to pumpMetrics (nil leaves it unset),
// and restores the pumps and instruments afterwards.
func setupPumpMetrics(t *testing.T, pumpMetrics *bool) *metrictest.Recorder {
	t.Helper()
	enabled := true
	conf := otel.OpenTelemetry{}
	conf.Metrics.Enabled = &enabled
	conf.Metrics.PumpMetrics = pumpMetrics
	rec := metrictest.NewRecorder(t)
	m := otel.InitMetrics(context.Background(), log, conf, otel.Identity{InstanceID: "test"}, rec.Option())
	require.True(t, m.Enabled())

	origPumps, origIDs, origConfig := Pumps, pumpIdentities, SystemConfig
	otel.SetActive(m)
	t.Cleanup(func() {
		otel.SetActive(nil)
		Pumps, pumpIdentities, SystemConfig = origPumps, origIDs, origConfig
	})
	return rec
}

// usePumps installs pumps as if initialisePumps had built them under the
// given configured identities.
func usePumps(byID map[otel.PumpIdentity]pumps.Pump) {
	Pumps = nil
	pumpIdentities = map[pumps.Pump]otel.PumpIdentity{}
	for id, p := range byID {
		Pumps = append(Pumps, p)
		pumpIdentities[p] = id
	}
}

// batch returns n records, the first skipped of them for API "skip".
func batch(n, skipped int) []interface{} {
	keys := make([]interface{}, n)
	for i := range keys {
		apiID := "keep"
		if i < skipped {
			apiID = "skip"
		}
		keys[i] = analytics.AnalyticsRecord{APIID: apiID, OrgID: "org", ResponseCode: 200}
	}
	return keys
}

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

var byPumpTypeOutcome = []string{otel.AttrPump, otel.AttrPumpType, otel.AttrOutcome}

// assertWrites asserts the write and write-record counters hold exactly
// writes and records, keyed by pump/pump_type/outcome.
func assertWrites(t *testing.T, rec *metrictest.Recorder, writes, records map[string]int64) {
	t.Helper()
	assertSums(t, rec, otel.PumpWritesMetricName, byPumpTypeOutcome, writes)
	assertSums(t, rec, otel.PumpWriteRecordsMetricName, byPumpTypeOutcome, records)
}

func TestExecPumpWriting_RecordsOutcomePerPump(t *testing.T) {
	rec := setupPumpMetrics(t, nil)

	ok := newFakePump(nil)
	failing := newFakePump(errors.New("connection refused"))
	slow := newFakePump(nil)
	slow.release = make(chan struct{})
	slow.SetTimeout(1)
	filtered := newFakePump(nil)
	filtered.SetFilters(analytics.AnalyticsFilters{SkippedAPIIDs: []string{"skip"}})

	usePumps(map[otel.PumpIdentity]pumps.Pump{
		{Name: "ok", Type: "dummy"}:           ok,
		{Name: "splunk-down", Type: "splunk"}: failing,
		{Name: "mongo-slow", Type: "mongo"}:   slow,
		{Name: "filtered", Type: "dummy"}:     filtered,
	})

	writeToPumps(batch(6, 3), nil, time.Now(), 10)

	wantWrites := map[string]int64{
		"ok/dummy/success":         1,
		"splunk-down/splunk/error": 1,
		"mongo-slow/mongo/timeout": 1,
		"filtered/dummy/success":   1,
	}
	wantRecords := map[string]int64{
		"ok/dummy/success":         6,
		"splunk-down/splunk/error": 6,
		"mongo-slow/mongo/timeout": 6,
		"filtered/dummy/success":   3,
		"filtered/dummy/filtered":  3,
	}
	// One write per pump, and the slow pump does not change the others' outcomes.
	assertWrites(t, rec, wantWrites, wantRecords)
	assert.EqualValues(t, 3, filtered.written.Load(), "the filtered pump is still only handed what its filters keep")

	// Let the abandoned write finish: its late result must add nothing.
	close(slow.release)
	select {
	case <-slow.returned:
	case <-time.After(5 * time.Second):
		t.Fatal("the slow pump's write never returned")
	}
	assertWrites(t, rec, wantWrites, wantRecords)
}

func TestExecPumpWriting_TimeoutWhileFiltering(t *testing.T) {
	rec := setupPumpMetrics(t, nil)

	p := newFakePump(nil)
	p.SetTimeout(1)
	p.SetFilters(analytics.AnalyticsFilters{SkippedAPIIDs: []string{"skip"}})
	p.filtering = make(chan struct{})
	usePumps(map[otel.PumpIdentity]pumps.Pump{{Name: "stuck", Type: "dummy"}: p})

	writeToPumps(batch(4, 2), nil, time.Now(), 10)

	// Before filterData returns, the whole batch counts as handed, so
	// handed + filtered == batch size.
	want := map[string]int64{"stuck/dummy/timeout": 1}
	wantRecords := map[string]int64{"stuck/dummy/timeout": 4}
	assertWrites(t, rec, want, wantRecords)

	// The late filter result adds nothing.
	close(p.filtering)
	<-p.returned
	assertWrites(t, rec, want, wantRecords)
}

func TestExecPumpWriting_UnknownIdentityFallback(t *testing.T) {
	rec := setupPumpMetrics(t, nil)

	// Pumps set directly, as the existing tests and the demo path do.
	Pumps = []pumps.Pump{&MockedPump{}}
	pumpIdentities = nil

	assert.NotPanics(t, func() { writeToPumps(batch(2, 0), nil, time.Now(), 10) })

	assertWrites(t, rec,
		map[string]int64{"unknown/unknown/success": 1},
		map[string]int64{"unknown/unknown/success": 2})
}

func TestExecPumpWriting_FamilyOff(t *testing.T) {
	off := false
	rec := setupPumpMetrics(t, &off)

	usePumps(map[otel.PumpIdentity]pumps.Pump{{Name: "ok", Type: "dummy"}: newFakePump(nil)})
	writeToPumps(batch(2, 0), nil, time.Now(), 10)

	names := rec.MetricNames()
	assert.NotContains(t, names, otel.PumpWritesMetricName)
	assert.NotContains(t, names, otel.PumpWriteRecordsMetricName)
}

func TestInitialisePumps_RecordsInitState(t *testing.T) {
	rec := setupPumpMetrics(t, nil)
	registerRecordingPump(t, "recording")
	pumps.AvailablePumps["failing"] = &failingInitPump{}
	t.Cleanup(func() { delete(pumps.AvailablePumps, "failing") })

	SystemConfig = TykPumpConfiguration{
		DontPurgeUptimeData: true,
		// Keys are upper-cased on load, as LoadConfig does.
		Pumps: map[string]PumpConfig{
			"GOOD":          {Type: "Recording"},
			"SPLUNK-NOINIT": {Type: "failing"},
			"TYPO":          {Type: "does-not-exist"},
		},
	}

	initialisePumps(nil)

	// One series per configured pump: lower-cased name, registry type or unknown.
	g := rec.FindMetric(t, otel.PumpInitializedMetricName)
	metrictest.AssertDataPointCount(t, g, 3)
	for _, id := range []otel.PumpIdentity{
		{Name: "good", Type: "recording"},
		{Name: "splunk-noinit", Type: "failing"},
		{Name: "typo", Type: otel.UnknownPump},
	} {
		metrictest.AssertHasAttributes(t, g,
			tykmetric.StringAttribute(otel.AttrPump, id.Name),
			tykmetric.StringAttribute(otel.AttrPumpType, id.Type))
	}
	values := metrictest.DataPointValues[float64](t, g)
	sort.Float64s(values)
	assert.Equal(t, []float64{0, 0, 1}, values, "only the good pump initialised")

	require.Len(t, Pumps, 1)
	assert.Equal(t, map[pumps.Pump]otel.PumpIdentity{Pumps[0]: {Name: "good", Type: "recording"}}, pumpIdentities,
		"only the initialised pump gets an identity")
}

func TestInitialisePumps_TwoPumpsOfOneType(t *testing.T) {
	rec := setupPumpMetrics(t, nil)
	registerRecordingPump(t, "recording")

	SystemConfig = TykPumpConfiguration{
		DontPurgeUptimeData: true,
		Pumps: map[string]PumpConfig{
			"SPLUNK-A": {Type: "recording"},
			"SPLUNK-B": {Type: "recording"},
		},
	}
	initialisePumps(nil)

	writeToPumps(batch(3, 0), nil, time.Now(), 10)

	// Pumps of one type are distinct by pump and share pump_type.
	assertSums(t, rec, otel.PumpWritesMetricName, byPumpTypeOutcome, map[string]int64{
		"splunk-a/recording/success": 1,
		"splunk-b/recording/success": 1,
	})
}

func TestPreprocessAnalyticsValues_RecordsDecodeResults(t *testing.T) {
	rec := setupPumpMetrics(t, nil)
	usePumps(map[otel.PumpIdentity]pumps.Pump{{Name: "ok", Type: "dummy"}: newFakePump(nil)})

	msgp := serializer.NewAnalyticsSerializer(serializer.MSGP_SERIALIZER)
	values := []interface{}{}
	for i := 0; i < 3; i++ {
		encoded, err := msgp.Encode(&analytics.AnalyticsRecord{APIID: "api", OrgID: "org"})
		require.NoError(t, err)
		values = append(values, string(encoded))
	}
	values = append(values, "not-msgpack")

	PreprocessAnalyticsValues(values, msgp, "analytics-tyk-system-analytics", false, instrument.NewJob("TestJob"), time.Now(), 10)

	assertSums(t, rec, otel.PurgeRecordsMetricName, []string{otel.AttrResult}, map[string]int64{
		otel.PurgeResultDecoded:      3,
		otel.PurgeResultDecodeFailed: 1,
	})
	// A decode failure still occupies a slot, so the pump's records reconcile
	// with the purge total.
	assertSums(t, rec, otel.PumpWriteRecordsMetricName, byPumpTypeOutcome, map[string]int64{"ok/dummy/success": 4})
}
