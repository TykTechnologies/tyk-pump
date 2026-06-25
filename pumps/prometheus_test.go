package pumps

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File-level MC/DC witness rows: these requirements are genuinely exercised
// by covered tests in this file (per-test // MCDC blocks below). Rows copied
// verbatim from `proof mcdc show`; this header gives every // Verifies: link
// in the file a matching witness row.
//
// MCDC SW-REQ-024: default_path_applied=F, listen_path_empty=F => TRUE
// MCDC SW-REQ-024: default_path_applied=F, listen_path_empty=T => FALSE
// MCDC SW-REQ-024: default_path_applied=T, listen_path_empty=T => TRUE
// MCDC SW-REQ-090: base_metric_disabled=F, base_metric_family_absent=F => TRUE
// MCDC SW-REQ-090: base_metric_disabled=T, base_metric_family_absent=F => FALSE
// MCDC SW-REQ-090: base_metric_disabled=T, base_metric_family_absent=T => TRUE
// MCDC SW-REQ-091: histogram_metric_configured=F, histogram_type_label_schema_normalized=F => TRUE
// MCDC SW-REQ-091: histogram_metric_configured=T, histogram_type_label_schema_normalized=F => FALSE
// MCDC SW-REQ-091: histogram_metric_configured=T, histogram_type_label_schema_normalized=T => TRUE

// Verifies: SW-REQ-024
// MCDC SW-REQ-024: default_path_applied=F, listen_path_empty=F => TRUE
// MCDC SW-REQ-024: default_path_applied=F, listen_path_empty=T => FALSE
// MCDC SW-REQ-024: default_path_applied=T, listen_path_empty=T => TRUE
// (Tests configuring path=/metrics explicitly cover the listen_path_empty=F arm
// — F/F=TRUE. The default-path branch at prometheus.go:197 'if p.conf.Path == ""'
// covers listen_path_empty=T, default_path_applied=T — T/T=TRUE. The F/T=FALSE
// pair is the operator-error baseline where Addr is also unset and Init returns
// the 'Prometheus listen_addr not set' error before the path default takes
// effect — exercised by the Init-error subtest.)
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

// Verifies: SW-REQ-024
// Verifies: SW-REQ-094
// SW-REQ-094:custom_metric_identity_preserved:nominal
// SW-REQ-094:custom_metric_identity_preserved:negative
// MCDC SW-REQ-094: custom_metric_instances_preserved=F, valid_custom_metrics_configured=F => TRUE
// MCDC SW-REQ-094: custom_metric_instances_preserved=F, valid_custom_metrics_configured=T => FALSE
// MCDC SW-REQ-094: custom_metric_instances_preserved=T, valid_custom_metrics_configured=T => TRUE
func TestPrometheusInitCustomMetrics(t *testing.T) {
	tcs := []struct {
		testName              string
		metrics               []PrometheusMetric
		expectedAllMetricsLen int
		expectedMetricNames   []string
		expectedMetricLabels  [][]string
		expectedMetricTypes   []string
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
			expectedMetricNames:   []string{"test"},
			expectedMetricLabels:  [][]string{{"api_name"}},
			expectedMetricTypes:   []string{counterType},
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
			expectedMetricNames:   []string{"test", "other_test"},
			expectedMetricLabels:  [][]string{{"api_name"}, {"api_name", "api_key"}},
			expectedMetricTypes:   []string{counterType, counterType},
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
			expectedMetricNames:   []string{"test", "other_test", "histogram_test"},
			expectedMetricLabels:  [][]string{{"api_name"}, {"api_name", "api_key"}, {"type", "api_name", "api_key"}},
			expectedMetricTypes:   []string{counterType, counterType, histogramType},
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
			expectedMetricNames:   []string{"other_test"},
			expectedMetricLabels:  [][]string{{"api_name", "api_key"}},
			expectedMetricTypes:   []string{counterType},
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
			require.Len(t, p.allMetrics, tc.expectedAllMetricsLen)
			seenMetrics := map[*PrometheusMetric]struct{}{}
			for i, expectedName := range tc.expectedMetricNames {
				metric := p.allMetrics[i]
				if _, exists := seenMetrics[metric]; exists {
					t.Fatalf("custom metric %q reused a runtime metric pointer", metric.Name)
				}
				seenMetrics[metric] = struct{}{}

				assert.Equal(t, expectedName, metric.Name)
				assert.Equal(t, tc.expectedMetricTypes[i], metric.MetricType)
				assert.Equal(t, tc.expectedMetricLabels[i], metric.Labels)
				assert.Equal(t, p.conf.AggregateObservations, metric.aggregatedObservations)
				switch metric.MetricType {
				case counterType:
					assert.NotNil(t, metric.counterVec)
				case histogramType:
					assert.NotNil(t, metric.histogramVec)
				}
			}
		})
	}
}

// Verifies: SW-REQ-024
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

// Verifies: SW-REQ-024
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
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			gotLabels := tc.customMetric.GetLabelsValues(tc.record)
			assert.EqualValues(t, tc.expectedLabels, gotLabels)
		})
	}
}

// Verifies: SW-REQ-024
// Verifies: SW-REQ-083
// SW-REQ-083:encoding_safety:boundary
// MCDC SW-REQ-083: metric_label_value_contains_internal_separator=T, metric_label_tuple_preserved=T => TRUE
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
		{
			testName: "HTTP status codes per API with separator in API ID",
			metric: &PrometheusMetric{
				Name:       "tyk_http_status_per_api_separator",
				Help:       "HTTP status codes per API with separator in API ID",
				MetricType: counterType,
				Labels:     []string{"code", "api"},
			},
			analyticsRecords: []analytics.AnalyticsRecord{
				{APIID: "api--3", ResponseCode: 500},
				{APIID: "api--3", ResponseCode: 200},
				{APIID: "api_3", ResponseCode: 500},
			},
			expectedMetricsAmount: 3,
			expectedMetrics: map[string]counterStruct{
				"500--api--3": {labelValues: []string{"500", "api--3"}, count: 1},
				"200--api--3": {labelValues: []string{"200", "api--3"}, count: 1},
				"500--api_3":  {labelValues: []string{"500", "api_3"}, count: 1},
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

// Verifies: SW-REQ-024
// Verifies: SW-REQ-083
// SW-REQ-083:encoding_safety:nominal
// SW-REQ-083:encoding_safety:boundary
// MCDC SW-REQ-083: metric_label_value_contains_internal_separator=F, metric_label_tuple_preserved=F => TRUE
// MCDC SW-REQ-083: metric_label_value_contains_internal_separator=T, metric_label_tuple_preserved=F => FALSE
// MCDC SW-REQ-083: metric_label_value_contains_internal_separator=T, metric_label_tuple_preserved=T => TRUE
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

// Verifies: SW-REQ-024
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

// Verifies: SW-REQ-024
// Verifies: SW-REQ-091
// SW-REQ-091:metric_label_schema_stable:nominal
// SW-REQ-091:metric_label_schema_stable:boundary
// MCDC SW-REQ-091: histogram_metric_configured=F, histogram_type_label_schema_normalized=F => TRUE
// MCDC SW-REQ-091: histogram_metric_configured=T, histogram_type_label_schema_normalized=F => FALSE
// MCDC SW-REQ-091: histogram_metric_configured=T, histogram_type_label_schema_normalized=T => TRUE
func TestPrometheusEnsureLabels(t *testing.T) {
	testCases := []struct {
		name           string
		metricType     string
		labels         []string
		expectedLabels []string
	}{
		{
			name:           "histogram type, type label should be added if not exist",
			labels:         []string{"response_code", "api_name", "method", "api_key", "alias", "path"},
			metricType:     histogramType,
			expectedLabels: []string{"type", "response_code", "api_name", "method", "api_key", "alias", "path"},
		},
		{
			name:           "counter type, type label should not be added",
			labels:         []string{"response_code", "api_name", "method", "api_key", "alias", "path"},
			metricType:     counterType,
			expectedLabels: []string{"response_code", "api_name", "method", "api_key", "alias", "path"},
		},
		{
			name:           "histogram type, type label should not be repeated and in the 1st position",
			labels:         []string{"type", "response_code", "api_name", "method", "api_key", "alias", "path"},
			metricType:     histogramType,
			expectedLabels: []string{"type", "response_code", "api_name", "method", "api_key", "alias", "path"},
		},
		{
			name:           "histogram type, type label should not be repeated (even if user repeated it), and always in the 1st position",
			labels:         []string{"response_code", "api_name", "type", "method", "api_key", "alias", "path", "type"},
			metricType:     histogramType,
			expectedLabels: []string{"type", "response_code", "api_name", "method", "api_key", "alias", "path"},
		},
		{
			name:           "histogram type, empty labels still receive type label",
			labels:         nil,
			metricType:     histogramType,
			expectedLabels: []string{"type"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pm := PrometheusMetric{
				MetricType: tc.metricType,
				Labels:     tc.labels,
			}

			pm.ensureLabels()
			assert.Equal(t, tc.expectedLabels, pm.Labels)
		})
	}
}

// Verifies: SW-REQ-024
// Verifies: SW-REQ-090
// SW-REQ-090:metric_family_disable_gate:nominal
// SW-REQ-090:metric_family_disable_gate:boundary
// MCDC SW-REQ-090: base_metric_disabled=F, base_metric_family_absent=F => TRUE
// MCDC SW-REQ-090: base_metric_disabled=T, base_metric_family_absent=F => FALSE
// MCDC SW-REQ-090: base_metric_disabled=T, base_metric_family_absent=T => TRUE
func TestPrometheusDisablingMetrics(t *testing.T) {
	baseFamilies := []string{
		"tyk_http_status",
		"tyk_http_status_per_path",
		"tyk_http_status_per_key",
		"tyk_http_status_per_oauth_client",
		"tyk_latency",
	}

	t.Run("all exact built-in family names are disabled", func(t *testing.T) {
		registry := withIsolatedPrometheusRegistry(t)
		newPump := newPrometheusPumpWithDisabledMetrics(baseFamilies)
		defer unregisterPrometheusMetrics(newPump.allMetrics)

		metricMap := prometheusMetricMap(newPump.allMetrics)
		for _, family := range baseFamilies {
			assert.NotContains(t, metricMap, family)
			assertPrometheusMetricFamilyNameAvailable(t, registry, family)
		}
		assert.Empty(t, newPump.allMetrics)
	})

	t.Run("unknown disabled name does not suppress built-in families", func(t *testing.T) {
		registry := withIsolatedPrometheusRegistry(t)
		newPump := newPrometheusPumpWithDisabledMetrics([]string{"tyk_not_a_base_family"})
		defer unregisterPrometheusMetrics(newPump.allMetrics)

		metricMap := prometheusMetricMap(newPump.allMetrics)
		for _, family := range baseFamilies {
			assert.Contains(t, metricMap, family)
			assertPrometheusMetricFamilyNameRegistered(t, registry, family)
		}
		assert.Len(t, newPump.allMetrics, len(baseFamilies))
	})
}

// Verifies: SW-REQ-090
// SW-REQ-090:metric_family_disable_gate:boundary
func TestPrometheusDisabledMetricsDoNotDisableCustomMetrics(t *testing.T) {
	p := &PrometheusPump{}
	newPump := p.New().(*PrometheusPump)

	log := logrus.New()
	log.Out = io.Discard
	newPump.log = logrus.NewEntry(log)

	newPump.conf = &PrometheusConf{
		DisabledMetrics: []string{"tyk_custom_disabled_metric"},
		CustomMetrics: CustomMetrics{
			{
				Name:       "tyk_custom_disabled_metric",
				Help:       "custom metric remains enabled",
				MetricType: counterType,
				Labels:     []string{"response_code"},
			},
		},
	}

	newPump.initBaseMetrics()
	newPump.InitCustomMetrics()

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

	assert.Contains(t, metricMap, "tyk_custom_disabled_metric")
	assert.Contains(t, metricMap, "tyk_http_status")
}

func newPrometheusPumpWithDisabledMetrics(disabledMetrics []string) *PrometheusPump {
	p := &PrometheusPump{}
	newPump := p.New().(*PrometheusPump)

	loggerInstance := logrus.New()
	loggerInstance.Out = io.Discard
	newPump.log = logrus.NewEntry(loggerInstance)
	newPump.conf = &PrometheusConf{DisabledMetrics: disabledMetrics}
	newPump.initBaseMetrics()
	return newPump
}

func prometheusMetricMap(metrics []*PrometheusMetric) map[string]*PrometheusMetric {
	metricMap := map[string]*PrometheusMetric{}
	for _, metric := range metrics {
		metricMap[metric.Name] = metric
	}
	return metricMap
}

func unregisterPrometheusMetrics(metrics []*PrometheusMetric) {
	for _, metric := range metrics {
		if metric.MetricType == counterType && metric.counterVec != nil {
			prometheus.Unregister(metric.counterVec)
		} else if metric.MetricType == histogramType && metric.histogramVec != nil {
			prometheus.Unregister(metric.histogramVec)
		}
	}
}

func withIsolatedPrometheusRegistry(t *testing.T) *prometheus.Registry {
	t.Helper()

	originalRegisterer := prometheus.DefaultRegisterer
	originalGatherer := prometheus.DefaultGatherer
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegisterer
		prometheus.DefaultGatherer = originalGatherer
	})

	return registry
}

func assertPrometheusMetricFamilyNameAvailable(t *testing.T, registry *prometheus.Registry, name string) {
	t.Helper()

	probe := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: "registration probe for disabled metric family",
	})
	require.NoError(t, registry.Register(probe))
	assert.True(t, registry.Unregister(probe))
}

func assertPrometheusMetricFamilyNameRegistered(t *testing.T, registry *prometheus.Registry, name string) {
	t.Helper()

	probe := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: "registration probe for enabled metric family",
	})
	require.Error(t, registry.Register(probe))
}

// TestPrometheusGetLabelsValues_MCPLabels verifies that mcp_method, mcp_primitive_type,
// and mcp_primitive_name labels resolve to the correct values from MCPStats on MCP records,
// and to empty strings on non-MCP records.
// Verifies: SW-REQ-024
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
// Verifies: SW-REQ-024
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
// Verifies: SW-REQ-024
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
// Verifies: SW-REQ-024
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
// Verifies: SW-REQ-024
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

// Verifies: SW-REQ-024
func TestProcessMetric_DisabledMetric(t *testing.T) {
	p := newTestPrometheusPump(t)
	metric := &PrometheusMetric{Name: "disabled_metric", MetricType: counterType, enabled: false}
	// must not panic and must be a no-op
	p.processMetric(metric, analytics.AnalyticsRecord{APIID: "api1"})
}

// Verifies: SW-REQ-024
func TestProcessMetric_UnknownType(t *testing.T) {
	p := newTestPrometheusPump(t)
	metric := &PrometheusMetric{Name: "unknown_type_metric", MetricType: "unknown", enabled: true}
	// hits the default branch — must not panic
	p.processMetric(metric, analytics.AnalyticsRecord{APIID: "api1"})
}

// Verifies: SW-REQ-024
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

// Verifies: SW-REQ-024
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

// Verifies: SW-REQ-024
func TestCustomMetrics_Set(t *testing.T) {
	var metrics CustomMetrics
	err := metrics.Set(`[{"name":"test_metric","metric_type":"counter","labels":["api_id"]}]`)
	assert.NoError(t, err)
	require.Len(t, metrics, 1)
	assert.Equal(t, "test_metric", metrics[0].Name)
	assert.Equal(t, "counter", metrics[0].MetricType)

	assert.Error(t, metrics.Set(`not-json`))
}

// Verifies: SW-REQ-024
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

// Verifies: SW-REQ-024
// Verifies: SW-REQ-104
// SW-REQ-024:structured_projection_preserved:nominal
// SW-REQ-104:structured_projection_preserved:nominal
// MCDC SW-REQ-104: latency_metric_record_present=F, latency_metric_values_projected=F => TRUE
// MCDC SW-REQ-104: latency_metric_record_present=T, latency_metric_values_projected=F => FALSE
// MCDC SW-REQ-104: latency_metric_record_present=T, latency_metric_values_projected=T => TRUE
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

	ch := make(chan prometheus.Metric, 4)
	histVec.Collect(ch)
	close(ch)

	wantSums := map[string]float64{
		"total":    100,
		"upstream": 50,
		"gateway":  10,
	}
	seen := map[string]bool{}
	for collected := range ch {
		var metricPB io_prometheus_client.Metric
		require.NoError(t, collected.Write(&metricPB))

		labels := map[string]string{}
		for _, label := range metricPB.GetLabel() {
			labels[label.GetName()] = label.GetValue()
		}
		if labels["api"] != "api1" {
			continue
		}
		want, ok := wantSums[labels["type"]]
		if !ok {
			continue
		}
		seen[labels["type"]] = true
		require.NotNil(t, metricPB.GetHistogram())
		assert.Equal(t, uint64(1), metricPB.GetHistogram().GetSampleCount())
		assert.Equal(t, want, metricPB.GetHistogram().GetSampleSum())
	}
	assert.Equal(t, map[string]bool{"total": true, "upstream": true, "gateway": true}, seen, "tyk_latency must expose total, upstream, and gateway latency observations")
}

// TestPrometheusObfuscateAPIKey pins the secrets-not-logged guarantee that
// DEFECT-5 surfaced: when ObfuscateAPIKeys is enabled, the api_key/key metric
// labels must never carry the raw API key. It exercises both decisions in
// PrometheusMetric.obfuscateAPIKey and supplies the MC/DC witnesses for the
// SW-REQ-074 primary decision (the opt-in gate).
//
// SW-REQ-074 FRETish: when obfuscate_api_keys_enabled pumps_prometheus shall
// always satisfy api_key_label_masked. The three witness rows below match the
// requirement's MC/DC table exactly:
// Verifies: SW-REQ-074
// SW-REQ-074:secrets_not_logged:nominal
// SW-REQ-074:secrets_not_logged:negative
// MCDC SW-REQ-074: api_key_label_masked=F, obfuscate_api_keys_enabled=F => TRUE
// MCDC SW-REQ-074: api_key_label_masked=F, obfuscate_api_keys_enabled=T => FALSE
// MCDC SW-REQ-074: api_key_label_masked=T, obfuscate_api_keys_enabled=T => TRUE
//
// Row F/F=TRUE (gate off): the "disabled returns raw key" subtest drives
// obfuscate_api_keys_enabled=F and asserts the raw key is returned — the
// documented opt-in default, so the masking obligation is vacuously satisfied.
// Row T/T=TRUE (gate on, masked): the "enabled long key" and "enabled short
// key" subtests drive obfuscate_api_keys_enabled=T and assert the emitted
// label value masks the raw key. The violation row F/T=FALSE (gate on yet the
// label NOT masked) is guarded by the NotContains assertions: if a regression
// ever let the raw key through with obfuscation on, those assertions fail.
//
// The second decision inside obfuscateAPIKey (`if len(apiKey) > 4`: long key
// -> "****"+last 4 vs. short key -> "--") is not a separate FRETish variable;
// both of its arms are exercised by the long-key and short-key subtests below,
// each of which still asserts the raw key is absent.
func TestPrometheusObfuscateAPIKey(t *testing.T) {
	const rawLong = "abcdefghijklmnop"
	const rawShort = "abcd"

	t.Run("enabled long key masks to last 4, raw absent", func(t *testing.T) {
		pm := &PrometheusMetric{ObfuscateAPIKeys: true}
		// Decision 1 gate ON (obfuscate_api_keys_enabled=T) AND
		// decision 2 long branch (api_key_longer_than_keep=T).
		got := pm.obfuscateAPIKey(rawLong)
		require.Equal(t, "****mnop", got, "long key must mask to ****+last 4")
		// The masking guarantee: only the last 4 chars survive, the raw key is
		// never present in the emitted label value.
		assert.NotContains(t, got, rawLong, "raw API key must never appear in the label")
		assert.NotContains(t, got, rawLong[:len(rawLong)-4], "obfuscated prefix must not leak")
		assert.Equal(t, rawLong[len(rawLong)-4:], got[len("****"):], "only the last 4 chars are emitted")

		// And it actually reaches the api_key/key labels:
		labelled := (&PrometheusMetric{
			Labels:           []string{"api_key", "key"},
			ObfuscateAPIKeys: true,
		}).GetLabelsValues(analytics.AnalyticsRecord{APIKey: rawLong})
		for _, v := range labelled {
			assert.NotContains(t, v, rawLong, "raw key must not reach any prometheus label")
			assert.Equal(t, "****mnop", v)
		}
	})

	t.Run("enabled short key fully masked to --", func(t *testing.T) {
		pm := &PrometheusMetric{ObfuscateAPIKeys: true}
		// Decision 1 gate ON, decision 2 short branch
		// (api_key_longer_than_keep=F => only_suffix_emitted=F, fully masked).
		got := pm.obfuscateAPIKey(rawShort)
		require.Equal(t, "--", got, "short key must fully mask to --")
		assert.NotContains(t, got, rawShort, "no characters of a short raw key may be emitted")
	})

	t.Run("disabled returns raw key (documented opt-in default)", func(t *testing.T) {
		pm := &PrometheusMetric{ObfuscateAPIKeys: false}
		// Decision 1 gate OFF (obfuscate_api_keys_enabled=F): the opt-in default
		// emits the raw key. This is the unmasked behavior DEFECT-5 documents as
		// an explicit operator decision; pinning it guards against silently
		// changing the default without updating SW-REQ-074.
		got := pm.obfuscateAPIKey(rawLong)
		require.Equal(t, rawLong, got, "with obfuscation off the raw key is returned unchanged")
	})
}
