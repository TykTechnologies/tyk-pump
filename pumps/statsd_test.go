package pumps

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/quipo/statsd"
	"github.com/sirupsen/logrus"
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
				"path":             "GET/test",
				"response_code":    200,
				"api_key":          "",
				"time_stamp":       sanitizedTime,
				"api_version":      "",
				"api_name":         "",
				"api_id":           "test",
				"org_id":           "",
				"oauth_id":         "",
				"raw_request":      "",
				"request_time":     int64(0),
				"raw_response":     "",
				"ip_address":       "",
				"latency_total":    int64(0),
				"latency_upstream": int64(0),
				"latency_gateway":  int64(0),
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
				"path":             "/test",
				"method":           "GET",
				"response_code":    200,
				"api_key":          "",
				"time_stamp":       sanitizedTime,
				"api_version":      "",
				"api_name":         "",
				"api_id":           "test",
				"org_id":           "",
				"oauth_id":         "",
				"raw_request":      "",
				"request_time":     int64(0),
				"raw_response":     "",
				"ip_address":       "",
				"latency_total":    int64(0),
				"latency_upstream": int64(0),
				"latency_gateway":  int64(0),
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

func TestStatsdPump_isTimingField(t *testing.T) {
	pmp := &StatsdPump{}

	tcs := []struct {
		testName     string
		field        string
		expectedBool bool
	}{
		{
			testName:     "request_time should be timing field",
			field:        "request_time",
			expectedBool: true,
		},
		{
			testName:     "latency_total should be timing field",
			field:        "latency_total",
			expectedBool: true,
		},
		{
			testName:     "latency_upstream should be timing field",
			field:        "latency_upstream",
			expectedBool: true,
		},
		{
			testName:     "latency_gateway should be timing field",
			field:        "latency_gateway",
			expectedBool: true,
		},
		{
			testName:     "response_code should not be timing field",
			field:        "response_code",
			expectedBool: false,
		},
		{
			testName:     "api_id should not be timing field",
			field:        "api_id",
			expectedBool: false,
		},
		{
			testName:     "empty string should not be timing field",
			field:        "",
			expectedBool: false,
		},
		{
			testName:     "random field should not be timing field",
			field:        "random_field",
			expectedBool: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			result := pmp.isTimingField(tc.field)
			assert.Equal(t, tc.expectedBool, result)
		})
	}
}

func TestStatsdPump_sendTimingMetric(t *testing.T) {
	// Create a real StatsD client with a dummy address
	client := statsd.NewStatsdClient("127.0.0.1:8125", "")

	pmp := &StatsdPump{}
	log := logrus.New()
	log.Out = io.Discard
	pmp.log = logrus.NewEntry(log)

	// Test successful timing metric
	field := "request_time"
	metricTags := "api123"
	value := int64(150)

	// This should not panic - the actual StatsD client will try to connect
	// but since we're not actually running a StatsD server, it will fail silently
	pmp.sendTimingMetric(client, field, metricTags, value)

	// Close the client
	client.Close()
}

func TestStatsdPump_sendTimingMetric_ErrorHandling(t *testing.T) {
	// Create a StatsD client with an invalid address to trigger connection error
	client := statsd.NewStatsdClient("invalid:address", "")

	pmp := &StatsdPump{}
	log := logrus.New()
	log.Out = io.Discard
	pmp.log = logrus.NewEntry(log)

	// Test error handling
	field := "request_time"
	metricTags := "api123"
	value := int64(150)

	// This should not panic and should log the error
	pmp.sendTimingMetric(client, field, metricTags, value)

	// Close the client
	client.Close()
}
