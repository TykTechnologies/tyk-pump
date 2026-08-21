package pumps

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

// benchSyslogServer stands up a UDP listener that drains and discards.
//
// Deliberately not reusing mockSyslogServer from syslog_test.go: that pushes every
// message onto a 100-buffered channel, which fills within the first few hundred
// benchmark iterations and wedges its reader goroutine. Writes are UDP so the
// sender never blocks either way, but a drained socket keeps the measurement
// honest and leaves nothing stuck.
func benchSyslogServer(b *testing.B) (string, func()) {
	b.Helper()

	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("resolve: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		b.Fatalf("listen: %v", err)
	}

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-done:
				return
			default:
				_, _, err := conn.ReadFromUDP(buf)
				if err != nil {
					return
				}
			}
		}
	}()

	return conn.LocalAddr().String(), func() {
		close(done)
		conn.Close()
	}
}

func silentLogger() *logrus.Entry {
	l := logrus.New()
	l.Out = io.Discard
	l.Level = logrus.PanicLevel

	return l.WithField("prefix", "bench")
}

func createBenchSyslogPump(b *testing.B, addr string, includeTags bool) *SyslogPump {
	b.Helper()

	pump := &SyslogPump{
		syslogConf: &SyslogConf{
			Transport:   "udp",
			NetworkAddr: addr,
			LogLevel:    6,
			Tag:         "bench",
			IncludeTags: includeTags,
		},
		CommonPumpConfig: CommonPumpConfig{
			// Discard log output: WriteData emits an Info line per call, which would
			// otherwise dominate the measurement and flood the benchmark output.
			log: silentLogger(),
		},
	}
	pump.initWriter()

	return pump
}

// benchTags returns n realistic tags: the automatic ones the Gateway attaches
// (key-/org-/api-/pol-/dev-) followed by user-defined session and API tags.
func benchTags(n int) []string {
	all := []string{
		"key-test-api-key-aaaaaaaaaaaaaaaaaaaaaaa",
		"org-5e9d9544a1dcd60001d0ed20",
		"api-b84fe1a04e5648927971c0557971565c",
		"pol-6a1b2c3d4e5f60718293a4b5",
		"dev-9f8e7d6c5b4a39281706f5e4",
		"cached-response",
		"tier-gold",
		"region-emea",
		"team-payments",
		"env-production",
	}
	if n <= 0 {
		return nil
	}
	if n > len(all) {
		n = len(all)
	}

	return all[:n]
}

// benchRecord builds a representative record. Fixed timestamp so rendered size is
// stable across runs. detailed mirrors the gateway's detailed-recording mode, where
// raw_request/raw_response are populated.
func benchRecord(nTags int, detailed bool) analytics.AnalyticsRecord {
	rec := analytics.AnalyticsRecord{
		Method:        "POST",
		Host:          "api.example.com",
		Path:          "/api/v1/orders/12345",
		RawPath:       "/api/v1/orders/12345?expand=items",
		ContentLength: 512,
		UserAgent:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		ResponseCode:  200,
		APIKey:        "test-api-key-aaaaaaaaaaaaaaaaaaaaaaa",
		TimeStamp:     time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC),
		APIVersion:    "v1",
		APIName:       "Orders API",
		APIID:         "b84fe1a04e5648927971c0557971565c",
		OrgID:         "5e9d9544a1dcd60001d0ed20",
		RequestTime:   42,
		IPAddress:     "10.42.7.19",
		Tags:          benchTags(nTags),
	}

	if detailed {
		rec.RawRequest = "UE9TVCAvYXBpL3YxL29yZGVycy8xMjM0NSBIVFRQLzEuMQpIb3N0OiBhcGkuZXhhbXBsZS5jb20K" +
			"Q29udGVudC1UeXBlOiBhcHBsaWNhdGlvbi9qc29uCgp7Iml0ZW1zIjpbeyJza3UiOiJBQkMtMTIzIiwicXR5IjoyfV19"
		rec.RawResponse = "SFRUUC8xLjEgMjAwIE9LCkNvbnRlbnQtVHlwZTogYXBwbGljYXRpb24vanNvbgoKeyJvcmRlcl9pZCI6" +
			"MTIzNDUsInN0YXR1cyI6ImNvbmZpcm1lZCJ9"
	}

	return rec
}

// BenchmarkSyslogPump_WriteData measures the per-record cost of building and
// writing a syslog message at varying tag counts, including allocation counts.
//
// The message is rendered with fmt's map formatting, which sorts keys and reflects
// over every value on each record, so per-record cost is worth tracking: WriteData
// writes once per record with no batching.
func BenchmarkSyslogPump_WriteData(b *testing.B) {
	cases := []struct {
		name        string
		nTags       int
		detailed    bool
		includeTags bool
	}{
		// Disabled is the default and must stay level with the pre-field baseline:
		// a record can carry tags without paying for them.
		{"Disabled_RecordHas10Tags", 10, false, false},
		{"Enabled_NoTags", 0, false, true},
		{"Enabled_Tags3", 3, false, true},
		{"Enabled_Tags5", 5, false, true},
		{"Enabled_Tags10", 10, false, true},
		{"Enabled_Tags10_DetailedRecording", 10, true, true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			addr, stop := benchSyslogServer(b)
			defer stop()

			pump := createBenchSyslogPump(b, addr, tc.includeTags)
			data := []interface{}{benchRecord(tc.nTags, tc.detailed)}
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := pump.WriteData(ctx, data); err != nil {
					b.Fatalf("WriteData: %v", err)
				}
			}
		})
	}
}

// BenchmarkSyslogPump_WriteData_Batch measures a whole purge batch rather than a
// single record, approximating the cost of one pump cycle.
//
// This is a proxy for sustained load, not a substitute for it: a real syslog daemon
// under real traffic is the only way to answer that properly.
func BenchmarkSyslogPump_WriteData_Batch(b *testing.B) {
	const batchSize = 1000

	for _, tc := range []struct {
		label       string
		nTags       int
		includeTags bool
	}{
		{"Disabled", 10, false},
		{"Enabled_Tags0", 0, true},
		{"Enabled_Tags10", 10, true},
	} {
		b.Run(fmt.Sprintf("Batch%d_%s", batchSize, tc.label), func(b *testing.B) {
			addr, stop := benchSyslogServer(b)
			defer stop()

			pump := createBenchSyslogPump(b, addr, tc.includeTags)

			data := make([]interface{}, 0, batchSize)
			for i := 0; i < batchSize; i++ {
				data = append(data, benchRecord(tc.nTags, false))
			}
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := pump.WriteData(ctx, data); err != nil {
					b.Fatalf("WriteData: %v", err)
				}
			}
		})
	}
}
