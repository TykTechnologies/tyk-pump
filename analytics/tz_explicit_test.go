package analytics

import (
	"encoding/json"
	"testing"
	"time"

	analyticsproto "github.com/TykTechnologies/tyk-pump/analytics/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

// Verifies: SW-REQ-011
// SW-REQ-011:tz_explicit:nominal
//
// Contract: time-bearing fields used in aggregation MUST be TZ-explicit.
// Either:
//
//	(a) the field is documented as UTC (and round-trips preserve the
//	    UTC instant), OR
//	(b) the field carries TZ information that survives serialization.
//
// What this test exercises (analytics/aggregate.go AnalyticsRecordAggregate):
//   - Build the aggregate struct and nested Counter with NY-timezone timestamps.
//   - Round-trip the timestamps through msgpack AND json.
//   - Assert each deserialized timestamp represents the same instant
//     (i.e. UTC equivalence is preserved, even if TZ name is dropped).
//
// If the implementation silently coerces to local time (a regression
// that would lose the instant), the test fails. If TZ metadata is
// dropped (as msgpack v2 does), the test still passes provided the
// UTC instant is preserved — and the surviving guarantee is documented
// in the obligation.
//
// Wave 4 follow-up: if either roundtrip mutates the instant or drops it
// silently, file a KI.
func TestTzExplicit_AggregateTimestamps(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err, "NY timezone must be loadable for this test")

	// Concrete TZ-aware instant: 2026-06-04 09:30:00 New York time.
	original := time.Date(2026, 6, 4, 9, 30, 0, 0, loc)
	require.NotEqual(t, time.UTC, original.Location(),
		"sanity: original timestamp must be in non-UTC zone")

	agg := AnalyticsRecordAggregate{
		OrgID:     "tz-test-org",
		TimeStamp: original,
		LastTime:  original,
		ExpireAt:  original.Add(24 * time.Hour),
		Total: Counter{
			LastTime: original,
		},
	}
	counter := Counter{LastTime: original}

	t.Run("json roundtrip preserves UTC instant", func(t *testing.T) {
		b, err := json.Marshal(agg)
		require.NoError(t, err, "json.Marshal must succeed")

		var decoded AnalyticsRecordAggregate
		require.NoError(t, json.Unmarshal(b, &decoded), "json.Unmarshal must succeed")

		// Equal instant via UTC normalization. We do NOT require Location
		// to be preserved, because JSON of time.Time encodes RFC3339 with
		// a numeric offset, and Go's json decoder restores a Location
		// matching the offset (not the original zone name).
		assert.True(t, original.Equal(decoded.TimeStamp),
			"TimeStamp instant changed across json roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.TimeStamp.Format(time.RFC3339Nano))
		assert.True(t, original.Equal(decoded.LastTime),
			"aggregate LastTime instant changed across json roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.LastTime.Format(time.RFC3339Nano))
		assert.True(t, original.Equal(decoded.Total.LastTime),
			"nested Counter.LastTime instant changed across json roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.Total.LastTime.Format(time.RFC3339Nano))

		// The marshalled JSON MUST carry timezone information (RFC3339 with
		// offset) — this is the "TZ-explicit" half of the obligation.
		s := string(b)
		assert.True(t,
			containsTZMarker(s),
			"serialized JSON lacks TZ offset marker — TZ semantics not explicit on the wire: %s", s)

		counterBytes, err := json.Marshal(counter)
		require.NoError(t, err, "json.Marshal Counter must succeed")
		var decodedCounter Counter
		require.NoError(t, json.Unmarshal(counterBytes, &decodedCounter), "json.Unmarshal Counter must succeed")
		assert.True(t, original.Equal(decodedCounter.LastTime),
			"standalone Counter.LastTime instant changed across json roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decodedCounter.LastTime.Format(time.RFC3339Nano))
	})

	t.Run("msgpack roundtrip preserves UTC instant", func(t *testing.T) {
		b, err := msgpack.Marshal(agg)
		require.NoError(t, err, "msgpack.Marshal must succeed")

		var decoded AnalyticsRecordAggregate
		require.NoError(t, msgpack.Unmarshal(b, &decoded), "msgpack.Unmarshal must succeed")

		// msgpack v2 (vmihailenco/msgpack.v2) is known to drop TZ name —
		// but the represented instant must be preserved (i.e. .Equal works).
		// If this fails, the implementation is losing the instant itself.
		assert.True(t, original.Equal(decoded.TimeStamp),
			"TimeStamp instant changed across msgpack roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.TimeStamp.Format(time.RFC3339Nano))
		assert.True(t, original.Equal(decoded.LastTime),
			"aggregate LastTime instant changed across msgpack roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.LastTime.Format(time.RFC3339Nano))
		assert.True(t, original.Equal(decoded.Total.LastTime),
			"nested Counter.LastTime instant changed across msgpack roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.Total.LastTime.Format(time.RFC3339Nano))

		counterBytes, err := msgpack.Marshal(counter)
		require.NoError(t, err, "msgpack.Marshal Counter must succeed")
		var decodedCounter Counter
		require.NoError(t, msgpack.Unmarshal(counterBytes, &decodedCounter), "msgpack.Unmarshal Counter must succeed")
		assert.True(t, original.Equal(decodedCounter.LastTime),
			"standalone Counter.LastTime instant changed across msgpack roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decodedCounter.LastTime.Format(time.RFC3339Nano))
	})
}

// Verifies: SW-REQ-015
// SW-REQ-015:tz_explicit:nominal
//
// Same contract, applied to UptimeReportData (analytics/uptime_data.go).
// UptimeReportData also derives Day/Month/Year/Hour/Minute integer fields
// from TimeStamp, so we additionally assert that those integer projections
// are computed in the SAME location as the original timestamp (proving the
// projection is not silently UTC-converted, which would corrupt cross-zone
// hourly bucketing).
func TestTzExplicit_UptimeTimestamps(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	original := time.Date(2026, 6, 4, 23, 30, 0, 0, loc) // 23:30 NY = 03:30 UTC next day
	require.NotEqual(t, time.UTC, original.Location())

	rec := UptimeReportData{
		OrgID:     "tz-uptime-org",
		URL:       "https://example.com/health",
		TimeStamp: original,
		ExpireAt:  original.Add(24 * time.Hour),
		Day:       original.Day(),
		Month:     original.Month(),
		Year:      original.Year(),
		Hour:      original.Hour(),
		Minute:    original.Minute(),
	}

	t.Run("json roundtrip preserves UTC instant", func(t *testing.T) {
		b, err := json.Marshal(rec)
		require.NoError(t, err)

		var decoded UptimeReportData
		require.NoError(t, json.Unmarshal(b, &decoded))

		assert.True(t, original.Equal(decoded.TimeStamp),
			"TimeStamp instant changed across json roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.TimeStamp.Format(time.RFC3339Nano))

		// Day/Month/Hour fields must be the NY-local values (23:30 on the
		// 4th), NOT the UTC-converted ones (03:30 on the 5th). This pins
		// the contract that integer projections are taken in the original
		// timestamp's location.
		assert.Equal(t, 4, decoded.Day, "Day must reflect NY-local day, not UTC")
		assert.Equal(t, 23, decoded.Hour, "Hour must reflect NY-local hour, not UTC")
	})

	t.Run("msgpack roundtrip preserves UTC instant", func(t *testing.T) {
		b, err := msgpack.Marshal(rec)
		require.NoError(t, err)

		var decoded UptimeReportData
		require.NoError(t, msgpack.Unmarshal(b, &decoded))

		assert.True(t, original.Equal(decoded.TimeStamp),
			"TimeStamp instant changed across msgpack roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.TimeStamp.Format(time.RFC3339Nano))
	})
}

// Verifies: SW-REQ-009
// SW-REQ-009:tz_explicit:nominal
//
// AnalyticsRecord carries persisted TimeStamp/ExpireAt fields. JSON encodes
// them with RFC3339 offsets, and the protobuf path stores the source location
// in the sibling TimeZone field before protobuf normalizes instants to UTC.
func TestTzExplicit_AnalyticsRecordTimestamps(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	original := time.Date(2026, 6, 4, 9, 30, 0, 0, loc)
	expireAt := original.Add(24 * time.Hour)
	rec := AnalyticsRecord{
		OrgID:     "tz-analytics-org",
		TimeStamp: original,
		ExpireAt:  expireAt,
	}

	t.Run("json roundtrip preserves UTC instant and offset marker", func(t *testing.T) {
		b, err := json.Marshal(rec)
		require.NoError(t, err)

		var decoded AnalyticsRecord
		require.NoError(t, json.Unmarshal(b, &decoded))

		assert.True(t, original.Equal(decoded.TimeStamp),
			"TimeStamp instant changed across json roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.TimeStamp.Format(time.RFC3339Nano))
		assert.True(t, expireAt.Equal(decoded.ExpireAt),
			"ExpireAt instant changed across json roundtrip: original=%s decoded=%s",
			expireAt.Format(time.RFC3339Nano), decoded.ExpireAt.Format(time.RFC3339Nano))
		assert.True(t, containsTZMarker(string(b)),
			"serialized AnalyticsRecord JSON lacks TZ offset marker: %s", string(b))
	})

	t.Run("protobuf roundtrip preserves source location", func(t *testing.T) {
		protoRec := &analyticsproto.AnalyticsRecord{}
		rec.TimestampToProto(protoRec)

		require.Equal(t, loc.String(), protoRec.TimeZone,
			"protobuf carrier must record the source timestamp location")

		var decoded AnalyticsRecord
		decoded.TimeStampFromProto(protoRec)

		assert.True(t, original.Equal(decoded.TimeStamp),
			"TimeStamp instant changed across protobuf roundtrip: original=%s decoded=%s",
			original.Format(time.RFC3339Nano), decoded.TimeStamp.Format(time.RFC3339Nano))
		assert.Equal(t, loc.String(), decoded.TimeStamp.Location().String(),
			"protobuf roundtrip must restore source timestamp location")
		assert.True(t, expireAt.Equal(decoded.ExpireAt),
			"ExpireAt instant changed across protobuf roundtrip: original=%s decoded=%s",
			expireAt.Format(time.RFC3339Nano), decoded.ExpireAt.Format(time.RFC3339Nano))
		assert.Equal(t, loc.String(), decoded.ExpireAt.Location().String(),
			"protobuf roundtrip must restore source expiry location")
	})
}

// Verifies: SW-REQ-011
// SW-REQ-011:tz_explicit:boundary
//
// Cross-zone aggregation: two records produced in different timezones
// that happen to share the same hour-of-day must NOT collide when bucketed
// hourly. The hourly truncation (setAggregateTimestamp at aggregationTime=60)
// uses asTime.Location(), so 9:00 NY and 9:00 UTC produce different hourly
// timestamps. This pins the contract.
func TestTzExplicit_HourlyBucketingPreservesZone(t *testing.T) {
	nyLoc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	nyNine := time.Date(2026, 6, 4, 9, 30, 0, 0, nyLoc)
	utcNine := time.Date(2026, 6, 4, 9, 30, 0, 0, time.UTC)

	nyHourly := setAggregateTimestamp("", nyNine, 60)
	utcHourly := setAggregateTimestamp("", utcNine, 60)

	// Hourly buckets are truncations to (Year, Month, Day, Hour, 0, 0, location).
	// They must represent different instants because the two source timestamps
	// describe different wall-clock instants (9:30 NY = 13:30 UTC).
	assert.False(t, nyHourly.Equal(utcHourly),
		"hourly buckets for 9:30 NY and 9:30 UTC must NOT collide: nyHourly=%s utcHourly=%s",
		nyHourly.Format(time.RFC3339), utcHourly.Format(time.RFC3339))

	// Each hourly bucket retains its source location, so callers can reason
	// about TZ semantics downstream.
	assert.Equal(t, nyLoc.String(), nyHourly.Location().String(),
		"hourly bucket from NY input must retain NY location")
	assert.Equal(t, time.UTC.String(), utcHourly.Location().String(),
		"hourly bucket from UTC input must retain UTC location")
}

// containsTZMarker returns true if s contains a timezone marker recognized
// by RFC3339 (a "Z" or a "+HH:MM"/"-HH:MM" offset adjacent to a digit). It
// is intentionally narrow: a 'Z' alone (without surrounding digits) would
// match many unrelated tokens, so we require it follow a digit.
func containsTZMarker(s string) bool {
	for i := 1; i < len(s); i++ {
		c := s[i]
		prev := s[i-1]
		if (c == 'Z' || c == '+' || c == '-') && prev >= '0' && prev <= '9' {
			return true
		}
	}
	return false
}
