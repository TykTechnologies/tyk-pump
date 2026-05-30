package serializer

import (
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

// Verifies: SW-REQ-008
// Verifies: SYS-REQ-002
// SW-REQ-008:encoding_safety:negative
func TestSerializer_RichRecordRoundtrip(t *testing.T) {
	// Fully populated record drives TransformSingleRecordToProto field branches
	// (each non-zero field exercises its assignment), then the inverse.
	now := time.Now().UTC().Truncate(time.Second)
	rec := analytics.AnalyticsRecord{
		Method: "POST", Host: "api.example", Path: "/v1/x", RawPath: "/v1/x",
		ContentLength: 42, UserAgent: "tyk-pump-test", Day: now.Day(), Month: now.Month(),
		Year: now.Year(), Hour: now.Hour(), ResponseCode: 200, APIKey: "k1",
		TimeStamp: now, APIVersion: "v1", APIName: "n", APIID: "api1", OrgID: "org1",
		OauthID: "oa1", RequestTime: 12345, RawRequest: "req", RawResponse: "resp",
		IPAddress: "1.2.3.4", Tags: []string{"t1", "t2"}, Alias: "a", TrackPath: true,
	}
	rec.Latency = analytics.Latency{Total: 9, Upstream: 4}
	rec.Network = analytics.NetworkStats{OpenConnections: 1, ClosedConnection: 1, BytesIn: 100, BytesOut: 200}
	rec.Geo.Country.ISOCode = "US"

	for _, name := range []string{MSGP_SERIALIZER, PROTOBUF_SERIALIZER} {
		s := NewAnalyticsSerializer(name)
		buf, err := s.Encode(&rec)
		if err != nil {
			t.Fatalf("%s encode: %v", name, err)
		}
		if len(buf) == 0 {
			t.Fatalf("%s: empty encoded buffer", name)
		}
		var out analytics.AnalyticsRecord
		if err := s.Decode(buf, &out); err != nil {
			t.Fatalf("%s decode: %v", name, err)
		}
		if out.APIID != rec.APIID || out.OrgID != rec.OrgID || out.ResponseCode != rec.ResponseCode {
			t.Fatalf("%s: round-trip lost key fields: %+v", name, out)
		}
	}
}

// Verifies: SW-REQ-008
// SW-REQ-008:encoding_safety:negative
func TestSerializer_Decode_Malformed(t *testing.T) {
	for _, name := range []string{MSGP_SERIALIZER, PROTOBUF_SERIALIZER} {
		s := NewAnalyticsSerializer(name)
		var out analytics.AnalyticsRecord
		// Random bytes are not a valid encoded record for either format.
		err := s.Decode([]byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb}, &out)
		if err == nil {
			t.Fatalf("%s: expected decode error on malformed bytes", name)
		}
	}
}

// Verifies: SW-REQ-008
func TestNewAnalyticsSerializer_DefaultsToMsgp(t *testing.T) {
	if s := NewAnalyticsSerializer(""); s.GetSuffix() != NewAnalyticsSerializer(MSGP_SERIALIZER).GetSuffix() {
		t.Fatalf("empty type should default to msgp; got suffix %q", s.GetSuffix())
	}
	if s := NewAnalyticsSerializer("unknown-format"); s.GetSuffix() != NewAnalyticsSerializer(MSGP_SERIALIZER).GetSuffix() {
		t.Fatalf("unknown type should fall back to msgp; got suffix %q", s.GetSuffix())
	}
}
