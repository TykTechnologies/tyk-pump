package pumps

import (
	"errors"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestInitVec(t *testing.T) {
	tcs := []struct {
		testName     string
		customMetric PrometheusMetric
		expectedErr  error
		isEnabled    bool
	}{
		{
			testName: "Counter metric",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"response_code", "api_id"},
			},
			expectedErr: nil,
			isEnabled:   true,
		},
		{
			testName: "Histogram metric",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"response_code", "api_id"},
			},
			expectedErr: nil,
			isEnabled:   true,
		},
		{
			testName: "RandomType metric",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: "RandomType",
				Labels:     []string{"response_code", "api_id"},
			},
			expectedErr: errors.New("invalid metric type:RandomType"),
			isEnabled:   false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			err := tc.customMetric.InitVec()
			assert.Equal(t, tc.expectedErr, err)
			if err != nil {
				return
			}

			assert.Equal(t, tc.isEnabled, tc.isEnabled)

			if tc.customMetric.MetricType == COUNTER_TYPE {
				assert.NotNil(t, tc.customMetric.counterVec)
				assert.Equal(t, tc.isEnabled, prometheus.Unregister(tc.customMetric.counterVec))

			} else if tc.customMetric.MetricType == HISTOGRAM_TYPE {
				assert.NotNil(t, tc.customMetric.histogramVec)
				assert.Equal(t, tc.isEnabled, prometheus.Unregister(tc.customMetric.histogramVec))

			}

		})
	}
}

func TestGetLabelsValues(t *testing.T) {
	tcs := []struct {
		testName       string
		customMetric   PrometheusMetric
		record         analytics.AnalyticsRecord
		expectedLabels []string
	}{
		{
			testName: "tree valid labels",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"response_code", "api_id", "api_key"},
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "apikey",
			},
			expectedLabels: []string{"200", "api_1", "apikey"},
		},
		{
			testName: "two valid labels - one wrong",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"host", "method", "randomLabel"},
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				Host:         "testHost",
				Method:       "testMethod",
				ResponseCode: 200,
				APIKey:       "apikey",
			},
			expectedLabels: []string{"testHost", "testMethod"},
		},
		{
			testName: "situational labels names ",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"code", "api", "key"},
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				Host:         "testHost",
				Method:       "testMethod",
				ResponseCode: 200,
				APIKey:       "apikey",
			},
			expectedLabels: []string{"200", "api_1", "apikey"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			gotLabels := tc.customMetric.GetLabelsValues(tc.record)
			assert.EqualValues(t, tc.expectedLabels, gotLabels)
		})
	}
}

func TestPrometheusCounterMetric(t *testing.T) {

	tcs := []struct {
		testName string

		metric                *PrometheusMetric
		analyticsRecords      []analytics.AnalyticsRecord
		expectedMetricsAmount int
		expectedMetrics       map[string]uint64
	}{
		{
			testName: "HTTP status codes per API",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status",
				Help:       "HTTP status codes per API",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"code", "api"},
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", ResponseCode: 500},
				{APIID: "api_1", ResponseCode: 500},
				{APIID: "api_1", ResponseCode: 200},
				{APIID: "api_2", ResponseCode: 404},
			},
			expectedMetricsAmount: 3,
			expectedMetrics: map[string]uint64{
				"500--api_1": 2,
				"200--api_1": 1,
				"404--api_2": 1,
			},
		},
		{
			testName: "HTTP status codes per API path and method",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_path",
				Help:       "HTTP status codes per API path and method",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"code", "api", "path", "method"},
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", ResponseCode: 500, Path: "test", Method: "GET"},
				{APIID: "api_1", ResponseCode: 500, Path: "test2", Method: "GET"},
				{APIID: "api_1", ResponseCode: 500, Path: "test", Method: "GET"},
				{APIID: "api_1", ResponseCode: 500, Path: "test", Method: "POST"},
				{APIID: "api_1", ResponseCode: 200, Path: "test2", Method: "GET"},
				{APIID: "api_2", ResponseCode: 200, Path: "test", Method: "GET"},
			},
			expectedMetricsAmount: 5,
			expectedMetrics: map[string]uint64{
				"500--api_1--test--GET":  2,
				"500--api_1--test--POST": 1,
				"500--api_1--test2--GET": 1,
				"200--api_1--test2--GET": 1,
				"200--api_2--test--GET":  1,
			},
		},
		{
			testName: "HTTP status codes per API key",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_key",
				Help:       "HTTP status codes per API key",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"code", "key"},
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", ResponseCode: 500, APIKey: "key1"},
				{APIID: "api_1", ResponseCode: 500, APIKey: "key1"},
				{APIID: "api_1", ResponseCode: 500, APIKey: "key2"},
				{APIID: "api_1", ResponseCode: 200, APIKey: "key1"},
				{APIID: "api_2", ResponseCode: 200, APIKey: "key1"},
			},
			expectedMetricsAmount: 3,
			expectedMetrics: map[string]uint64{
				"500--key1": 2,
				"200--key1": 2,
				"500--key2": 1,
			},
		},
		{
			testName: "HTTP status codes per oAuth client id",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_oauth_client",
				Help:       "HTTP status codes per oAuth client id",
				MetricType: COUNTER_TYPE,
				Labels:     []string{"code", "client_id"},
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", ResponseCode: 500, OauthID: "oauth1"},
				{APIID: "api_1", ResponseCode: 500, OauthID: "oauth1"},
				{APIID: "api_1", ResponseCode: 500, OauthID: "oauth2"},
				{APIID: "api_1", ResponseCode: 200, OauthID: "oauth1"},
				{APIID: "api_2", ResponseCode: 200, OauthID: "oauth1"},
			},
			expectedMetricsAmount: 3,
			expectedMetrics: map[string]uint64{
				"500--oauth1": 2,
				"200--oauth1": 2,
				"500--oauth2": 1,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			err := tc.metric.InitVec()
			assert.Nil(t, err)
			defer prometheus.Unregister(tc.metric.counterVec)
			for _, record := range tc.analyticsRecords {
				labelValues := tc.metric.GetLabelsValues(record)
				assert.Equal(t, len(tc.metric.Labels), len(labelValues))

				errInc := tc.metric.Inc(labelValues...)
				assert.Nil(t, errInc)
			}

			assert.Equal(t, len(tc.metric.counterMap), tc.expectedMetricsAmount)

			assert.EqualValues(t, tc.expectedMetrics, tc.metric.counterMap)

			errExpose := tc.metric.Expose()
			assert.Nil(t, errExpose)
			assert.Equal(t, len(tc.metric.counterMap), 0)
		})
	}
}

func TestPrometheusHistogramMetric(t *testing.T) {

	tcs := []struct {
		testName string

		metric                *PrometheusMetric
		analyticsRecords      []analytics.AnalyticsRecord
		expectedMetricsAmount int
		expectedMetrics       map[string]histogramCounter
		expectedAverages      map[string]float64
	}{
		{
			testName: "Total Latency per API - aggregated observations true",
			metric: &PrometheusMetric{
				Name:                   "tyk_latency_per_api",
				Help:                   "Latency added by Tyk, Total Latency, and upstream latency per API",
				MetricType:             HISTOGRAM_TYPE,
				Buckets:                buckets,
				Labels:                 []string{"type", "api"},
				aggregatedObservations: true,
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", RequestTime: 60},
				{APIID: "api_1", RequestTime: 140},
				{APIID: "api_1", RequestTime: 100},
				{APIID: "api_2", RequestTime: 323},
			},
			expectedMetricsAmount: 2,
			expectedMetrics: map[string]histogramCounter{
				"total--api_1": {hits: 3, totalRequestTime: 300},
				"total--api_2": {hits: 1, totalRequestTime: 323},
			},
			expectedAverages: map[string]float64{
				"total--api_1": 100,
				"total--api_2": 323,
			},
		},
		{
			testName: " Total Latency per API - aggregated observations false",
			metric: &PrometheusMetric{
				Name:                   "tyk_latency_per_api_2",
				Help:                   "Latency added by Tyk, Total Latency, and upstream latency per API",
				MetricType:             HISTOGRAM_TYPE,
				Buckets:                buckets,
				Labels:                 []string{"type", "api"},
				aggregatedObservations: false,
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", RequestTime: 60},
				{APIID: "api_1", RequestTime: 140},
				{APIID: "api_1", RequestTime: 100},
				{APIID: "api_2", RequestTime: 323},
			},
			expectedMetricsAmount: 0,
			expectedMetrics:       map[string]histogramCounter{},
			expectedAverages:      map[string]float64{},
		},
		{
			testName: " Total Latency per API_ID, Method and Path - aggregated observations true",
			metric: &PrometheusMetric{
				Name:                   "tyk_latency_per_api_method_path",
				Help:                   "Latency added by Tyk, Total Latency, and upstream latency per API_ID, Method and Path",
				MetricType:             HISTOGRAM_TYPE,
				Buckets:                buckets,
				Labels:                 []string{"type", "api_id", "method", "path"},
				aggregatedObservations: true,
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", Method: "GET", Path: "test", RequestTime: 60},
				{APIID: "api_1", Method: "GET", Path: "test", RequestTime: 140},
				{APIID: "api_1", Method: "POST", Path: "test", RequestTime: 200},
				{APIID: "api_2", Method: "GET", Path: "ping", RequestTime: 10},
				{APIID: "api_2", Method: "GET", Path: "ping", RequestTime: 20},
				{APIID: "api_2", Method: "GET", Path: "health", RequestTime: 400},
			},
			expectedMetricsAmount: 4,
			expectedMetrics: map[string]histogramCounter{
				"total--api_1--GET--test":   {hits: 2, totalRequestTime: 200},
				"total--api_1--POST--test":  {hits: 1, totalRequestTime: 200},
				"total--api_2--GET--ping":   {hits: 2, totalRequestTime: 30},
				"total--api_2--GET--health": {hits: 1, totalRequestTime: 400},
			},
			expectedAverages: map[string]float64{
				"total--api_1--GET--test":   100,
				"total--api_1--POST--test":  200,
				"total--api_2--GET--ping":   15,
				"total--api_2--GET--health": 400,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			err := tc.metric.InitVec()
			assert.Nil(t, err)
			defer prometheus.Unregister(tc.metric.histogramVec)

			for _, record := range tc.analyticsRecords {
				labelValues := tc.metric.GetLabelsValues(record)

				assert.Equal(t, len(tc.metric.Labels)-1, len(labelValues))
				errObserve := tc.metric.Observe(record.RequestTime, labelValues...)
				assert.Nil(t, errObserve)
			}

			assert.Equal(t, len(tc.metric.histogramMap), tc.expectedMetricsAmount)

			assert.EqualValues(t, tc.expectedMetrics, tc.metric.histogramMap)

			for keyName, histogramCounter := range tc.metric.histogramMap {
				if expectedValue, ok := tc.expectedAverages[keyName]; ok {
					assert.Equal(t, expectedValue, histogramCounter.getAverageRequestTime())
				} else {
					t.Error("keyName " + keyName + " doesnt exist in expectedAverages map")
				}
			}

			errExpose := tc.metric.Expose()
			assert.Nil(t, errExpose)
			assert.Equal(t, len(tc.metric.histogramMap), 0)
		})
	}
}

func TestPromtheusCreateBasicMetrics(t *testing.T) {
	newPump := &PrometheusPump{}
	err := newPump.Init(PrometheusConf{Addr: "localhost:8080"})
	assert.Nil(t, err)
	assert.Len(t, newPump.allMetrics, 5)

	actualMetricsNames := []string{}
	actualMetricTypeCounter := make(map[string]int)
	for _, metric := range newPump.allMetrics {
		actualMetricsNames = append(actualMetricsNames, metric.Name)
		actualMetricTypeCounter[metric.MetricType] += 1
	}

	assert.EqualValues(t, actualMetricsNames, []string{"tyk_http_status", "tyk_http_status_per_path", "tyk_http_status_per_key", "tyk_http_status_per_oauth_client", "tyk_latency"})

	assert.Equal(t, 4, actualMetricTypeCounter[COUNTER_TYPE])
	assert.Equal(t, 1, actualMetricTypeCounter[HISTOGRAM_TYPE])

}
