package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Verifies: SW-REQ-010
// Verifies: SYS-REQ-009
// Verifies: STK-REQ-003
// MCDC SW-REQ-010: filter_true=F, in_should_filter=T, outside_allow_list=F, skip_match=T => FALSE
// MCDC SW-REQ-010: filter_true=F, in_should_filter=T, outside_allow_list=T, skip_match=F => FALSE
// MCDC SW-REQ-010: filter_true=F, in_should_filter=T, outside_allow_list=F, skip_match=F => TRUE
// MCDC SYS-REQ-009: record_excluded=F, record_matches_block_filter=F, record_outside_allow_list=F => TRUE
// MCDC SYS-REQ-009: record_excluded=F, record_matches_block_filter=F, record_outside_allow_list=T => FALSE
// MCDC SYS-REQ-009: record_excluded=F, record_matches_block_filter=T, record_outside_allow_list=F => FALSE
// MCDC SYS-REQ-009: record_excluded=F, record_matches_block_filter=T, record_outside_allow_list=T => FALSE
// MCDC SYS-REQ-009: record_excluded=T, record_matches_block_filter=T, record_outside_allow_list=T => TRUE
//
// Sub-case "skip_apiids"/"skip_org_ids"/"skip_response_codes" sets skip_match=T (block list
// matches) -> ShouldFilter returns true -> record_excluded=T, record_matches_block_filter=T,
// record_outside_allow_list=T (the record is also not in any explicit allow list) -> TRUE row.
// Sub-cases "api_ids"/"org_ids"/"response_codes" set outside_allow_list=F (record is in the
// allow list) -> ShouldFilter returns false (record_excluded=F, record_matches_block_filter=F,
// record_outside_allow_list=F) -> vacuous TRUE for SYS-REQ-009 / TRUE row for SW-REQ-010.
// Sub-cases "different api_ids"/"different org_ids"/"different response_codes" set
// outside_allow_list=T (allow list set but record doesn't match) -> ShouldFilter returns true
// (record_excluded=F arm with record_outside_allow_list=T) -> FALSE row for SYS-REQ-009.
// "no filter" sub-case is the vacuous TRUE arm (no triggers).
func TestShouldFilter(t *testing.T) {
	record := AnalyticsRecord{
		APIID:        "apiid123",
		OrgID:        "orgid123",
		ResponseCode: 200,
	}

	tcs := []struct {
		testName          string
		filter            AnalyticsFilters
		expectedFiltering bool
	}{
		{
			testName: "skip_apiids",
			filter: AnalyticsFilters{
				SkippedAPIIDs: []string{"apiid123"},
			},
			expectedFiltering: true,
		},
		{
			testName: "skip_org_ids",
			filter: AnalyticsFilters{
				SkippedOrgsIDs: []string{"orgid123"},
			},
			expectedFiltering: true,
		},
		{
			testName: "skip_response_codes",
			filter: AnalyticsFilters{
				SkippedResponseCodes: []int{200},
			},
			expectedFiltering: true,
		},
		{
			testName: "api_ids",
			filter: AnalyticsFilters{
				APIIDs: []string{"apiid123"},
			},
			expectedFiltering: false,
		},
		{
			testName: "org_ids",
			filter: AnalyticsFilters{
				OrgsIDs: []string{"orgid123"},
			},
			expectedFiltering: false,
		},
		{
			testName: "response_codes",
			filter: AnalyticsFilters{
				ResponseCodes: []int{200},
			},
			expectedFiltering: false,
		},
		{
			testName: "different org_ids",
			filter: AnalyticsFilters{
				OrgsIDs: []string{"orgid321"},
			},
			expectedFiltering: true,
		},
		{
			testName: "different api_ids",
			filter: AnalyticsFilters{
				APIIDs: []string{"apiid231"},
			},
			expectedFiltering: true,
		},
		{
			testName: "different response_codes",
			filter: AnalyticsFilters{
				ResponseCodes: []int{201},
			},
			expectedFiltering: true,
		},
		{
			testName:          "no filter",
			filter:            AnalyticsFilters{},
			expectedFiltering: false,
		},
		{
			testName: "multiple filter",
			filter: AnalyticsFilters{
				ResponseCodes: []int{200},
				APIIDs:        []string{"apiid123"},
			},
			expectedFiltering: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			shouldFilter := tc.filter.ShouldFilter(record)
			assert.Equal(t, tc.expectedFiltering, shouldFilter)
		})
	}
}

// Verifies: SW-REQ-010
// MCDC SW-REQ-010: filter_true=T, in_should_filter=F, outside_allow_list=F, skip_match=F => TRUE
//
// The second assertion (HasFilter()==true on AnalyticsFilters{APIIDs:{"api123"}}) demonstrates
// the filter_true=T row when none of skip_match / outside_allow_list / in_should_filter
// have triggered yet (filter is configured but ShouldFilter has not been invoked on a
// matching record): the implication antecedent is FALSE so the formula evaluates to TRUE.
func TestHasFilter(t *testing.T) {
	filter := AnalyticsFilters{}

	hasFilter := filter.HasFilter()
	if hasFilter == true {
		t.Fatal("Has filter should be false.")
	}

	filter = AnalyticsFilters{
		APIIDs: []string{"api123"},
	}
	hasFilter = filter.HasFilter()
	if hasFilter == false {
		t.Fatal("HasFilter should be true.")
	}
}
