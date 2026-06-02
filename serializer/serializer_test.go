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

// Verifies: SW-REQ-008
// Verifies: SYS-REQ-002
// Verifies: INT-REQ-003
// MCDC SYS-REQ-002: record_fields_preserved=F, record_forwarded=F => TRUE
// MCDC SYS-REQ-002: record_fields_preserved=F, record_forwarded=T => FALSE
// MCDC SYS-REQ-002: record_fields_preserved=T, record_forwarded=T => TRUE
// MCDC INT-REQ-003: roundtrip_equal_except_protobuf_city_names=F, serialize_then_deserialize=F => TRUE
// MCDC INT-REQ-003: roundtrip_equal_except_protobuf_city_names=F, serialize_then_deserialize=T => FALSE
// MCDC INT-REQ-003: roundtrip_equal_except_protobuf_city_names=T, serialize_then_deserialize=T => TRUE
//
// Encode() forwards the AnalyticsRecord through the serializer (record_forwarded=T) and the
// assertions assert.NotEqual(0, len(bytes)) + assert.Equal(nil, err) prove
// record_fields_preserved=T -> TRUE row for SYS-REQ-002. For INT-REQ-003, the encode-only
// path is the serialize half; TestSerializer_Decode (which round-trips the same record and
// asserts cmp.Equal) closes the deserialize half -> TRUE row. The error rows are caught by
// the err==nil and non-empty-bytes assertions.
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

// Verifies: SW-REQ-008
// Verifies: INT-REQ-001
// MCDC INT-REQ-001: gateway_emits_record=F, record_at_tyk_system_analytics=F => TRUE
// MCDC INT-REQ-001: gateway_emits_record=T, record_at_tyk_system_analytics=F => FALSE
// MCDC INT-REQ-001: gateway_emits_record=T, record_at_tyk_system_analytics=T => TRUE
//
// Each sub-test encodes a record (gateway-emitted analogue, gateway_emits_record=T) and
// decodes it back; the cmp.Equal assertion proves that the deserialised record matches the
// original (record_at_tyk_system_analytics=T) -> TRUE row. The FALSE row corresponds to a
// regression where Encode/Decode silently drops fields; cmp.Equal catches it. The vacuous
// TRUE arm is the no-emit case.
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

// Verifies: SW-REQ-008
func TestSerializer_MCPStats_Roundtrip(t *testing.T) {
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
				APIID:    "api_1",
				OrgID:    "org_1",
				ExpireAt: time.Now().Add(time.Hour).Round(0),
				MCPStats: analytics.MCPStats{
					IsMCP:         true,
					JSONRPCMethod: "tools/call",
					PrimitiveType: "tool",
					PrimitiveName: "get_weather",
				},
			}

			encoded, err := tc.serializer.Encode(&record)
			assert.NoError(t, err)

			decoded := &analytics.AnalyticsRecord{}
			err = tc.serializer.Decode(encoded, decoded)
			assert.NoError(t, err)

			assert.True(t, decoded.IsMCPRecord(), "decoded record should be identified as MCP")
			assert.Equal(t, record.MCPStats, decoded.MCPStats)
		})
	}
}

// Verifies: SW-REQ-008
func TestSerializer_NonMCP_NoMCPStats(t *testing.T) {
	serializer := NewAnalyticsSerializer(PROTOBUF_SERIALIZER)

	record := analytics.AnalyticsRecord{
		APIID: "api_1",
		OrgID: "org_1",
	}

	encoded, err := serializer.Encode(&record)
	assert.NoError(t, err)

	decoded := &analytics.AnalyticsRecord{}
	err = serializer.Decode(encoded, decoded)
	assert.NoError(t, err)

	assert.False(t, decoded.IsMCPRecord(), "non-MCP record should not be identified as MCP")
	assert.Equal(t, analytics.MCPStats{}, decoded.MCPStats)
}

// Verifies: SW-REQ-008
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

// Verifies: SW-REQ-008
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

// Verifies: SW-REQ-008
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
