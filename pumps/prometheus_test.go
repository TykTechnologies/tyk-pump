package pumps

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusInitVec(t *testing.T) {
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
				MetricType: counterType,
				Labels:     []string{"response_code", "api_id"},
			},
			expectedErr: nil,
			isEnabled:   true,
		},
		{
			testName: "Histogram metric",
			customMetric: PrometheusMetric{
				Name:       "testHistogramMetric",
				MetricType: histogramType,
				Labels:     []string{"type", "api_id"},
			},
			expectedErr: nil,
			isEnabled:   true,
		},
		{
			testName: "Histogram metric without type label set",
			customMetric: PrometheusMetric{
				Name:       "testHistogramMetricWithoutTypeSet",
				MetricType: histogramType,
				Labels:     []string{"api_id"},
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

			if tc.customMetric.MetricType == counterType {
				assert.NotNil(t, tc.customMetric.counterVec)
				assert.Equal(t, tc.isEnabled, prometheus.Unregister(tc.customMetric.counterVec))

			} else if tc.customMetric.MetricType == histogramType {
				assert.NotNil(t, tc.customMetric.histogramVec)
				assert.Equal(t, tc.isEnabled, prometheus.Unregister(tc.customMetric.histogramVec))
				assert.Equal(t, tc.customMetric.Labels[0], "type")
			}
		})
	}
}

func TestPrometheusInitCustomMetrics(t *testing.T) {
	tcs := []struct {
		testName              string
		metrics               []PrometheusMetric
		expectedAllMetricsLen int
	}{
		{
			testName:              "no custom metrics",
			metrics:               []PrometheusMetric{},
			expectedAllMetricsLen: 0,
		},
		{
			testName: "single custom metrics",
			metrics: []PrometheusMetric{
				{
					Name:       "test",
					MetricType: counterType,
					Labels:     []string{"api_name"},
				},
			},
			expectedAllMetricsLen: 1,
		},
		{
			testName: "multiple custom metrics",
			metrics: []PrometheusMetric{
				{
					Name:       "test",
					MetricType: counterType,
					Labels:     []string{"api_name"},
				},
				{
					Name:       "other_test",
					MetricType: counterType,
					Labels:     []string{"api_name", "api_key"},
				},
			},
			expectedAllMetricsLen: 2,
		},
		{
			testName: "multiple custom metrics with histogram",
			metrics: []PrometheusMetric{
				{
					Name:       "test",
					MetricType: counterType,
					Labels:     []string{"api_name"},
				},
				{
					Name:       "other_test",
					MetricType: counterType,
					Labels:     []string{"api_name", "api_key"},
				},
				{
					Name:       "histogram_test",
					MetricType: histogramType,
					Labels:     []string{"api_name", "api_key"},
				},
			},
			expectedAllMetricsLen: 3,
		},
		{
			testName: "one with error",
			metrics: []PrometheusMetric{
				{
					Name:       "test",
					MetricType: "test_type",
					Labels:     []string{"api_name"},
				},
				{
					Name:       "other_test",
					MetricType: counterType,
					Labels:     []string{"api_name", "api_key"},
				},
			},
			expectedAllMetricsLen: 1,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			p := PrometheusPump{}
			p.conf = &PrometheusConf{}
			p.log = log.WithField("prefix", prometheusPrefix)
			p.conf.CustomMetrics = tc.metrics

			p.InitCustomMetrics()
			// this function do the unregistering for the metrics in the prometheus lib.
			defer func() {
				for i := range tc.metrics {
					if tc.metrics[i].MetricType == counterType {
						prometheus.Unregister(tc.metrics[i].counterVec)
					} else if tc.metrics[i].MetricType == histogramType {
						prometheus.Unregister(tc.metrics[i].histogramVec)
					}
				}
			}()
			assert.Equal(t, tc.expectedAllMetricsLen, len(p.allMetrics))
		})
	}
}

func TestInitCustomMetricsEnv(t *testing.T) {
	tcs := []struct {
		testName        string
		envKey          string
		envValue        string
		envPrefix       string
		expectedMetrics CustomMetrics
	}{
		{
			testName:  "valid custom - coutner metric",
			envPrefix: "TYK_PMP_PUMPS_PROMETHEUS_META",
			envKey:    "TYK_PMP_PUMPS_PROMETHEUS_META_CUSTOMMETRICS",
			envValue:  `[{"name":"tyk_http_requests_total","help":"Total of API requests","metric_type":"counter","labels":["response_code","api_name"]}]`,
			expectedMetrics: CustomMetrics{
				PrometheusMetric{
					Name:       "tyk_http_requests_total",
					Help:       "Total of API requests",
					MetricType: counterType,
					Labels:     []string{"response_code", "api_name"},
				},
			},
		},
		{
			testName:  "valid customs - counter metric",
			envPrefix: "TYK_PMP_PUMPS_PROMETHEUS_META",
			envKey:    "TYK_PMP_PUMPS_PROMETHEUS_META_CUSTOMMETRICS",
			envValue:  `[{"name":"tyk_http_requests_total","help":"Total of API requests","metric_type":"counter","labels":["response_code","api_name"]},{"name":"tyk_http_requests_total_two","help":"Total Two of API requests","metric_type":"counter","labels":["response_code","api_name"]}]`,
			expectedMetrics: CustomMetrics{
				PrometheusMetric{
					Name:       "tyk_http_requests_total",
					Help:       "Total of API requests",
					MetricType: counterType,
					Labels:     []string{"response_code", "api_name"},
				},
				PrometheusMetric{
					Name:       "tyk_http_requests_total_two",
					Help:       "Total Two of API requests",
					MetricType: counterType,
					Labels:     []string{"response_code", "api_name"},
				},
			},
		},
		{
			testName:  "valid customs - histogram metric",
			envPrefix: "TYK_PMP_PUMPS_PROMETHEUS_META",
			envKey:    "TYK_PMP_PUMPS_PROMETHEUS_META_CUSTOMMETRICS",
			envValue:  `[{"name":"tyk_http_requests_total","help":"Total of API requests","metric_type":"histogram","buckets":[100,200],"labels":["response_code","api_name"]}]`,
			expectedMetrics: CustomMetrics{
				PrometheusMetric{
					Name:       "tyk_http_requests_total",
					Help:       "Total of API requests",
					MetricType: histogramType,
					Buckets:    []float64{100, 200},
					Labels:     []string{"response_code", "api_name"},
				},
			},
		},
		{
			testName:        "invalid custom metric format",
			envPrefix:       "TYK_PMP_PUMPS_PROMETHEUS_META",
			envKey:          "TYK_PMP_PUMPS_PROMETHEUS_META_CUSTOMMETRICS",
			envValue:        `["name":"tyk_http_requests_total","help":"Total of API requests","metric_type":"histogram","buckets":[100,200],"labels":["response_code","api_name"]]`,
			expectedMetrics: CustomMetrics(nil),
		},
		{
			testName:        "invalid custom metric input",
			envPrefix:       "TYK_PMP_PUMPS_PROMETHEUS_META",
			envKey:          "TYK_PMP_PUMPS_PROMETHEUS_META_CUSTOMMETRICS",
			envValue:        `invalid-input`,
			expectedMetrics: CustomMetrics(nil),
		},
		{
			testName:        "empty custom metric input",
			envPrefix:       "TYK_PMP_PUMPS_PROMETHEUS_META",
			envKey:          "TYK_PMP_PUMPS_PROMETHEUS_META_CUSTOMMETRICS",
			envValue:        ``,
			expectedMetrics: CustomMetrics(nil),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			err := os.Setenv(tc.envKey, tc.envValue)
			assert.Nil(t, err)
			defer os.Unsetenv(tc.envKey)

			pmp := &PrometheusPump{}

			pmp.log = log.WithField("prefix", prometheusPrefix)
			pmp.conf = &PrometheusConf{}
			pmp.conf.EnvPrefix = tc.envPrefix
			processPumpEnvVars(pmp, pmp.log, pmp.conf, prometheusDefaultENV)

			assert.Equal(t, tc.expectedMetrics, pmp.conf.CustomMetrics)
		})
	}
}

func TestPrometheusGetLabelsValues(t *testing.T) {
	tcs := []struct {
		testName       string
		customMetric   PrometheusMetric
		record         analytics.AnalyticsRecord
		expectedLabels []string
	}{
		{
			testName: "empty API key with obfuscation enabled",
			customMetric: PrometheusMetric{
				Name:             "testCounterMetric",
				MetricType:       counterType,
				Labels:           []string{"code", "api", "key"},
				ObfuscateAPIKeys: true,
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "",
			},
			expectedLabels: []string{"200", "api_1", "--"},
		},
		{
			testName: "tree valid labels",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: counterType,
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
				MetricType: counterType,
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
				MetricType: counterType,
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
		{
			testName: "obfuscated API key - showing last 4 chars",
			customMetric: PrometheusMetric{
				Name:             "testCounterMetric",
				MetricType:       counterType,
				Labels:           []string{"code", "api", "key"},
				ObfuscateAPIKeys: true,
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "abcdefghijklmnop",
			},
			expectedLabels: []string{"200", "api_1", "****mnop"},
		},
		{
			testName: "obfuscated API key - short key (4 chars)",
			customMetric: PrometheusMetric{
				Name:             "testCounterMetric",
				MetricType:       counterType,
				Labels:           []string{"code", "api", "key"},
				ObfuscateAPIKeys: true,
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "abcd",
			},
			expectedLabels: []string{"200", "api_1", "--"},
		},
		{
			testName: "obfuscated API key - very short key (3 chars)",
			customMetric: PrometheusMetric{
				Name:             "testCounterMetric",
				MetricType:       counterType,
				Labels:           []string{"code", "api", "key"},
				ObfuscateAPIKeys: true,
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "abc",
			},
			expectedLabels: []string{"200", "api_1", "--"},
		},
		{
			testName: "obfuscation disabled",
			customMetric: PrometheusMetric{
				Name:             "testCounterMetric",
				MetricType:       counterType,
				Labels:           []string{"code", "api", "key"},
				ObfuscateAPIKeys: false,
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "abcdefghijklmnop",
			},
			expectedLabels: []string{"200", "api_1", "abcdefghijklmnop"},
		},
		{
			testName: "obfuscation disabled with short key",
			customMetric: PrometheusMetric{
				Name:             "testCounterMetric",
				MetricType:       counterType,
				Labels:           []string{"code", "api", "key"},
				ObfuscateAPIKeys: false,
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "abc",
			},
			expectedLabels: []string{"200", "api_1", "abc"},
		},
		{
			testName: "valid custom tag",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: counterType,
				Labels:     []string{"tag_custom"},
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "apikey",
				Tags:         []string{"custom-value"},
			},
			expectedLabels: []string{"value"},
		},
		{
			testName: "two valid tag labels - one without value",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: counterType,
				Labels:     []string{"tag_custom", "tag_noexists"},
			},
			record: analytics.AnalyticsRecord{
				APIID:        "api_1",
				ResponseCode: 200,
				APIKey:       "apikey",
				Tags:         []string{"custom-value"},
			},
			expectedLabels: []string{"value", ""},
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
		expectedMetrics       map[string]counterStruct
		trackAllPaths         bool
	}{
		{
			testName: "HTTP status codes per API",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status",
				Help:       "HTTP status codes per API",
				MetricType: counterType,
				Labels:     []string{"code", "api"},
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", ResponseCode: 500},
				{APIID: "api_1", ResponseCode: 500},
				{APIID: "api_1", ResponseCode: 200},
				{APIID: "api_2", ResponseCode: 404},
			},
			expectedMetricsAmount: 3,
			expectedMetrics: map[string]counterStruct{
				"500--api_1": {labelValues: []string{"500", "api_1"}, count: 2},
				"200--api_1": {labelValues: []string{"200", "api_1"}, count: 1},
				"404--api_2": {labelValues: []string{"404", "api_2"}, count: 1},
			},
		},
		{
			testName:      "HTTP status codes per API path and method - trackign all paths",
			trackAllPaths: true,
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_path",
				Help:       "HTTP status codes per API path and method",
				MetricType: counterType,
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
			expectedMetrics: map[string]counterStruct{
				"500--api_1--test--GET":  {labelValues: []string{"500", "api_1", "test", "GET"}, count: 2},
				"500--api_1--test--POST": {labelValues: []string{"500", "api_1", "test", "POST"}, count: 1},
				"500--api_1--test2--GET": {labelValues: []string{"500", "api_1", "test2", "GET"}, count: 1},
				"200--api_1--test2--GET": {labelValues: []string{"200", "api_1", "test2", "GET"}, count: 1},
				"200--api_2--test--GET":  {labelValues: []string{"200", "api_2", "test", "GET"}, count: 1},
			},
		},
		{
			testName:      "HTTP status codes per API path and method - tracking some paths",
			trackAllPaths: false,
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_path",
				Help:       "HTTP status codes per API path and method",
				MetricType: counterType,
				Labels:     []string{"code", "api", "path", "method"},
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", ResponseCode: 500, Path: "test", Method: "GET", TrackPath: true},
				{APIID: "api_1", ResponseCode: 500, Path: "test2", Method: "GET"},
				{APIID: "api_1", ResponseCode: 500, Path: "test", Method: "GET", TrackPath: true},
				{APIID: "api_1", ResponseCode: 500, Path: "test", Method: "POST", TrackPath: true},
				{APIID: "api_1", ResponseCode: 200, Path: "test2", Method: "GET"},
				{APIID: "api_2", ResponseCode: 200, Path: "test", Method: "GET"},
			},
			expectedMetricsAmount: 5,
			expectedMetrics: map[string]counterStruct{
				"500--api_1--test--GET":    {labelValues: []string{"500", "api_1", "test", "GET"}, count: 2},
				"500--api_1--test--POST":   {labelValues: []string{"500", "api_1", "test", "POST"}, count: 1},
				"500--api_1--unknown--GET": {labelValues: []string{"500", "api_1", "unknown", "GET"}, count: 1},
				"200--api_1--unknown--GET": {labelValues: []string{"200", "api_1", "unknown", "GET"}, count: 1},
				"200--api_2--unknown--GET": {labelValues: []string{"200", "api_2", "unknown", "GET"}, count: 1},
			},
		},
		{
			testName:      "HTTP status codes per API path and method - not tracking paths",
			trackAllPaths: false,
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_path",
				Help:       "HTTP status codes per API path and method",
				MetricType: counterType,
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
			expectedMetricsAmount: 4,
			expectedMetrics: map[string]counterStruct{
				"500--api_1--unknown--GET":  {labelValues: []string{"500", "api_1", "unknown", "GET"}, count: 3},
				"500--api_1--unknown--POST": {labelValues: []string{"500", "api_1", "unknown", "POST"}, count: 1},
				"200--api_1--unknown--GET":  {labelValues: []string{"200", "api_1", "unknown", "GET"}, count: 1},
				"200--api_2--unknown--GET":  {labelValues: []string{"200", "api_2", "unknown", "GET"}, count: 1},
			},
		},
		{
			testName: "HTTP status codes per API key",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_key",
				Help:       "HTTP status codes per API key",
				MetricType: counterType,
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
			expectedMetrics: map[string]counterStruct{
				"500--key1": {labelValues: []string{"500", "key1"}, count: 2},
				"200--key1": {labelValues: []string{"200", "key1"}, count: 2},
				"500--key2": {labelValues: []string{"500", "key2"}, count: 1},
			},
		},
		{
			testName: "HTTP status codes per oAuth client id",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_oauth_client",
				Help:       "HTTP status codes per oAuth client id",
				MetricType: counterType,
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
			expectedMetrics: map[string]counterStruct{
				"500--oauth1": {labelValues: []string{"500", "oauth1"}, count: 2},
				"200--oauth1": {labelValues: []string{"200", "oauth1"}, count: 2},
				"500--oauth2": {labelValues: []string{"500", "oauth2"}, count: 1},
			},
		},
		{
			testName: "HTTP status codes per api name and key alias",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_api_key_alias",
				Help:       "HTTP status codes per api name and key alias",
				MetricType: counterType,
				Labels:     []string{"code", "api", "alias"},
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api_1", ResponseCode: 500, Alias: "alias1"},
				{APIID: "api_1", ResponseCode: 500, Alias: "alias2"},
				{APIID: "api_1", ResponseCode: 200, Alias: "alias1"},
				{APIID: "api_1", ResponseCode: 500, Alias: "alias1"},
				{APIID: "api_2", ResponseCode: 500, Alias: "alias1"},
			},
			expectedMetricsAmount: 4,
			expectedMetrics: map[string]counterStruct{
				"500--api_1--alias1": {labelValues: []string{"500", "api_1", "alias1"}, count: 2},
				"500--api_1--alias2": {labelValues: []string{"500", "api_1", "alias2"}, count: 1},
				"200--api_1--alias1": {labelValues: []string{"200", "api_1", "alias1"}, count: 1},
				"500--api_2--alias1": {labelValues: []string{"500", "api_2", "alias1"}, count: 1},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			err := tc.metric.InitVec()
			assert.Nil(t, err)
			defer prometheus.Unregister(tc.metric.counterVec)
			for _, record := range tc.analyticsRecords {
				if !(tc.trackAllPaths || record.TrackPath) {
					record.Path = "unknown"
				}

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
				MetricType:             histogramType,
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
				"total--api_1": {
					hits:             3,
					totalRequestTime: 300,
					labelValues:      []string{"total", "api_1"},
				},
				"total--api_2": {
					hits:             1,
					totalRequestTime: 323,
					labelValues:      []string{"total", "api_2"},
				},
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
				MetricType:             histogramType,
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
				MetricType:             histogramType,
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
				{APIID: "api--3", Method: "GET", Path: "health", RequestTime: 300},
			},
			expectedMetricsAmount: 5,
			expectedMetrics: map[string]histogramCounter{
				"total--api_1--GET--test": {
					hits:             2,
					totalRequestTime: 200,
					labelValues:      []string{"total", "api_1", "GET", "test"},
				},
				"total--api_1--POST--test": {
					hits:             1,
					totalRequestTime: 200,
					labelValues:      []string{"total", "api_1", "POST", "test"},
				},
				"total--api_2--GET--ping": {
					hits:             2,
					totalRequestTime: 30,
					labelValues:      []string{"total", "api_2", "GET", "ping"},
				},
				"total--api_2--GET--health": {
					hits:             1,
					totalRequestTime: 400,
					labelValues:      []string{"total", "api_2", "GET", "health"},
				},
				"total--api--3--GET--health": {
					hits:             1,
					totalRequestTime: 300,
					labelValues:      []string{"total", "api--3", "GET", "health"},
				},
			},
			expectedAverages: map[string]float64{
				"total--api_1--GET--test":    100,
				"total--api_1--POST--test":   200,
				"total--api_2--GET--ping":    15,
				"total--api_2--GET--health":  400,
				"total--api--3--GET--health": 300,
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

func TestPrometheusCreateBasicMetrics(t *testing.T) {
	p := PrometheusPump{}
	newPump := p.New().(*PrometheusPump)

	assert.Len(t, newPump.allMetrics, 5)

	actualMetricsNames := []string{}
	actualMetricTypeCounter := make(map[string]int)
	for _, metric := range newPump.allMetrics {
		actualMetricsNames = append(actualMetricsNames, metric.Name)
		actualMetricTypeCounter[metric.MetricType] += 1
	}

	assert.EqualValues(t, actualMetricsNames, []string{
		"tyk_http_status",
		"tyk_http_status_per_path",
		"tyk_http_status_per_key",
		"tyk_http_status_per_oauth_client",
		metricTykLatency,
	})

	assert.Equal(t, 4, actualMetricTypeCounter[counterType])
	assert.Equal(t, 1, actualMetricTypeCounter[histogramType])
}

func TestPrometheusEnsureLabels(t *testing.T) {
	testCases := []struct {
		name                 string
		metricType           string
		labels               []string
		typeLabelShouldExist bool
	}{
		{
			name:                 "histogram type, type label should be added if not exist",
			labels:               []string{"response_code", "api_name", "method", "api_key", "alias", "path"},
			metricType:           histogramType,
			typeLabelShouldExist: true,
		},
		{
			name:                 "counter type, type label should not be added",
			labels:               []string{"response_code", "api_name", "method", "api_key", "alias", "path"},
			metricType:           counterType,
			typeLabelShouldExist: false,
		},
		{
			name:                 "histogram type, type label should not be repeated and in the 1st position",
			labels:               []string{"type", "response_code", "api_name", "method", "api_key", "alias", "path"},
			metricType:           histogramType,
			typeLabelShouldExist: true,
		},
		{
			name:                 "histogram type, type label should not be repeated (even if user repeated it), and always in the 1st position",
			labels:               []string{"response_code", "api_name", "type", "method", "api_key", "alias", "path", "type"},
			metricType:           histogramType,
			typeLabelShouldExist: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pm := PrometheusMetric{
				MetricType: tc.metricType,
				Labels:     tc.labels,
			}

			pm.ensureLabels()
			typeLabelFound := false
			numberOfTimesOfTypeLabel := 0

			for _, label := range pm.Labels {
				if label == "type" {
					typeLabelFound = true
					numberOfTimesOfTypeLabel++
				}
			}

			assert.Equal(t, tc.typeLabelShouldExist, typeLabelFound)

			// if should exist then it should be only one time
			if tc.typeLabelShouldExist {
				assert.Equal(t, 1, numberOfTimesOfTypeLabel)
				// label `type` should be in the 1st position always
				assert.Equal(t, pm.Labels[0], "type")
			}
		})
	}
}

func TestPrometheusDisablingMetrics(t *testing.T) {
	p := &PrometheusPump{}
	newPump := p.New().(*PrometheusPump)

	log := logrus.New()
	log.Out = io.Discard
	newPump.log = logrus.NewEntry(log)

	newPump.conf = &PrometheusConf{DisabledMetrics: []string{"tyk_http_status_per_path"}}

	newPump.initBaseMetrics()

	defer func() {
		for i := range newPump.allMetrics {
			if newPump.allMetrics[i].MetricType == counterType {
				prometheus.Unregister(newPump.allMetrics[i].counterVec)
			} else if newPump.allMetrics[i].MetricType == histogramType {
				prometheus.Unregister(newPump.allMetrics[i].histogramVec)
			}
		}
	}()

	metricMap := map[string]*PrometheusMetric{}
	for _, metric := range newPump.allMetrics {
		metricMap[metric.Name] = metric
	}

	assert.Contains(t, metricMap, "tyk_http_status")
	assert.NotContains(t, metricMap, "tyk_http_status_per_path")
}

// TestPrometheusGetLabelsValues_MCPLabels verifies that mcp_method, mcp_primitive_type,
// and mcp_primitive_name labels resolve to the correct values from MCPStats on MCP records,
// and to empty strings on non-MCP records.
func TestPrometheusGetLabelsValues_MCPLabels(t *testing.T) {
	mcpRecord := analytics.AnalyticsRecord{
		APIID:        "api_mcp",
		ResponseCode: 200,
		MCPStats: analytics.MCPStats{
			IsMCP:         true,
			JSONRPCMethod: "tools/call",
			PrimitiveType: "tool",
			PrimitiveName: "get_weather",
		},
	}
	restRecord := analytics.AnalyticsRecord{
		APIID:        "api_rest",
		ResponseCode: 200,
	}

	metric := PrometheusMetric{
		Name:       "test_mcp_labels",
		MetricType: counterType,
		Labels:     []string{"api_id", "mcp_method", "mcp_primitive_type", "mcp_primitive_name"},
	}

	t.Run("MCP record returns MCP label values", func(t *testing.T) {
		got := metric.GetLabelsValues(mcpRecord)
		assert.Equal(t, []string{"api_mcp", "tools/call", "tool", "get_weather"}, got)
	})

	t.Run("non-MCP record returns empty strings for MCP labels", func(t *testing.T) {
		got := metric.GetLabelsValues(restRecord)
		assert.Equal(t, []string{"api_rest", "", "", ""}, got)
	})
}

// TestPrometheusCreateBasicMetrics_IncludesMCPMetrics verifies that CreateBasicMetrics
// TestPrometheusCreateBasicMetrics_DoesNotIncludeMCPMetrics verifies that CreateBasicMetrics
// does not include MCP metrics, as they should be configured as custom metrics.
func TestPrometheusCreateBasicMetrics_DoesNotIncludeMCPMetrics(t *testing.T) {
	p := PrometheusPump{}
	p.CreateBasicMetrics()

	for _, m := range p.allMetrics {
		assert.False(t, m.MCPOnly, "%s must not have MCPOnly=true in base metrics", m.Name)
	}
}

// TestPrometheusMCPOnlyMetric_SkipsNonMCPRecords verifies that a metric with MCPOnly=true
// is not incremented for non-MCP analytics records.
// It tests the filtering by calling Inc() directly (simulating what WriteData does),
// so counterMap reflects only records that passed the filter.
func TestPrometheusMCPOnlyMetric_SkipsNonMCPRecords(t *testing.T) {
	metric := &PrometheusMetric{
		Name:       "test_mcp_only_counter_skip",
		Help:       "test",
		MetricType: counterType,
		Labels:     []string{"api_id", "mcp_method"},
		MCPOnly:    true,
	}
	require.NoError(t, metric.InitVec())
	defer prometheus.Unregister(metric.counterVec)

	p := &PrometheusPump{}
	loggerInstance := logrus.New()
	loggerInstance.Out = io.Discard
	p.log = logrus.NewEntry(loggerInstance)
	p.conf = &PrometheusConf{}
	p.allMetrics = []*PrometheusMetric{metric}

	records := []analytics.AnalyticsRecord{
		{APIID: "api1", ResponseCode: 200}, // REST — must be skipped
		{APIID: "api2", ResponseCode: 404}, // REST — must be skipped
		{APIID: "api3", ResponseCode: 200, MCPStats: analytics.MCPStats{ // MCP — must be counted
			IsMCP: true, JSONRPCMethod: "tools/call",
		}},
	}

	// Simulate what WriteData does per record, exercising the mcpOnly guard.
	for _, record := range records {
		if metric.MCPOnly && !record.IsMCPRecord() {
			continue
		}
		values := metric.GetLabelsValues(record)
		require.NoError(t, metric.Inc(values...))
	}

	assert.Len(t, metric.counterMap, 1, "only the MCP record should be counted")
	assert.Contains(t, metric.counterMap, "api3--tools/call")
}

// TestPrometheusMCPOnlyMetric_CountsMCPRecords verifies that a metric with mcpOnly=true
// is incremented only for MCP analytics records.
func TestPrometheusMCPOnlyMetric_CountsMCPRecords(t *testing.T) {
	metric := &PrometheusMetric{
		Name:       "test_mcp_only_counter_counts",
		Help:       "test",
		MetricType: counterType,
		Labels:     []string{"api_id", "mcp_method", "mcp_primitive_type", "mcp_primitive_name", "response_code"},
		MCPOnly:    true,
	}
	require.NoError(t, metric.InitVec())
	defer prometheus.Unregister(metric.counterVec)

	records := []analytics.AnalyticsRecord{
		{APIID: "api1", ResponseCode: 200, MCPStats: analytics.MCPStats{
			IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "get_weather",
		}},
		{APIID: "api1", ResponseCode: 200, MCPStats: analytics.MCPStats{
			IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "get_weather",
		}},
		{APIID: "api1", ResponseCode: 500, MCPStats: analytics.MCPStats{
			IsMCP: true, JSONRPCMethod: "resources/read", PrimitiveType: "resource", PrimitiveName: "docs",
		}},
	}

	for _, record := range records {
		if metric.MCPOnly && !record.IsMCPRecord() {
			continue
		}
		values := metric.GetLabelsValues(record)
		require.NoError(t, metric.Inc(values...))
	}

	assert.Len(t, metric.counterMap, 2)
	assert.Equal(t, uint64(2), metric.counterMap["api1--tools/call--tool--get_weather--200"].count)
	assert.Equal(t, uint64(1), metric.counterMap["api1--resources/read--resource--docs--500"].count)
}

// TestPrometheusMCPCustomMetric_MCPOnly verifies that a custom metric with MCPOnly=true
// is only processed for MCP records when configured as a custom metric.
func TestPrometheusMCPCustomMetric_MCPOnly(t *testing.T) {
	metric := &PrometheusMetric{
		Name:       "test_mcp_custom_counter",
		Help:       "MCP call counts",
		MetricType: counterType,
		Labels:     []string{"api_id", "mcp_method"},
		MCPOnly:    true,
	}
	require.NoError(t, metric.InitVec())
	defer prometheus.Unregister(metric.counterVec)

	p := newTestPrometheusPump(t)
	p.allMetrics = []*PrometheusMetric{metric}

	records := []analytics.AnalyticsRecord{
		{APIID: "api1", ResponseCode: 200},
		{APIID: "api2", ResponseCode: 200, MCPStats: analytics.MCPStats{
			IsMCP: true, JSONRPCMethod: "tools/call",
		}},
	}

	for _, record := range records {
		p.processMetric(metric, record)
	}

	assert.Len(t, metric.counterMap, 1, "only the MCP record should be counted")
	assert.Contains(t, metric.counterMap, "api2--tools/call")
}

func newTestPrometheusPump(t *testing.T) *PrometheusPump {
	t.Helper()
	p := &PrometheusPump{}
	loggerInstance := logrus.New()
	loggerInstance.Out = io.Discard
	p.log = logrus.NewEntry(loggerInstance)
	p.conf = &PrometheusConf{}
	return p
}

func TestProcessMetric_DisabledMetric(t *testing.T) {
	p := newTestPrometheusPump(t)
	metric := &PrometheusMetric{Name: "disabled_metric", MetricType: counterType, enabled: false}
	// must not panic and must be a no-op
	p.processMetric(metric, analytics.AnalyticsRecord{APIID: "api1"})
}

func TestProcessMetric_UnknownType(t *testing.T) {
	p := newTestPrometheusPump(t)
	metric := &PrometheusMetric{Name: "unknown_type_metric", MetricType: "unknown", enabled: true}
	// hits the default branch — must not panic
	p.processMetric(metric, analytics.AnalyticsRecord{APIID: "api1"})
}

func TestWriteData_ProcessesCounterMetric(t *testing.T) {
	metric := &PrometheusMetric{
		Name:       "test_write_data_counter",
		Help:       "test",
		MetricType: counterType,
		Labels:     []string{"api"},
	}
	require.NoError(t, metric.InitVec())
	defer prometheus.Unregister(metric.counterVec)

	p := newTestPrometheusPump(t)
	p.allMetrics = []*PrometheusMetric{metric}

	err := p.WriteData(context.Background(), []interface{}{
		analytics.AnalyticsRecord{APIID: "api1", ResponseCode: 200},
	})
	assert.NoError(t, err)
}

func TestWriteData_ContextCancellation(t *testing.T) {
	metric := &PrometheusMetric{
		Name:       "test_write_data_ctx_cancel",
		Help:       "test",
		MetricType: counterType,
		Labels:     []string{"api"},
	}
	require.NoError(t, metric.InitVec())
	defer prometheus.Unregister(metric.counterVec)

	p := newTestPrometheusPump(t)
	p.allMetrics = []*PrometheusMetric{metric}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := p.WriteData(ctx, []interface{}{
		analytics.AnalyticsRecord{APIID: "api1"},
	})
	assert.Error(t, err)
}

func TestCustomMetrics_Set(t *testing.T) {
	var metrics CustomMetrics
	err := metrics.Set(`[{"name":"test_metric","metric_type":"counter","labels":["api_id"]}]`)
	assert.NoError(t, err)
	require.Len(t, metrics, 1)
	assert.Equal(t, "test_metric", metrics[0].Name)
	assert.Equal(t, "counter", metrics[0].MetricType)

	assert.Error(t, metrics.Set(`not-json`))
}

func TestProcessMetric_HistogramType_NonLatency(t *testing.T) {
	metric := &PrometheusMetric{
		Name:       "test_process_histogram_nonlatency",
		Help:       "test",
		MetricType: histogramType,
		Labels:     []string{"type", "api"},
	}
	require.NoError(t, metric.InitVec())
	defer prometheus.Unregister(metric.histogramVec)

	p := newTestPrometheusPump(t)
	// hits the histogramType branch → observeHistogramMetric (non-tyk_latency name)
	p.processMetric(metric, analytics.AnalyticsRecord{APIID: "api1", RequestTime: 50})
}

func TestProcessMetric_HistogramType_LatencyMetric(t *testing.T) {
	// Create histogram vec with a unique name to avoid global registry conflicts.
	// Set the metric Name to metricTykLatency to trigger observeLatencyMetrics path.
	histVec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "test_custom_latency_metric_v2",
		Help:    "test",
		Buckets: buckets,
	}, []string{"type", "api"})
	prometheus.MustRegister(histVec)
	defer prometheus.Unregister(histVec)

	metric := &PrometheusMetric{
		Name:         metricTykLatency,
		MetricType:   histogramType,
		Labels:       []string{"api"},
		enabled:      true,
		histogramVec: histVec,
	}

	p := newTestPrometheusPump(t)
	// hits the histogramType branch → observeLatencyMetrics
	p.processMetric(metric, analytics.AnalyticsRecord{
		APIID:       "api1",
		RequestTime: 100,
		Latency:     analytics.Latency{Upstream: 50, Gateway: 10},
	})
}
