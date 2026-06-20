package pumps

import (
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

// SW-REQ-058:dst_transition_safe:boundary
//
// Contract: mongo aggregate window math (analytics.AggregateData /
// setAggregateTimestamp) is DST-immune when records are timestamped in UTC.
// Records straddling a DST boundary (spring-forward gap or fall-back
// ambiguous hour in any local zone) must NOT:
//   - be lost (window skipping), or
//   - be collapsed into a single window (window double-counting), or
//   - produce non-deterministic results on repeated invocation.
//
// Tyk-pump's hot path stores AnalyticsRecord.TimeStamp as time.Time. The
// production code at analytics/aggregate.go:1063 (setAggregateTimestamp)
// uses asTime.Location() preserved from the input record. The structural
// contract this test pins down is:
//
//	Given input records timestamped via time.Date(..., time.UTC), the
//	aggregation buckets are derived from UTC arithmetic and are therefore
//	DST-immune by construction.
//
// If a future regression introduces .Local() conversion or wall-clock-based
// bucketing, this test reveals it.
//
func TestDstTransitionSafe_SpringForwardBoundary(t *testing.T) {
	// US Eastern spring-forward 2026: at 2026-03-08T02:00:00 EST local time
	// clocks jump to 03:00 EDT. The instant 2026-03-08T07:00:00Z is 02:00 EST
	// (just before the jump); 2026-03-08T08:00:00Z is 04:00 EDT (after the jump)
	// in wall-clock terms even though it is one hour later in absolute time.
	//
	// We construct records straddling that boundary IN UTC to prove the
	// aggregator treats them as distinct UTC hours regardless of the
	// "missing" wall-clock hour in any local TZ.

	preGap := time.Date(2026, 3, 8, 6, 30, 0, 0, time.UTC)  // 01:30 EST
	atGap := time.Date(2026, 3, 8, 7, 30, 0, 0, time.UTC)   // 02:30 — does not exist in EST wall clock
	postGap := time.Date(2026, 3, 8, 8, 30, 0, 0, time.UTC) // 04:30 EDT

	mkRec := func(ts time.Time, orgID string) analytics.AnalyticsRecord {
		return analytics.AnalyticsRecord{
			OrgID:        orgID,
			APIID:        "api-dst",
			APIName:      "dst-test",
			ResponseCode: 200,
			TimeStamp:    ts,
		}
	}

	// Use a unique orgID per record so each one creates its own aggregate
	// entry and we can observe the TimeStamp the aggregator assigned.
	records := []interface{}{
		mkRec(preGap, "org-pre"),
		mkRec(atGap, "org-at"),
		mkRec(postGap, "org-post"),
	}

	// aggregationTime=60 => hourly bucket via setAggregateTimestamp's
	// 60-minute fast path (asTime.Hour() rounded down).
	// dbIdentifier="" => non-Mongo path (deterministic, no lastDocumentTS state).
	got := analytics.AggregateData(records, false, nil, "", 60)

	if len(got) != 3 {
		t.Fatalf("expected 3 distinct org aggregates (no merging across UTC boundary), got %d: %+v", len(got), got)
	}

	expect := map[string]time.Time{
		"org-pre":  time.Date(2026, 3, 8, 6, 0, 0, 0, time.UTC),
		"org-at":   time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC),
		"org-post": time.Date(2026, 3, 8, 8, 0, 0, 0, time.UTC),
	}

	for org, wantTS := range expect {
		agg, ok := got[org]
		if !ok {
			t.Errorf("missing aggregate for %s", org)
			continue
		}
		if !agg.TimeStamp.Equal(wantTS) {
			t.Errorf("%s: aggregate bucket = %s; want %s (hourly UTC truncation)",
				org, agg.TimeStamp.UTC(), wantTS)
		}
		// Bucket must be in UTC location (or at least equivalent offset)
		// so downstream consumers don't depend on host TZ.
		_, offset := agg.TimeStamp.Zone()
		if offset != 0 {
			t.Errorf("%s: aggregate TimeStamp zone offset = %d; want 0 (UTC) "+
				"— production must preserve UTC location for DST-immune bucketing", org, offset)
		}
	}
}

// SW-REQ-058:dst_transition_safe:boundary
//
// Fall-back ambiguous-hour arm: at the US Eastern fall-back transition
// 2026-11-01T02:00 EDT clocks roll back to 01:00 EST, so the wall-clock
// hour 01:00-02:00 occurs TWICE. Two records with distinct UTC instants
// inside that ambiguous wall-clock hour MUST land in distinct UTC hourly
// buckets (proving the aggregator is not double-counting on wall-clock
// collision).
func TestDstTransitionSafe_FallBackAmbiguousHour(t *testing.T) {
	// 2026-11-01T05:30:00Z = 01:30 EDT (first 01:30)
	// 2026-11-01T06:30:00Z = 01:30 EST (second 01:30 after fall-back)
	first := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
	second := time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC)

	mkRec := func(ts time.Time, orgID string) analytics.AnalyticsRecord {
		return analytics.AnalyticsRecord{
			OrgID:        orgID,
			APIID:        "api-fb",
			APIName:      "fallback-test",
			ResponseCode: 200,
			TimeStamp:    ts,
		}
	}

	records := []interface{}{
		mkRec(first, "org-first"),
		mkRec(second, "org-second"),
	}

	got := analytics.AggregateData(records, false, nil, "", 60)

	if len(got) != 2 {
		t.Fatalf("expected 2 distinct org aggregates across fall-back boundary, got %d", len(got))
	}

	if got["org-first"].TimeStamp.Equal(got["org-second"].TimeStamp) {
		t.Errorf("fall-back ambiguous wall-clock hour collapsed two UTC instants "+
			"into the same bucket: first=%s second=%s — production must bucket on UTC",
			got["org-first"].TimeStamp.UTC(), got["org-second"].TimeStamp.UTC())
	}

	wantFirst := time.Date(2026, 11, 1, 5, 0, 0, 0, time.UTC)
	wantSecond := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	if !got["org-first"].TimeStamp.Equal(wantFirst) {
		t.Errorf("first ambiguous-hour bucket = %s; want %s", got["org-first"].TimeStamp.UTC(), wantFirst)
	}
	if !got["org-second"].TimeStamp.Equal(wantSecond) {
		t.Errorf("second ambiguous-hour bucket = %s; want %s", got["org-second"].TimeStamp.UTC(), wantSecond)
	}
}

// SW-REQ-058:dst_transition_safe:nominal
// SW-REQ-058:determinism:nominal
//
// Determinism arm: running AggregateData twice on the same UTC inputs
// produces identical bucket assignments. Catches any hidden dependency
// on time.Now()/host TZ that would make DST-window math non-reproducible.
func TestDstTransitionSafe_Deterministic(t *testing.T) {
	ts := time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC)
	mk := func() []interface{} {
		return []interface{}{
			analytics.AnalyticsRecord{OrgID: "o1", APIID: "a", ResponseCode: 200, TimeStamp: ts},
		}
	}

	first := analytics.AggregateData(mk(), false, nil, "", 60)
	second := analytics.AggregateData(mk(), false, nil, "", 60)

	if !first["o1"].TimeStamp.Equal(second["o1"].TimeStamp) {
		t.Errorf("non-deterministic bucket assignment across runs: first=%s second=%s",
			first["o1"].TimeStamp, second["o1"].TimeStamp)
	}
}
