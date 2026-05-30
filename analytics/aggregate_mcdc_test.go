package analytics

import (
	"testing"
	"time"
)

// Verifies: SW-REQ-011
func mcdcRecord(org, apiID, apiVer, apiKey, oauth, iso string, code int, total, up int64, track bool, tags []string) AnalyticsRecord {
	r := AnalyticsRecord{
		OrgID:        org,
		APIID:        apiID,
		APIVersion:   apiVer,
		APIKey:       apiKey,
		OauthID:      oauth,
		ResponseCode: code,
		TrackPath:    track,
		Tags:         tags,
		TimeStamp:    time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
	}
	r.Latency = Latency{Total: total, Upstream: up}
	r.Geo.Country.ISOCode = iso
	return r
}

// Verifies: SW-REQ-011
// Verifies: SYS-REQ-003
// SW-REQ-011:monotonicity:negative
func TestAggregateData_MCDCBranches(t *testing.T) {
	// Records crafted to exercise incrementAggregate / incrementOrSetUnit decision
	// branches: success vs error codes, min/max latency updates in both directions,
	// present/absent APIVersion/APIKey/OauthID, geo, normal and ignored tags, and
	// both TrackPath values, across repeated API IDs so per-unit counters merge.
	records := []interface{}{
		mcdcRecord("org1", "api1", "v1", "key1", "oauth1", "US", 200, 100, 60, true, []string{"t1", "key-secret", ""}),
		mcdcRecord("org1", "api1", "v1", "key1", "oauth1", "US", 200, 50, 30, false, []string{"t1"}),   // lower latency -> min update
		mcdcRecord("org1", "api1", "v2", "key2", "oauth2", "", 500, 200, 120, true, []string{"t2"}),    // error path, higher latency -> max
		mcdcRecord("org1", "api2", "", "", "", "GB", 404, 80, 40, false, nil),                          // error, absent optional fields
		mcdcRecord("org2", "api3", "v1", "key3", "", "FR", 301, 70, 35, true, []string{"key-x", "t3"}), // 3xx, ignored-prefix tag
		mcdcRecord("", "apiX", "", "", "", "", 200, 10, 5, false, nil),                                 // empty org -> skipped
	}

	for _, track := range []bool{true, false} {
		agg := AggregateData(records, track, []string{"key-"}, "", 60)
		if _, ok := agg["org1"]; !ok {
			t.Fatalf("expected org1 aggregate (trackAllPaths=%v)", track)
		}
		if _, ok := agg[""]; ok {
			t.Fatal("empty-org record must be skipped")
		}
		if agg["org1"].Total.Hits == 0 {
			t.Fatal("expected org1 hits to accumulate")
		}
		if agg["org1"].Total.ErrorTotal == 0 {
			t.Fatal("expected org1 error total to accumulate from 4xx/5xx records")
		}
	}
}

// Verifies: SW-REQ-010
// Verifies: SYS-REQ-009
// SW-REQ-010:boundary:negative
func TestHasFilter_EachList(t *testing.T) {
	if (AnalyticsFilters{}).HasFilter() {
		t.Fatal("empty filter set must report no filter")
	}
	cases := []AnalyticsFilters{
		{APIIDs: []string{"a"}},
		{OrgsIDs: []string{"o"}},
		{ResponseCodes: []int{200}},
		{SkippedAPIIDs: []string{"a"}},
		{SkippedOrgsIDs: []string{"o"}},
		{SkippedResponseCodes: []int{500}},
	}
	for i, f := range cases {
		if !f.HasFilter() {
			t.Fatalf("case %d: expected HasFilter true", i)
		}
	}
}

// Verifies: SW-REQ-015
// Verifies: SYS-REQ-014
// SW-REQ-015:nominal:negative
func TestAggregateUptimeData_MCDCBranches(t *testing.T) {
	data := []UptimeReportData{
		{OrgID: "org1", APIID: "api1", URL: "http://a", ResponseCode: 200, RequestTime: 10},
		{OrgID: "org1", APIID: "api1", URL: "http://a", ResponseCode: 500, ServerError: true, RequestTime: 20},
		{OrgID: "org1", APIID: "api1", URL: "http://a", ResponseCode: -1, TCPError: true, RequestTime: 30},
		{OrgID: "org2", APIID: "api2", URL: "", ResponseCode: 200, RequestTime: 5},
		{OrgID: "", APIID: "apiX", URL: "http://x", ResponseCode: 200, RequestTime: 1}, // empty org -> skipped
	}
	agg := AggregateUptimeData(data)
	if _, ok := agg["org1"]; !ok {
		t.Fatal("expected org1 uptime aggregate")
	}
	if _, ok := agg[""]; ok {
		t.Fatal("empty-org uptime record must be skipped")
	}
}
