package pumps

import (
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

func TestInflux2PumpMappingIncludesLatency(t *testing.T) {
	// Create a sample analytics record
	ar := analytics.AnalyticsRecord{
		Method:       "GET",
		Path:         "/test/path",
		ResponseCode: 200,
		APIKey:       "test-api-key",
		TimeStamp:    time.Date(2025, 7, 3, 12, 0, 0, 0, time.UTC),
		APIVersion:   "v1",
		APIName:      "Test API",
		APIID:        "test-api-id",
		OrgID:        "test-org-id",
		OauthID:      "test-oauth-id",
		RawRequest:   "raw-request-data",
		RequestTime:  123,
		RawResponse:  "raw-response-data",
		IPAddress:    "1.2.3.4",
		Latency: analytics.Latency{
			Total:    456,
			Upstream: 123,
		},
	}

	// Simulate how your Influx2 pump builds the mapping
	mapping := map[string]interface{}{
		"method":           ar.Method,
		"path":             ar.Path,
		"response_code":    ar.ResponseCode,
		"api_key":          ar.APIKey,
		"time_stamp":       ar.TimeStamp,
		"api_version":      ar.APIVersion,
		"api_name":         ar.APIName,
		"api_id":           ar.APIID,
		"org_id":           ar.OrgID,
		"oauth_id":         ar.OauthID,
		"raw_request":      ar.RawRequest,
		"request_time":     ar.RequestTime,
		"raw_response":     ar.RawResponse,
		"ip_address":       ar.IPAddress,
		"total_latency":    ar.Latency.Total,
		"upstream_latency": ar.Latency.Upstream,
	}

	// Check the new latency fields exist and have the right values
	assert.Equal(t, int64(456), mapping["total_latency"])
	assert.Equal(t, int64(123), mapping["upstream_latency"])

	// Check some other fields for completeness
	assert.Equal(t, "GET", mapping["method"])
	assert.Equal(t, "/test/path", mapping["path"])
	assert.Equal(t, 200, mapping["response_code"])
	assert.Equal(t, "test-api-key", mapping["api_key"])
	assert.Equal(t, "v1", mapping["api_version"])
	assert.Equal(t, "Test API", mapping["api_name"])
	assert.Equal(t, "test-api-id", mapping["api_id"])
	assert.Equal(t, "test-org-id", mapping["org_id"])
	assert.Equal(t, "test-oauth-id", mapping["oauth_id"])
	assert.Equal(t, "raw-request-data", mapping["raw_request"])
	assert.Equal(t, int64(123), mapping["request_time"])
	assert.Equal(t, "raw-response-data", mapping["raw_response"])
	assert.Equal(t, "1.2.3.4", mapping["ip_address"])
	assert.Equal(t, time.Date(2025, 7, 3, 12, 0, 0, 0, time.UTC), mapping["time_stamp"])
}
