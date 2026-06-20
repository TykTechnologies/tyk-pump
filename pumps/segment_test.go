package pumps

import (
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

func CreateAnalyticsRecord() analytics.AnalyticsRecord {
	a := analytics.AnalyticsRecord{}
	a.Method = "POST"
	a.Path = "/v1/resource"
	a.ContentLength = 123
	a.UserAgent = "Test User Agent"
	a.Day = 1
	a.Month = time.January
	a.Year = 2016
	a.Hour = 14
	a.ResponseCode = 202
	a.APIKey = "APIKEY123"
	a.TimeStamp = time.Now()
	a.APIVersion = "1"
	a.APIName = "Test API"
	a.APIID = "API123"
	a.OrgID = "ORG123"
	a.OauthID = "Oauth123"
	a.RequestTime = time.Now().Unix()
	a.RawRequest = "{\"field\": \"value\"}"
	a.RawResponse = "{\"id\": \"123\"}"
	//a.IPAddress = "192.168.99.100"
	a.Tags = []string{"tag-1", "tag-2"}
	a.ExpireAt = time.Date(2020, time.November, 10, 23, 0, 0, 0, time.UTC)

	return a
}

// SW-REQ-053:nominal:nominal
func TestSegmentPump_ToJSONMap_MarshalsAnalyticsRecord(t *testing.T) {
	s := SegmentPump{}
	record := CreateAnalyticsRecord()

	got, err := s.ToJSONMap(record)
	if err != nil {
		t.Fatal(err)
	}

	if got["api_id"] != record.APIID {
		t.Fatalf("api_id = %v, want %q", got["api_id"], record.APIID)
	}
	if got["api_key"] != record.APIKey {
		t.Fatalf("api_key = %v, want %q", got["api_key"], record.APIKey)
	}
	if got["raw_request"] != record.RawRequest {
		t.Fatalf("raw_request = %v, want %q", got["raw_request"], record.RawRequest)
	}
}
