package analytics

import (
	b64 "encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	a := AnalyticsRecordAggregate{}
	newA := a.New()
	if newA.APIID == nil && newA.ApiEndpoint == nil {
		t.Fatal("New should have initialised APIID and ApiEndpoint")
	}
}

func TestDoHash(t *testing.T) {
	res := doHash("test")
	resDecoded, _ := b64.StdEncoding.DecodeString(res + "==")
	if string(resDecoded) != "test" {
		t.Fatal("Decoded Hash should be 'test'.")
	}
}

func TestIgnoreTag(t *testing.T) {
	tag := "key-test"
	prefixList := []string{"t-"}
	res := ignoreTag(tag, prefixList)

	if res == false {
		t.Fatal("ignoreTag should be true when tag has a key- prefix")
	}

	tag = "t-test"
	res = ignoreTag(tag, prefixList)
	if res == false {
		t.Fatal("ignoreTag should be true when tag has a prefix in the prefixList")
	}

	tag = "nest"
	res = ignoreTag(tag, prefixList)
	if res == true {
		t.Fatal("ignoreTag should be false when it has no tags to ignore.s")
	}
}

func TestReplaceUnsupportedChars(t *testing.T) {
	path := "/test.no"
	res := replaceUnsupportedChars(path)
	if strings.Contains(res, ".") {
		t.Fatal("replaceUnsupportedChars should replace the dots.")
	}
}

func TestAggregateData(t *testing.T) {
	records := []interface{}{
		AnalyticsRecord{APIID: "api-1", OrgID: "org-1", IPAddress: "127.0.0.1", Method: "GET", Path: "/test", TimeStamp: time.Now(),
			RequestTime: 100, ResponseCode: 200, OauthID: "oauth-1", APIVersion: "default", Tags: []string{"test"}},
		AnalyticsRecord{APIID: "api-2", OrgID: "org-1", IPAddress: "127.0.0.1", Method: "GET", Path: "/test", TimeStamp: time.Now(),
			RequestTime: 100, ResponseCode: 200, OauthID: "oauth-1", APIVersion: "default", Tags: []string{"test"}},
		AnalyticsRecord{APIID: "api-1", OrgID: "org-1", IPAddress: "127.0.0.1", Method: "GET", Path: "/test", TimeStamp: time.Now(),
			RequestTime: 200, ResponseCode: 500, OauthID: "oauth-1", APIVersion: "default", Tags: []string{"test"}},
		AnalyticsRecord{APIID: "api-1", OrgID: "org-1", IPAddress: "127.0.0.1", Method: "GET", Path: "/test", TimeStamp: time.Now(),
			RequestTime: 100, ResponseCode: 500, OauthID: "oauth-1", APIVersion: "default", Tags: []string{"test2"}},
		AnalyticsRecord{APIID: "api-3", OrgID: "org-2", IPAddress: "127.0.0.1", Method: "GET", Path: "/test", TimeStamp: time.Now(),
			RequestTime: 100, ResponseCode: 500, OauthID: "oauth-1", APIVersion: "default", Tags: []string{"test2"}},
		AnalyticsRecord{APIID: "api-2", OrgID: "org-2", IPAddress: "127.0.0.1", Method: "GET", Path: "/test_2", TimeStamp: time.Now(),
			RequestTime: 100, ResponseCode: 200, OauthID: "oauth-2", APIVersion: "v1", Tags: []string{"test3"}},
		AnalyticsRecord{APIID: "api-5", OrgID: "", IPAddress: "127.0.0.1", Method: "GET", Path: "/test_2", TimeStamp: time.Now(),
			RequestTime: 100, ResponseCode: 200, OauthID: "oauth-2", APIVersion: "v1", Tags: []string{"test3"}},
	}

	results := AggregateData(records, true, []string{}, true)
	if len(results) != 2 {
		t.Fatal("AggregatedData map should have 2 orgs")
	}
	org1Res := results["org-1"]
	t.Run("org-1-errors", func(t *testing.T) {
		if len(org1Res.Errors) != 1 {
			t.Fatal("org1Res should have only 500 type err.")
		}
		errors500 := org1Res.Errors["500"]
		if errors500.Hits != 2 || errors500.ErrorTotal != 2 {
			t.Fatal("org1Res should have 2 hits  on 500 errs.")
		}
		if errors500.RequestTime != 150 || errors500.TotalRequestTime != 300 {
			t.Fatal("org1Res errors have miscalculated requests times.")
		}
	})

	t.Run("org-1-apiid", func(t *testing.T) {
		if len(org1Res.APIID) != 2 {
			t.Fatal("org1Res should have 2 apiids.")
		}
		api1 := org1Res.APIID["api-1"]
		if api1.Hits != 3 || api1.Success != 1 || api1.ErrorTotal != 2 || api1.TotalRequestTime != 400 {
			t.Fatal("Miscalculated aggregations for api-1.")
		}
		api2 := org1Res.APIID["api-2"]
		if api2.Hits != 1 || api2.Success != 1 || api2.ErrorTotal != 0 || api2.TotalRequestTime != 100 {
			t.Fatal("Miscalculated aggregations for api-2.")
		}
	})

	t.Run("org-1-apiendpoint", func(t *testing.T) {
		if len(org1Res.ApiEndpoint) != 2 {
			t.Fatal("org1Res should have 2 api endpoints.")
		}

		endpointTest := org1Res.ApiEndpoint["6170692d313a64656661756c743a2f74657374"]
		if endpointTest.Hits != 3 || endpointTest.Success != 1 || endpointTest.ErrorTotal != 2 || endpointTest.HumanIdentifier != "/test" {
			t.Fatal("Miscalculated aggregations for endpoint /test. ")
		}
	})
}
