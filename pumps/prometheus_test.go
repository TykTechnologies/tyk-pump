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
				MetricType: "counter",
				Labels:     []string{"response_code", "api_id"},
			},
			expectedErr: nil,
			isEnabled:   true,
		},
		{
			testName: "Histogram metric",
			customMetric: PrometheusMetric{
				Name:       "testCounterMetric",
				MetricType: "counter",
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

			if tc.customMetric.MetricType == "counter" {
				assert.NotNil(t, tc.customMetric.counterVec)
				assert.Equal(t, tc.isEnabled, prometheus.Unregister(tc.customMetric.counterVec))

			} else if tc.customMetric.MetricType == "histogram" {
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
				MetricType: "counter",
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
				MetricType: "counter",
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
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			gotLabels := tc.customMetric.GetLabelsValues(tc.record)
			assert.EqualValues(t, tc.expectedLabels, gotLabels)
		})
	}
}
