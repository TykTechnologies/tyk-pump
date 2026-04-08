package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
