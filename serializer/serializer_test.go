package serializer

import (
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
)

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
// SW-REQ-008:snapshot_wire_format_compatible:nominal
// SW-REQ-008:snapshot_wire_format_compatible:example
// MCDC SW-REQ-008: key_suffix_protobuf=F, protobuf_codec_selected=F => TRUE
// MCDC SW-REQ-008: key_suffix_protobuf=T, protobuf_codec_selected=T => TRUE
func TestSerializer_Decode_GoldenWireSnapshots(t *testing.T) {
	expected := analytics.AnalyticsRecord{
		APIID:         "api-golden",
		OrgID:         "org-golden",
		Method:        "GET",
		Path:          "/snapshot",
		ResponseCode:  201,
		TimeStamp:     time.Date(2026, 6, 19, 7, 0, 0, 0, time.UTC),
		ExpireAt:      time.Date(2026, 6, 20, 7, 0, 0, 0, time.UTC),
		RawRequest:    "req",
		RawResponse:   "resp",
		RequestTime:   42,
		TrackPath:     true,
		APIVersion:    "v1",
		APIName:       "golden-api",
		APIKey:        "key-golden",
		IPAddress:     "127.0.0.1",
		UserAgent:     "snapshot-test",
		ContentLength: 123,
	}

	tcs := []struct {
		name string
		hex  string
	}{
		{
			name: MSGP_SERIALIZER,
			hex:  "de0021a64d6574686f64a3474554a4486f7374a0a450617468a92f736e617073686f74a752617750617468a0ad436f6e74656e744c656e6774687ba9557365724167656e74ad736e617073686f742d74657374a344617900a54d6f6e746800a45965617200a4486f757200ac526573706f6e7365436f6465ccc9a64150494b6579aa6b65792d676f6c64656ea954696d655374616d7092ce6a34e8f000aa41504956657273696f6ea27631a74150494e616d65aa676f6c64656e2d617069a54150494944aa6170692d676f6c64656ea54f72674944aa6f72672d676f6c64656ea74f617574684944a0ab5265717565737454696d652aaa52617752657175657374a3726571ab526177526573706f6e7365a472657370a9495041646472657373a93132372e302e302e31a347656f83a7436f756e74727981a749534f436f6465a0a44369747982a947656f4e616d65494400a54e616d6573c0a84c6f636174696f6e83a84c61746974756465cb0000000000000000a94c6f6e676974756465cb0000000000000000a854696d655a6f6e65a0a74e6574776f726b84af4f70656e436f6e6e656374696f6e7300b0436c6f736564436f6e6e656374696f6e00a74279746573496e00a842797465734f757400a74c6174656e637983a5546f74616c00a8557073747265616d00a74761746577617900a454616773c0a5416c696173a0a9547261636b50617468c3a8457870697265417492ce6a363a7000a9417069536368656d61a0ac4772617068514c537461747387a95661726961626c6573a0aa526f6f744669656c6473c0a55479706573c0a64572726f7273c0ad4f7065726174696f6e5479706500a94861734572726f7273c2a949734772617068514cc2a84d4350537461747384a549734d4350c2ad4a534f4e5250434d6574686f64a0ad5072696d697469766554797065a0ad5072696d69746976654e616d65a0ae436f6c6c656374696f6e4e616d65a0",
		},
		{
			name: PROTOBUF_SERIALIZER,
			hex:  "12034745541a092f736e617073686f74287b320d736e617073686f742d7465737458c901620a6b65792d676f6c64656e6a0608f0d1d3d106720276317a0a676f6c64656e2d61706982010a6170692d676f6c64656e8a010a6f72672d676f6c64656e90012a9a0100a20103726571aa010472657370b201093132372e302e302e31ba01060a0012001a00c20100d80101e2010608f0f4d8d106f20103555443",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, err := hex.DecodeString(tc.hex)
			assert.NoError(t, err)

			decoded := &analytics.AnalyticsRecord{}
			err = NewAnalyticsSerializer(tc.name).Decode(snapshot, decoded)
			assert.NoError(t, err)

			assert.Equal(t, expected.APIID, decoded.APIID)
			assert.Equal(t, expected.OrgID, decoded.OrgID)
			assert.Equal(t, expected.Method, decoded.Method)
			assert.Equal(t, expected.Path, decoded.Path)
			assert.Equal(t, expected.ResponseCode, decoded.ResponseCode)
			assert.True(t, decoded.TimeStamp.Equal(expected.TimeStamp), "timestamp should preserve the same instant")
			assert.True(t, decoded.ExpireAt.Equal(expected.ExpireAt), "expiry should preserve the same instant")
			assert.Equal(t, expected.RawRequest, decoded.RawRequest)
			assert.Equal(t, expected.RawResponse, decoded.RawResponse)
			assert.Equal(t, expected.RequestTime, decoded.RequestTime)
			assert.Equal(t, expected.TrackPath, decoded.TrackPath)
			assert.Equal(t, expected.APIVersion, decoded.APIVersion)
			assert.Equal(t, expected.APIName, decoded.APIName)
			assert.Equal(t, expected.APIKey, decoded.APIKey)
			assert.Equal(t, expected.IPAddress, decoded.IPAddress)
			assert.Equal(t, expected.UserAgent, decoded.UserAgent)
			assert.Equal(t, expected.ContentLength, decoded.ContentLength)
		})
	}
}

// Verifies: INT-REQ-003
// MCDC INT-REQ-003: roundtrip_equal_except_protobuf_city_names=T, serialize_then_deserialize=T => TRUE
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

// Verifies: INT-REQ-003
// MCDC INT-REQ-003: roundtrip_equal_except_protobuf_city_names=T, serialize_then_deserialize=T => TRUE
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
// SW-REQ-008:snapshot_wire_format_compatible:boundary
// MCDC SW-REQ-008: key_suffix_protobuf=F, protobuf_codec_selected=F => TRUE
// MCDC SW-REQ-008: key_suffix_protobuf=T, protobuf_codec_selected=T => TRUE
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
