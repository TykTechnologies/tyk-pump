package pumps

import (
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

func TestGetMappings(t *testing.T) {
	ts := time.Now()
	unixTime := time.Unix(ts.Unix(), 0)

	// Replace : to -
	sanitizedTime := strings.Replace(unixTime.String(), ":", "-", -1)
	tcs := []struct {
		testName         string
		separatedMethood bool
		record           analytics.AnalyticsRecord
		expectedMappings map[string]interface{}
	}{
		{
			testName:         "disabled separated methods",
			separatedMethood: false,
			record: analytics.AnalyticsRecord{
				Path:         "/test",
				Method:       "GET",
				APIID:        "test",
				ResponseCode: 200,
				TimeStamp:    ts,
			},
			expectedMappings: map[string]interface{}{
				"path":          "GET/test",
				"response_code": 200,
				"api_key":       "",
				"time_stamp":    sanitizedTime,
				"api_version":   "",
				"api_name":      "",
				"api_id":        "test",
				"org_id":        "",
				"oauth_id":      "",
				"raw_request":   "",
				"request_time":  int64(0),
				"raw_response":  "",
				"ip_address":    "",
			},
		},
		{
			testName:         "enabled separated methods",
			separatedMethood: true,
			record: analytics.AnalyticsRecord{
				Path:         "/test",
				Method:       "GET",
				APIID:        "test",
				ResponseCode: 200,
				TimeStamp:    ts,
			},
			expectedMappings: map[string]interface{}{
				"path":          "/test",
				"method":        "GET",
				"response_code": 200,
				"api_key":       "",
				"time_stamp":    sanitizedTime,
				"api_version":   "",
				"api_name":      "",
				"api_id":        "test",
				"org_id":        "",
				"oauth_id":      "",
				"raw_request":   "",
				"request_time":  int64(0),
				"raw_response":  "",
				"ip_address":    "",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			conf := StatsdConf{SeparatedMethod: tc.separatedMethood}
			pmp := StatsdPump{dbConf: &conf}

			mappings := pmp.getMappings(tc.record)
			assert.Equal(t, tc.expectedMappings, mappings)
		})
	}
}
