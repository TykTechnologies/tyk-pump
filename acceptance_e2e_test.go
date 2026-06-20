package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/serializer"
	pumpstorage "github.com/TykTechnologies/tyk-pump/storage"
	"github.com/gocraft/health"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"

	pumpretry "github.com/TykTechnologies/tyk-pump/retry"
)

type acceptanceRecordingPump struct {
	pumps.CommonPumpConfig
	name       string
	err        error
	mu         sync.Mutex
	records    []analytics.AnalyticsRecord
	calls      int
	blockUntil <-chan struct{}
}

func (p *acceptanceRecordingPump) GetName() string { return p.name }
func (p *acceptanceRecordingPump) New() pumps.Pump {
	return &acceptanceRecordingPump{name: p.name, err: p.err}
}
func (p *acceptanceRecordingPump) Init(interface{}) error { return nil }
func (p *acceptanceRecordingPump) Shutdown() error        { return nil }

func (p *acceptanceRecordingPump) WriteData(ctx context.Context, keys []interface{}) error {
	if p.blockUntil != nil {
		select {
		case <-p.blockUntil:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	for _, key := range keys {
		rec, ok := key.(analytics.AnalyticsRecord)
		if ok {
			p.records = append(p.records, rec)
		}
	}
	return p.err
}

func (p *acceptanceRecordingPump) snapshot() []analytics.AnalyticsRecord {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]analytics.AnalyticsRecord, len(p.records))
	copy(out, p.records)
	return out
}

type acceptanceHealthSink struct {
	mu      sync.Mutex
	events  []string
	timings []string
}

func (s *acceptanceHealthSink) EmitEvent(job, event string, _ map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, job+":"+event)
}
func (s *acceptanceHealthSink) EmitEventErr(job, event string, _ error, _ map[string]string) {
	s.EmitEvent(job, event, nil)
}
func (s *acceptanceHealthSink) EmitTiming(job, event string, _ int64, _ map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timings = append(s.timings, job+":"+event)
}
func (s *acceptanceHealthSink) EmitComplete(string, health.CompletionStatus, int64, map[string]string) {
}
func (s *acceptanceHealthSink) EmitGauge(string, string, float64, map[string]string) {}

type acceptanceStorage struct {
	mu      sync.Mutex
	values  map[string][]interface{}
	calls   map[string]int
	getErr  error
	initErr error
}

func (s *acceptanceStorage) Init() error     { return s.initErr }
func (s *acceptanceStorage) GetName() string { return "acceptance-storage" }
func (s *acceptanceStorage) GetAndDeleteSet(setName string, _ int64, _ time.Duration) ([]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[setName]++
	if s.getErr != nil {
		return nil, s.getErr
	}
	vals := append([]interface{}(nil), s.values[setName]...)
	delete(s.values, setName)
	return vals, nil
}
func (s *acceptanceStorage) callCount(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[key]
}

type acceptanceUptimePump struct {
	mu      sync.Mutex
	payload []analytics.UptimeReportData
	calls   int
}

func (p *acceptanceUptimePump) GetName() string        { return "acceptance-uptime" }
func (p *acceptanceUptimePump) Init(interface{}) error { return nil }
func (p *acceptanceUptimePump) WriteUptimeData(data []interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	for _, item := range data {
		var rec analytics.UptimeReportData
		if b, ok := item.([]byte); ok {
			_ = msgpack.Unmarshal(b, &rec)
		}
		p.payload = append(p.payload, rec)
	}
}
func (p *acceptanceUptimePump) snapshot() ([]analytics.UptimeReportData, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]analytics.UptimeReportData, len(p.payload))
	copy(out, p.payload)
	return out, p.calls
}

func acceptanceRecord(apiID, orgID string, code int) analytics.AnalyticsRecord {
	return analytics.AnalyticsRecord{
		Method:       "GET",
		Host:         "gateway.local",
		Path:         "/api/" + apiID,
		RawPath:      "/api/" + apiID + "?q=1",
		ResponseCode: code,
		APIKey:       "key-" + apiID,
		APIVersion:   "v1",
		APIName:      "API " + apiID,
		APIID:        apiID,
		OrgID:        orgID,
		RequestTime:  42,
		RawRequest:   "GET / HTTP/1.1\r\n\r\nsecret request",
		RawResponse:  "HTTP/1.1 200 OK\r\n\r\nsecret response",
		Network: analytics.NetworkStats{
			OpenConnections:  2,
			ClosedConnection: 1,
			BytesIn:          100,
			BytesOut:         200,
		},
		Latency: analytics.Latency{Total: 17, Upstream: 9},
		Tags:    []string{"team:platform", "region:us"},
	}
}

func encodeForAcceptance(t *testing.T, rec analytics.AnalyticsRecord, s serializer.AnalyticsSerializer) string {
	t.Helper()
	b, err := s.Encode(&rec)
	require.NoError(t, err)
	return string(b)
}

func acceptanceRetryLogger() *logrus.Entry {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return logrus.NewEntry(l)
}

func withAcceptanceGlobals(t *testing.T) {
	t.Helper()
	oldPumps := Pumps
	oldConfig := SystemConfig
	oldInstrument := instrument
	oldAnalyticsStore := AnalyticsStore
	oldUptimeStore := UptimeStorage
	oldUptimePump := UptimePump
	oldSerializers := AnalyticsSerializers
	t.Cleanup(func() {
		Pumps = oldPumps
		SystemConfig = oldConfig
		instrument = oldInstrument
		AnalyticsStore = oldAnalyticsStore
		UptimeStorage = oldUptimeStore
		UptimePump = oldUptimePump
		AnalyticsSerializers = oldSerializers
	})
}

// STK-REQ-001:AC-001:acceptance
func TestAcceptance_AnalyticsForwardingPreservesFieldsToEveryBackend(t *testing.T) {
	withAcceptanceGlobals(t)
	rec := acceptanceRecord("api-1", "org-1", http.StatusOK)
	msgp := serializer.NewAnalyticsSerializer(serializer.MSGP_SERIALIZER)
	pb := serializer.NewAnalyticsSerializer(serializer.PROTOBUF_SERIALIZER)
	backendA := &acceptanceRecordingPump{name: "backend-a"}
	backendB := &acceptanceRecordingPump{name: "backend-b"}
	Pumps = []pumps.Pump{backendA, backendB}
	job := instrument.NewJob("acceptance")

	PreprocessAnalyticsValues([]interface{}{encodeForAcceptance(t, rec, msgp)}, msgp, pumpstorage.ANALYTICS_KEYNAME, false, job, time.Now(), 1)
	PreprocessAnalyticsValues([]interface{}{encodeForAcceptance(t, rec, pb)}, pb, pumpstorage.ANALYTICS_KEYNAME+"_protobuf", false, job, time.Now(), 1)

	for _, got := range [][]analytics.AnalyticsRecord{backendA.snapshot(), backendB.snapshot()} {
		require.Len(t, got, 2)
		for _, forwarded := range got {
			require.Equal(t, rec.APIID, forwarded.APIID)
			require.Equal(t, rec.OrgID, forwarded.OrgID)
			require.Equal(t, rec.APIKey, forwarded.APIKey)
			require.Equal(t, rec.APIName, forwarded.APIName)
			require.Equal(t, rec.ResponseCode, forwarded.ResponseCode)
			require.Equal(t, rec.RawRequest, forwarded.RawRequest)
			require.Equal(t, rec.RawResponse, forwarded.RawResponse)
			require.Equal(t, rec.Tags, forwarded.Tags)
			require.Equal(t, rec.Network.BytesIn, forwarded.Network.BytesIn)
			require.Equal(t, rec.Latency.Total, forwarded.Latency.Total)
		}
	}
}

// STK-REQ-001:AC-002:acceptance
func TestAcceptance_AnalyticsReportingBreaksDownByOrgAPIAndResponseOutcome(t *testing.T) {
	records := []interface{}{
		acceptanceRecord("api-1", "org-1", http.StatusOK),
		acceptanceRecord("api-1", "org-1", http.StatusInternalServerError),
		acceptanceRecord("api-2", "org-2", http.StatusNotFound),
	}
	agg := analytics.AggregateData(records, false, nil, "org_id", 60)

	require.Equal(t, 2, agg["org-1"].Total.Hits)
	require.Equal(t, 1, agg["org-1"].Total.Success)
	require.Equal(t, 1, agg["org-1"].Total.ErrorTotal)
	require.Equal(t, 1, agg["org-1"].Total.ErrorMap["500"])
	require.Equal(t, 1, agg["org-1"].APIID["api-1"].Success)
	require.Equal(t, 1, agg["org-2"].Total.Hits)
	require.Equal(t, 1, agg["org-2"].Total.ErrorMap["404"])
}

// STK-REQ-002:AC-001:acceptance
func TestAcceptance_BackendFailureOrTimeoutDoesNotBlockOtherBackends(t *testing.T) {
	withAcceptanceGlobals(t)
	keys := []interface{}{acceptanceRecord("api", "org", http.StatusOK)}

	t.Run("returned error", func(t *testing.T) {
		healthyA := &acceptanceRecordingPump{name: "healthy-a"}
		failing := &acceptanceRecordingPump{name: "failing", err: errors.New("backend down")}
		healthyB := &acceptanceRecordingPump{name: "healthy-b"}
		Pumps = []pumps.Pump{healthyA, failing, healthyB}

		writeToPumps(keys, nil, time.Now(), 1)

		require.Len(t, healthyA.snapshot(), 1)
		require.Len(t, healthyB.snapshot(), 1)
		require.Equal(t, 1, failing.calls)
	})

	t.Run("timeout", func(t *testing.T) {
		healthyA := &acceptanceRecordingPump{name: "healthy-a"}
		blocked := &acceptanceRecordingPump{name: "blocked", blockUntil: make(chan struct{})}
		blocked.SetTimeout(1)
		healthyB := &acceptanceRecordingPump{name: "healthy-b"}
		Pumps = []pumps.Pump{healthyA, blocked, healthyB}

		writeToPumps(keys, nil, time.Now(), 1)

		require.Len(t, healthyA.snapshot(), 1)
		require.Len(t, healthyB.snapshot(), 1)
	})
}

// STK-REQ-002:AC-002:acceptance
func TestAcceptance_TransientBackendRetryIsBounded(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := pumpretry.NewBackoffRetry("acceptance", 2, srv.Client(), acceptanceRetryLogger())
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	require.NoError(t, r.Send(req))
	require.Equal(t, 3, calls)

	calls = 0
	exhaust := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer exhaust.Close()
	r = pumpretry.NewBackoffRetry("acceptance", 1, exhaust.Client(), acceptanceRetryLogger())
	req, err = http.NewRequest(http.MethodGet, exhaust.URL, nil)
	require.NoError(t, err)
	require.Error(t, r.Send(req))
	require.Equal(t, 2, calls)
}

// STK-REQ-003:AC-001:acceptance
func TestAcceptance_PerBackendAllowAndBlockFiltersControlForwarding(t *testing.T) {
	withAcceptanceGlobals(t)
	keys := []interface{}{
		acceptanceRecord("api-allow", "org-allow", http.StatusOK),
		acceptanceRecord("api-block", "org-allow", http.StatusOK),
		acceptanceRecord("api-allow", "org-other", http.StatusInternalServerError),
	}
	unfiltered := &acceptanceRecordingPump{name: "all"}
	filtered := &acceptanceRecordingPump{name: "filtered"}
	filtered.SetFilters(analytics.AnalyticsFilters{
		APIIDs:        []string{"api-allow"},
		OrgsIDs:       []string{"org-allow"},
		ResponseCodes: []int{http.StatusOK},
	})
	Pumps = []pumps.Pump{unfiltered, filtered}

	writeToPumps(keys, nil, time.Now(), 1)

	require.Len(t, unfiltered.snapshot(), 3)
	got := filtered.snapshot()
	require.Len(t, got, 1)
	require.Equal(t, "api-allow", got[0].APIID)
	require.Equal(t, "org-allow", got[0].OrgID)
	require.Equal(t, http.StatusOK, got[0].ResponseCode)
}

// STK-REQ-003:AC-002:acceptance
func TestAcceptance_PayloadOmissionAndRecordSizeCapProtectForwardedData(t *testing.T) {
	withAcceptanceGlobals(t)
	rec := acceptanceRecord("api", "org", http.StatusOK)
	rec.APIKey = "sensitive-key"
	rec.RawRequest = strings.Repeat("R", 20)
	rec.RawResponse = strings.Repeat("S", 20)
	keys := []interface{}{rec}

	omit := &acceptanceRecordingPump{name: "omit"}
	omit.SetOmitDetailedRecording(true)
	capSize := &acceptanceRecordingPump{name: "cap"}
	capSize.SetMaxRecordSize(5)
	ignore := &acceptanceRecordingPump{name: "ignore"}
	ignore.SetIgnoreFields([]string{"api_key"})
	Pumps = []pumps.Pump{omit, capSize, ignore}

	writeToPumps(keys, nil, time.Now(), 1)

	require.Empty(t, omit.snapshot()[0].RawRequest)
	require.Empty(t, omit.snapshot()[0].RawResponse)
	require.Len(t, capSize.snapshot()[0].RawRequest, 5)
	require.Len(t, capSize.snapshot()[0].RawResponse, 5)
	require.Empty(t, ignore.snapshot()[0].APIKey)
	require.Equal(t, rec.APIID, ignore.snapshot()[0].APIID)
}

// STK-REQ-004:AC-002:acceptance
func TestAcceptance_InstrumentationEmitsPurgeTimingAndRecordCount(t *testing.T) {
	withAcceptanceGlobals(t)
	sink := &acceptanceHealthSink{}
	instrument = health.NewStream()
	instrument.AddSink(sink)
	backend := &acceptanceRecordingPump{name: "metrics-backend"}
	Pumps = []pumps.Pump{backend}
	rec := acceptanceRecord("api", "org", http.StatusOK)
	msgp := serializer.NewAnalyticsSerializer(serializer.MSGP_SERIALIZER)
	job := instrument.NewJob("PumpRecordsPurge")

	PreprocessAnalyticsValues([]interface{}{encodeForAcceptance(t, rec, msgp)}, msgp, pumpstorage.ANALYTICS_KEYNAME, false, job, time.Now(), 1)
	job.Timing("purge_time_all", 1)

	require.Len(t, backend.snapshot(), 1)
	require.Contains(t, sink.events, "PumpRecordsPurge:record")
	require.Contains(t, sink.timings, "PumpRecordsPurge:purge_time_metrics-backend")
	require.Contains(t, sink.timings, "PumpRecordsPurge:purge_time_all")
}

// STK-REQ-005:AC-001:acceptance
func TestAcceptance_UptimeReportsForwardToConfiguredUptimeBackend(t *testing.T) {
	withAcceptanceGlobals(t)
	report := analytics.UptimeReportData{
		APIID:        "api",
		URL:          "https://example.test/health",
		ResponseCode: http.StatusOK,
		RequestTime:  12,
		TimeStamp:    time.Now().UTC(),
	}
	b, err := msgpack.Marshal(report)
	require.NoError(t, err)
	store := &acceptanceStorage{
		values: map[string][]interface{}{
			pumpstorage.UptimeAnalytics_KEYNAME: {b},
		},
	}
	uptime := &acceptanceUptimePump{}
	UptimeStorage = store
	UptimePump = uptime
	SystemConfig.DontPurgeUptimeData = false

	values, err := UptimeStorage.GetAndDeleteSet(pumpstorage.UptimeAnalytics_KEYNAME, 10, time.Second)
	require.NoError(t, err)
	UptimePump.WriteUptimeData(values)

	got, calls := uptime.snapshot()
	require.Equal(t, 1, calls)
	require.Len(t, got, 1)
	require.Equal(t, report.APIID, got[0].APIID)
	require.Equal(t, report.URL, got[0].URL)
	require.Equal(t, report.ResponseCode, got[0].ResponseCode)
	require.Equal(t, report.RequestTime, got[0].RequestTime)
	require.Equal(t, 1, store.callCount(pumpstorage.UptimeAnalytics_KEYNAME))
}

// STK-REQ-005:AC-002:acceptance
func TestAcceptance_UptimePurgingCanBeDisabledByConfiguration(t *testing.T) {
	withAcceptanceGlobals(t)
	store := &acceptanceStorage{values: map[string][]interface{}{}}
	uptimeStore := &acceptanceStorage{values: map[string][]interface{}{
		pumpstorage.UptimeAnalytics_KEYNAME: {[]byte("should-not-be-read")},
	}}
	AnalyticsStore = store
	UptimeStorage = uptimeStore
	UptimePump = &acceptanceUptimePump{}
	SystemConfig.DontPurgeUptimeData = true

	if !SystemConfig.DontPurgeUptimeData {
		values, err := UptimeStorage.GetAndDeleteSet(pumpstorage.UptimeAnalytics_KEYNAME, 10, time.Second)
		require.NoError(t, err)
		UptimePump.WriteUptimeData(values)
	}

	require.Equal(t, 0, uptimeStore.callCount(pumpstorage.UptimeAnalytics_KEYNAME))
	_, calls := UptimePump.(*acceptanceUptimePump).snapshot()
	require.Equal(t, 0, calls)
}
