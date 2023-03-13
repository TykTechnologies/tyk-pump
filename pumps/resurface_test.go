package pumps

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

const (
	rawReq string = "R0VUIGdldCBIVFRQLzEuMQ0KSG9zdDogbG9jYWxob3N0OjgwODANClVzZXItQWdlbnQ6IE1vemls" +
		"bGEvNS4wIChYMTE7IFVidW50dTsgTGludXggeDg2XzY0OyBydjo5MS4wKSBHZWNrby8yMDEwMDEwMSBGaXJlZm94" +
		"LzkxLjANCkFjY2VwdDogdGV4dC9odG1sLGFwcGxpY2F0aW9uL3hodG1sK3htbCxhcHBsaWNhdGlvbi94bWw7cT0w" +
		"LjksaW1hZ2Uvd2VicCwqLyo7cT0wLjgNCkFjY2VwdC1FbmNvZGluZzogZ3ppcCwgZGVmbGF0ZQ0KQWNjZXB0LUxh" +
		"bmd1YWdlOiBlbi1VUyxlbjtxPTAuNQ0KU2VjLUZldGNoLURlc3Q6IGRvY3VtZW50DQpTZWMtRmV0Y2gtTW9kZTog" +
		"bmF2aWdhdGUNClNlYy1GZXRjaC1TaXRlOiBub25lDQo="

	rawResp string = "SFRUUC8xLjEgMjAwIE9LDQpDb250ZW50LUxlbmd0aDogNDI5DQpBY2Nlc3MtQ29udHJvbC1BbGxv" +
		"dy1DcmVkZW50aWFsczogdHJ1ZQ0KQWNjZXNzLUNvbnRyb2wtQWxsb3ctT3JpZ2luOiAqDQpDb250ZW50LVR5cGU6I" +
		"GFwcGxpY2F0aW9uL2pzb24NCkRhdGU6IFR1ZSwgMTYgTWF5IDIwMjAgMjA6NDA6NDUgR01UDQpTZXJ2ZXI6IGd1bm" +
		"ljb3JuLzE5LjkuMA0KWC1SYXRlbGltaXQtTGltaXQ6IDANClgtUmF0ZWxpbWl0LVJlbWFpbmluZzogMA0KWC1SYXR" +
		"lbGltaXQtUmVzZXQ6IDANCg0Kew0KICAic2xpZGVzaG93Ijogew0KICAgICJhdXRob3IiOiAiWW91cnMgVHJ1bHki" +
		"LCANCiAgICAiZGF0ZSI6ICJkYXRlIG9mIHB1YmxpY2F0aW9uIiwgDQogICAgInNsaWRlcyI6IFsNCiAgICAgIHsNC" +
		"iAgICAgICAgInRpdGxlIjogIldha2UgdXAgdG8gV29uZGVyV2lkZ2V0cyEiLCANCiAgICAgICAgInR5cGUiOiAiYW" +
		"xsIg0KICAgICAgfSwgDQogICAgICB7DQogICAgICAgICJpdGVtcyI6IFsNCiAgICAgICAgICAiV2h5IDxlbT5Xb25" +
		"kZXJXaWRnZXRzPC9lbT4gYXJlIGdyZWF0IiwgDQogICAgICAgICAgIldobyA8ZW0+YnV5czwvZW0+IFdvbmRlcldp" +
		"ZGdldHMiDQogICAgICAgIF0sIA0KICAgICAgICAidGl0bGUiOiAiT3ZlcnZpZXciLCANCiAgICAgICAgInR5cGUiO" +
		"iAiYWxsIg0KICAgICAgfQ0KICAgIF0sIA0KICAgICJ0aXRsZSI6ICJTYW1wbGUgU2xpZGUgU2hvdyINCiAgfQ0KfQ" +
		"=="

	rawRespOneChunk string = "SFRUUC8xLjEgMjAwIE9LDQpDb250ZW50LVR5cGU6IHRleHQvcGxhaW4NClRyYW5zZmVy" +
		"LUVuY29kaW5nOiBjaHVua2VkDQoNCjcNCg0KTW96aWxsYQ0KDQoxMQ0KDQpEZXZlbG9wZXIgTmV0d29yaw0KDQowD" +
		"QoNCg0KDQo="

	rawRespChunks string = "SFRUUC8xLjEgMjAwIE9LDQpUcmFuc2Zlci1FbmNvZGluZzogY2h1bmtlZA0KQ29udGVudC" +
		"1UeXBlOiB0ZXh0L2h0bWwNCg0KYw0KPGgxPmdvITwvaDE+DQoNCjFiDQo8aDE+Zmlyc3QgY2h1bmsgbG9hZGVkPC9" +
		"oMT4NCg0KMmENCjxoMT5zZWNvbmQgY2h1bmsgbG9hZGVkIGFuZCBkaXNwbGF5ZWQ8L2gxPg0KDQoyOQ0KPGgxPnRo" +
		"aXJkIGNodW5rIGxvYWRlZCBhbmQgZGlzcGxheWVkPC9oMT4NCg0KMA0K"

	rawRespChunksTrailer string = "SFRUUC8xLjEgMjAwIE9LDQpUcmFuc2Zlci1FbmNvZGluZzogY2h1bmtlZA0KQ29" +
		"udGVudC1UeXBlOiB0ZXh0L2h0bWwNClRyYWlsZXI6IEV4cGlyZXMNCg0KYw0KPGgxPmdvITwvaDE+DQoNCjFiDQo8" +
		"aDE+Zmlyc3QgY2h1bmsgbG9hZGVkPC9oMT4NCg0KMmENCjxoMT5zZWNvbmQgY2h1bmsgbG9hZGVkIGFuZCBkaXNwb" +
		"GF5ZWQ8L2gxPg0KDQoyOQ0KPGgxPnRoaXJkIGNodW5rIGxvYWRlZCBhbmQgZGlzcGxheWVkPC9oMT4NCg0KMA0KRX" +
		"hwaXJlczogV2VkLCAyMSBPY3QgMjAxNSAwNzoyODowMCBHTVQNCg0K"
)

func SetUp(t *testing.T, url string, queue []string, rules string) (*ResurfacePump, map[string]interface{}) {
	pmp := ResurfacePump{}
	cfg := make(map[string]interface{})
	cfg["capture_url"] = url
	cfg["queue"] = queue
	cfg["rules"] = rules

	err := pmp.Init(cfg)
	assert.Nil(t, err, "Problem initializing "+pmp.GetName())

	return &pmp, cfg
}

func TestResurfaceInit(t *testing.T) {
	pmp, cfg := SetUp(t, "http://localhost:7701/message", nil, "include debug")
	assert.NotNil(t, pmp.logger)
	assert.True(t, pmp.logger.Enabled())

	// Checking with invalid config
	cfg["capture_url"] = "not a valid URL"
	pmp2 := ResurfacePump{}
	err2 := pmp2.Init(cfg)
	assert.NotNil(t, err2)
	assert.False(t, pmp2.logger.Enabled())
}

func TestResurfaceWriteData(t *testing.T) {
	const MockHost = "test0"

	pmp, _ := SetUp(t, "", make([]string, 0), "include debug")

	recs := []interface{}{
		analytics.AnalyticsRecord{
			Host:         MockHost,
			Method:       "GET",
			ResponseCode: 200,
			RawRequest:   rawReq,
			RawResponse:  rawResp,
			TimeStamp:    time.Now(),
		},
		analytics.AnalyticsRecord{
			Host:         MockHost,
			Method:       "POST",
			ResponseCode: 200,
			RawRequest:   rawReq,
			RawResponse:  rawResp,
			TimeStamp:    time.Now(),
		},
		analytics.AnalyticsRecord{
			Host:         MockHost,
			Method:       "GET",
			ResponseCode: 500,
			RawRequest:   rawReq,
			RawResponse:  rawResp,
			TimeStamp:    time.Now(),
		},
		analytics.AnalyticsRecord{
			Host:         MockHost,
			Method:       "Not valid",
			ResponseCode: 1200,
			RawRequest:   rawReq,
			RawResponse:  rawResp,
			TimeStamp:    time.Now(),
		},
		analytics.AnalyticsRecord{
			Host:        MockHost,
			Method:      "GET",
			RawRequest:  rawReq,
			RawResponse: rawResp,
			TimeStamp:   time.Now(),
		},
	}

	err := pmp.WriteData(context.TODO(), recs)
	assert.Nil(t, err, pmp.GetName()+"couldn't write records")

	queue := pmp.logger.Queue()
	assert.Equal(t, len(recs), len(queue))

	for i, message := range queue {
		assert.Contains(t, message, "[\"request_url\",\"http://"+MockHost)
		assert.NotContains(t, message, "[\"request_url\",\"http://localhost:8080/get\"]")
		if i%2 == 0 {
			assert.Contains(t, message, "[\"request_method\",\"GET\"]")
		}
		assert.Contains(t, message, "[\"request_header:user-agent\",\"Mozilla/5.0 (X11; Ubuntu")
		assert.Contains(t, message, "[\"request_header:accept\",\"text/html,application/xhtml+xml,application/xml;q=0.9,")
		assert.Contains(t, message, "[\"request_header:accept-encoding\",\"gzip, deflate\"]")
		assert.Contains(t, message, "[\"request_header:accept-language\",\"en-US,en;q=0.5\"]")
		assert.Contains(t, message, "[\"request_header:sec-fetch-dest\",\"document\"]")
		assert.Contains(t, message, "[\"request_header:sec-fetch-mode\",\"navigate\"]")
		assert.Contains(t, message, "[\"request_header:sec-fetch-site\",\"none\"]")

		if i&2 != 2 {
			assert.Contains(t, message, "response_code\",\"200")
		}
		assert.Contains(t, message, "[\"response_header:content-length\",\"429\"]")
		assert.Contains(t, message, "[\"response_header:access-control-allow-credentials\",\"true\"]")
		assert.Contains(t, message, "[\"response_header:access-control-allow-origin\",\"*\"]")
		assert.Contains(t, message, "[\"response_header:content-type\",\"application/json\"]")
		assert.Contains(t, message, "[\"response_header:content-type\",\"application/json\"]")
		assert.Contains(t, message, "[\"response_body")
		assert.Contains(t, message, "Yours Truly")
	}

	err = pmp.WriteData(context.TODO(), []interface{}{
		analytics.AnalyticsRecord{
			Host:         MockHost,
			Method:       "PUT",
			ResponseCode: 404,
			RawRequest:   "bm90IHZhbGlkCg==",
			RawResponse:  "bm90IHZhbGlkCg==",
			TimeStamp:    time.Now(),
		},
	})
	assert.Nil(t, err, pmp.GetName()+"couldn't write records")

	queue = pmp.logger.Queue()
	assert.Equal(t, len(recs)+1, len(queue))

	message := queue[len(queue)-1]
	assert.Contains(t, message, "[\"request_url\",\"http://"+MockHost)
	assert.NotContains(t, message, "[\"request_url\",\"http://localhost:8080/get\"]")
	assert.Contains(t, message, "[\"request_method\",\"PUT\"]")
	assert.NotContains(t, message, "[\"request_method\",\"GET\"]")
	assert.NotContains(t, message, "request_header")

	assert.Contains(t, message, "response_code\",\"404")
	assert.NotContains(t, message, "response_code\",\"200")
	assert.NotContains(t, message, "response_header")
	assert.NotContains(t, message, "response_body")
	assert.NotContains(t, message, "Yours Truly")
}

func TestResurfaceWriteCustomFields(t *testing.T) {
	pmp, _ := SetUp(t, "", make([]string, 0), "include debug")

	recs := []interface{}{
		analytics.AnalyticsRecord{
			APIID:        "my-api-123",
			OrgID:        "my-org-abc",
			Host:         "testone",
			Method:       "GET",
			ResponseCode: 200,
			RawRequest:   rawReq,
			RawResponse:  rawResp,
			TimeStamp:    time.Now(),
		},
		analytics.AnalyticsRecord{
			APIID:        " hello  ",
			OrgID:        "  world",
			Host:         "testtwo",
			Method:       "POST",
			ResponseCode: 200,
			RawRequest:   rawReq,
			RawResponse:  rawResp,
			TimeStamp:    time.Now(),
		},
		analytics.AnalyticsRecord{
			APIID:        "727dad853a8a45f64ab981154d1ffdad",
			APIKey:       "an-uhashed-key",
			APIName:      "Foo API",
			APIVersion:   "0.1.0-b",
			OauthID:      "my-oauth-client-id",
			OrgID:        "my-org-abc",
			Host:         "test-3",
			Method:       "GET",
			ResponseCode: 500,
			RawRequest:   rawReq,
			RawResponse:  rawResp,
			TimeStamp:    time.Now(),
		},
		analytics.AnalyticsRecord{
			APIID:       "",
			OrgID:       "",
			Host:        "test-four",
			Method:      "GET",
			RawRequest:  rawReq,
			RawResponse: rawResp,
			TimeStamp:   time.Now(),
		},
	}

	err := pmp.WriteData(context.TODO(), recs)
	assert.Nil(t, err, pmp.GetName()+"couldn't write records")

	queue := pmp.logger.Queue()
	assert.Equal(t, len(recs), len(queue))

	for i, message := range queue {
		if i < 3 {
			assert.Contains(t, message, strings.ToLower("custom_field:tyk-API-ID\",\""+recs[i].(analytics.AnalyticsRecord).APIID))
			assert.Contains(t, message, strings.ToLower("custom_field:tyk-Org-ID\",\""+recs[i].(analytics.AnalyticsRecord).OrgID))
			if i == 2 {
				assert.Contains(t, message, strings.ToLower("custom_field:tyk-API-Key\",\""+recs[i].(analytics.AnalyticsRecord).APIKey))
				assert.Contains(t, message, strings.ToLower("custom_field:tyk-API-Name\",\""+recs[i].(analytics.AnalyticsRecord).APIName))
				assert.Contains(t, message, strings.ToLower("custom_field:tyk-API-Version\",\""+recs[i].(analytics.AnalyticsRecord).APIVersion))
				assert.Contains(t, message, strings.ToLower("custom_field:tyk-Oauth-ID\",\""+recs[i].(analytics.AnalyticsRecord).OauthID))
			}
		} else {
			assert.NotContains(t, message, "custom_field:tyk")
		}
	}
}

func TestResurfaceWriteChunkedResponse(t *testing.T) {
	pmp, _ := SetUp(t, "", make([]string, 0), "include debug")

	recs := []interface{}{
		analytics.AnalyticsRecord{
			Host:        "test-three",
			Method:      "GET",
			RawRequest:  rawReq,
			RawResponse: rawRespOneChunk,
			TimeStamp:   time.Now(),
		},
		analytics.AnalyticsRecord{
			APIID:       "api-id-x",
			OrgID:       "api-org-y",
			Host:        "test-4",
			Method:      "GET",
			RawRequest:  rawReq,
			RawResponse: rawRespChunks,
			TimeStamp:   time.Now(),
		},
		analytics.AnalyticsRecord{
			APIID:       "",
			OrgID:       "",
			Host:        "test.five",
			Method:      "GET",
			RawRequest:  rawReq,
			RawResponse: rawRespChunksTrailer,
			TimeStamp:   time.Now(),
		},
	}

	err := pmp.WriteData(context.TODO(), recs)
	if err != nil {
		t.Fatal(pmp.GetName()+"couldn't write records with err:", err)
	}

	queue := pmp.logger.Queue()
	assert.Equal(t, len(recs), len(queue))

	for i, message := range queue {
		assert.Contains(t, message, "request_url\",\"http://test")
		assert.NotContains(t, message, "response_header:content-length")
		assert.Contains(t, message, "[\"response_header:transfer-encoding\",\"chunked\"]")
		if i != 4 {
			assert.Regexp(t, `\[\"response_body\",\".*\\r\\n0(?:\\r\\n)+\"\]`, message)
		} else {
			assert.Regexp(t, `\[\"response_body\",\".*\\r\\n0\\r\\nExpires:.*\"\]`, message)
		}
	}
}

func TestResurfaceSkipWrite(t *testing.T) {
	pmp, _ := SetUp(t, "", make([]string, 0), "include debug")

	recs := []interface{}{
		analytics.AnalyticsRecord{
			APIID:       "an-api-id",
			OrgID:       "an-api-org",
			Host:        "test6",
			Method:      "POST",
			RawRequest:  "",
			RawResponse: "",
			TimeStamp:   time.Now(),
		},
	}

	err := pmp.WriteData(context.TODO(), recs)
	assert.Nil(t, err, pmp.GetName()+"couldn't write records")

	queue := pmp.logger.Queue()
	assert.Equal(t, 0, len(queue))
	assert.Empty(t, queue)
}
