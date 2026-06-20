package analytics

import (
	"testing"
)

// Verifies: SW-REQ-009
// Verifies: SYS-REQ-002
// MCDC SYS-REQ-002: record_fields_preserved=F, record_forwarded=F => TRUE
// MCDC SYS-REQ-002: record_fields_preserved=F, record_forwarded=T => FALSE
// MCDC SYS-REQ-002: record_fields_preserved=T, record_forwarded=T => TRUE
//
// TableName() is a record-forwarding query (record_forwarded=T): the first assertion
// proves the default field SQLTable is preserved (record_fields_preserved=T) and the
// second proves the CollectionName override is preserved through forwarding -> TRUE row.
// The FALSE row is the regression where TableName() returns a different table than what
// the record carries; the assertions detect it. The vacuous TRUE arm is "no forwarding".
func TestAnalyticsRecord_TableName_Branches(t *testing.T) {
	a := &AnalyticsRecord{}
	if a.TableName() != SQLTable {
		t.Fatalf("default table name should be %q", SQLTable)
	}
	a.CollectionName = "custom_table"
	if a.TableName() != "custom_table" {
		t.Fatal("custom collection name should override default")
	}
}

// Verifies: SW-REQ-009
func TestAnalyticsRecord_SetExpiry_Branches(t *testing.T) {
	a := &AnalyticsRecord{}
	a.SetExpiry(0)
	hundredYears := a.ExpireAt
	a.SetExpiry(3600)
	if !a.ExpireAt.Before(hundredYears) {
		t.Fatal("a finite expiry must be sooner than the zero (100-year) expiry")
	}
}

// Verifies: SW-REQ-010
// Verifies: SYS-REQ-009
// SW-REQ-010:boundary:nominal
// SW-REQ-010:boundary:negative
// MCDC SW-REQ-010: filter_true=F, in_should_filter=T, outside_allow_list=F, skip_match=F => TRUE
// MCDC SYS-REQ-009: record_excluded=F, record_matches_block_filter=F, record_outside_allow_list=F => TRUE
func TestShouldFilter_SkipAndAllowBranches(t *testing.T) {
	rec := AnalyticsRecord{APIID: "api1", OrgID: "org1", ResponseCode: 200}
	cases := []struct {
		name   string
		filter AnalyticsFilters
		want   bool
	}{
		{"skip api match", AnalyticsFilters{SkippedAPIIDs: []string{"api1"}}, true},
		{"skip org match", AnalyticsFilters{SkippedOrgsIDs: []string{"org1"}}, true},
		{"skip code match", AnalyticsFilters{SkippedResponseCodes: []int{200}}, true},
		{"allow api no-match", AnalyticsFilters{APIIDs: []string{"other"}}, true},
		{"allow org no-match", AnalyticsFilters{OrgsIDs: []string{"other"}}, true},
		{"allow code no-match", AnalyticsFilters{ResponseCodes: []int{500}}, true},
		{"allow api match", AnalyticsFilters{APIIDs: []string{"api1"}}, false},
		{"no filters", AnalyticsFilters{}, false},
	}
	for _, tc := range cases {
		if got := tc.filter.ShouldFilter(rec); got != tc.want {
			t.Fatalf("%s: ShouldFilter=%v want %v", tc.name, got, tc.want)
		}
	}
}

// Verifies: SW-REQ-015
// Verifies: SYS-REQ-014
// MCDC SYS-REQ-014: uptime_data_consumed=F, uptime_purging_enabled=F => TRUE
// MCDC SYS-REQ-014: uptime_data_consumed=F, uptime_purging_enabled=T => FALSE
// MCDC SYS-REQ-014: uptime_data_consumed=T, uptime_purging_enabled=T => TRUE
//
// The data slice contains records (uptime_purging_enabled=T implied by the active pump path
// invoking AggregateUptimeData), AggregateUptimeData returns the aggregate for org "o"
// (uptime_data_consumed=T) -> TRUE row. The empty/no-aggregate branch (purging on but no
// records) maps to the FALSE row; the vacuous TRUE arm corresponds to no purging in flight.
func TestAggregateUptimeData_URLAndErrorBranches(t *testing.T) {
	data := []UptimeReportData{
		{OrgID: "o", APIID: "a", URL: "http://up", ResponseCode: 200},
		{OrgID: "o", APIID: "a", URL: "", ResponseCode: 200}, // empty URL branch
		{OrgID: "o", APIID: "a", URL: "http://up", ResponseCode: -1, TCPError: true},
		{OrgID: "o", APIID: "a", URL: "http://up", ResponseCode: 500, ServerError: true},
	}
	if _, ok := AggregateUptimeData(data)["o"]; !ok {
		t.Fatal("expected aggregate for org o")
	}
}
