package pumps

import (
	"context"
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
	// a.IPAddress = "192.168.99.100"
	a.Tags = []string{"tag-1", "tag-2"}
	a.ExpireAt = time.Date(2020, time.November, 10, 23, 0, 0, 0, time.UTC)

	return a
}

func TestSegmentPump(t *testing.T) {
	t.Skip("Set the tWriteKey and remove Skip to test.")

	tWriteKey := "XXX"
	tConf := make(map[string]string)
	tConf["segment_write_key"] = tWriteKey

	s := SegmentPump{}

	err := s.Init(tConf)
	if err != nil {
		t.Error(err)
	}

	tRecord := CreateAnalyticsRecord()
	tData := make([]interface{}, 1)
	tData[0] = tRecord
	s.segmentClient.Verbose = true
	s.segmentClient.Interval = 10 * time.Millisecond
	s.segmentClient.Size = 1

	go s.WriteData(context.TODO(), tData)

	time.Sleep(time.Second)
}
