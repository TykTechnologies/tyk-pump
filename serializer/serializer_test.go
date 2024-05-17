package serializer

import (
	"math/rand"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
)

func TestSerializer_Encode(t *testing.T) {
	tcs := []struct {
		testName   string
		serializer AnalyticsSerializer
	}{
		{
			testName:   "msgpack",
			serializer: NewAnalyticsSerializer(MSGP_SERIALIZER),
		},
		{
			testName:   "protobuf",
			serializer: NewAnalyticsSerializer(PROTOBUF_SERIALIZER),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			record := analytics.AnalyticsRecord{
				APIID: "api_1",
				OrgID: "org_1",
			}

			bytes, err := tc.serializer.Encode(&record)

			assert.Equal(t, nil, err)
			assert.NotEqual(t, 0, len(bytes))
		})
	}
}

func TestSerializer_Decode(t *testing.T) {
	tcs := []struct {
		testName   string
		serializer AnalyticsSerializer
	}{
		{
			testName:   "msgpack",
			serializer: NewAnalyticsSerializer(MSGP_SERIALIZER),
		},
		{
			testName:   "protobuf",
			serializer: NewAnalyticsSerializer(PROTOBUF_SERIALIZER),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			record := analytics.AnalyticsRecord{
				APIID: "api_1",
				OrgID: "org_1",
				// The canonical way to strip a monotonic clock reading is to use t = t.Round(0)
				ExpireAt:  time.Now().Add(time.Hour).Round(0),
				TimeStamp: time.Now().Round(0),
			}

			bytes, _ := tc.serializer.Encode(&record)
			newRecord := &analytics.AnalyticsRecord{}

			err := tc.serializer.Decode(bytes, newRecord)
			if err != nil {
				t.Fatal(err)
			}

			recordsAreEqual := cmp.Equal(record, *newRecord, cmpopts.IgnoreUnexported(analytics.AnalyticsRecord{}))
			assert.Equal(t, true, recordsAreEqual, "records should be equal after decoding")
		})
	}
}

func TestSerializer_GetSuffix(t *testing.T) {
	tcs := []struct {
		testName       string
		serializer     AnalyticsSerializer
		expectedSuffix string
	}{
		{
			testName:       "msgpack",
			serializer:     NewAnalyticsSerializer(MSGP_SERIALIZER),
			expectedSuffix: "",
		},
		{
			testName:       "protobuf",
			serializer:     NewAnalyticsSerializer(PROTOBUF_SERIALIZER),
			expectedSuffix: "_protobuf",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			assert.Equal(t, tc.expectedSuffix, tc.serializer.GetSuffix())
		})
	}
}

func BenchmarkProtobufEncoding(b *testing.B) {
	serializer := NewAnalyticsSerializer(PROTOBUF_SERIALIZER)
	records := []analytics.AnalyticsRecord{
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
	}
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	var serialSize int

	for n := 0; n < b.N; n++ {
		record := records[rand.Intn(len(records))]
		bytes, _ := serializer.Encode(&record)
		serialSize += len(bytes)
	}
	b.ReportMetric(float64(serialSize)/float64(b.N), "B/serial")
}

func BenchmarkMsgpEncoding(b *testing.B) {
	serializer := NewAnalyticsSerializer(MSGP_SERIALIZER)
	records := []analytics.AnalyticsRecord{
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_1", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
		demo.GenerateRandomAnalyticRecord("org_2", true),
	}
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	var serialSize int

	for n := 0; n < b.N; n++ {
		record := records[rand.Intn(len(records))]
		bytes, _ := serializer.Encode(&record)
		serialSize += len(bytes)
	}
	b.ReportMetric(float64(serialSize)/float64(b.N), "B/serial")
}
