package analytics

import (
	"testing"

	"github.com/TykTechnologies/tyk-pump/analyticspb"
)

func TestShouldFilter(t *testing.T) {
	record := analyticspb.AnalyticsRecord{
		APIID:        "apiid123",
		OrgID:        "orgid123",
		ResponseCode: 200,
	}

	//test skip_api_ids
	filter := AnalyticsFilters{
		SkippedAPIIDs: []string{"apiid123"},
	}
	shouldFilter := filter.ShouldFilter(record)
	if shouldFilter == false {
		t.Fatal("filter should be filtering the record")
	}

	//test skip_org_ids
	filter = AnalyticsFilters{
		SkippedOrgsIDs: []string{"orgid123"},
	}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == false {
		t.Fatal("filter should be filtering the record")
	}

	//test skip_response_codes
	filter = AnalyticsFilters{
		SkippedResponseCodes: []int{200},
	}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == false {
		t.Fatal("filter should be filtering the record")
	}

	//test api_ids
	filter = AnalyticsFilters{
		APIIDs: []string{"apiid123"},
	}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == true {
		t.Fatal("filter should not be filtering the record")
	}

	//test org_ids
	filter = AnalyticsFilters{
		OrgsIDs: []string{"orgid123"},
	}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == true {
		t.Fatal("filter should not be filtering the record")
	}

	//test response_codes
	filter = AnalyticsFilters{
		ResponseCodes: []int{200},
	}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == true {
		t.Fatal("filter should not be filtering the record")
	}

	//test different org_ids
	filter = AnalyticsFilters{
		OrgsIDs: []string{"orgid321"},
	}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == false {
		t.Fatal("filter should be filtering the record")
	}

	//test different api_ids
	filter = AnalyticsFilters{
		APIIDs: []string{"apiid231"},
	}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == false {
		t.Fatal("filter should be filtering the record")
	}

	//test different response_codes
	filter = AnalyticsFilters{
		ResponseCodes: []int{201},
	}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == false {
		t.Fatal("filter should be filtering the record")
	}

	//test no filter
	filter = AnalyticsFilters{}
	shouldFilter = filter.ShouldFilter(record)
	if shouldFilter == true {
		t.Fatal("filter should not be filtering the record")
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
