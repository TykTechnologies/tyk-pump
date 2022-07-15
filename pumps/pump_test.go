package pumps

import (
	"errors"
	"testing"

	"github.com/TykTechnologies/tyk-pump/pumps/mongo"
	"github.com/stretchr/testify/assert"
)

func TestGetPumpByName(t *testing.T) {
	tcs := []struct {
		testName string

		pmpType      string
		expectedPump Pump
		expectedErr  error
	}{
		{
			testName:     "invalid pump type",
			pmpType:      "xyz",
			expectedPump: nil,
			expectedErr:  errors.New("xyz Not found"),
		},
		{
			testName:     "dummy pump",
			pmpType:      "dummy",
			expectedPump: &DummyPump{},
			expectedErr:  nil,
		},
		{
			testName:     "mongo pump",
			pmpType:      "mongo",
			expectedPump: &mongo.Pump{},
			expectedErr:  nil,
		},
		{
			testName:     "mongo-selective pump",
			pmpType:      "mongo-pump-selective",
			expectedPump: &MongoSelectivePump{},
			expectedErr:  nil,
		}, {
			testName:     "mongo-aggregate pump",
			pmpType:      "mongo-pump-aggregate",
			expectedPump: &MongoAggregatePump{},
			expectedErr:  nil,
		}, {
			testName:     "csv pump",
			pmpType:      "csv",
			expectedPump: &CSVPump{},
			expectedErr:  nil,
		}, {
			testName:     "elasticsearch pump",
			pmpType:      "elasticsearch",
			expectedPump: &ElasticsearchPump{},
			expectedErr:  nil,
		}, {
			testName:     "influx pump",
			pmpType:      "influx",
			expectedPump: &InfluxPump{},
			expectedErr:  nil,
		}, {
			testName:     "influx2 pump",
			pmpType:      "influx2",
			expectedPump: &Influx2Pump{},
			expectedErr:  nil,
		}, {
			testName:     "moesif pump",
			pmpType:      "moesif",
			expectedPump: &MoesifPump{},
			expectedErr:  nil,
		}, {
			testName:     "statsd pump",
			pmpType:      "statsd",
			expectedPump: &StatsdPump{},
			expectedErr:  nil,
		}, {
			testName:     "segment pump",
			pmpType:      "segment",
			expectedPump: &SegmentPump{},
			expectedErr:  nil,
		},
		{
			testName:     "graylog pump",
			pmpType:      "graylog",
			expectedPump: &GraylogPump{},
			expectedErr:  nil,
		},
		{
			testName:     "splunk pump",
			pmpType:      "splunk",
			expectedPump: &SplunkPump{},
			expectedErr:  nil,
		}, {
			testName:     "hybrid pump",
			pmpType:      "hybrid",
			expectedPump: &HybridPump{},
			expectedErr:  nil,
		}, {
			testName:     "prometheus pump",
			pmpType:      "prometheus",
			expectedPump: &PrometheusPump{},
			expectedErr:  nil,
		}, {
			testName:     "logzio pump",
			pmpType:      "logzio",
			expectedPump: &LogzioPump{},
			expectedErr:  nil,
		}, {
			testName:     "dogstatsd pump",
			pmpType:      "dogstatsd",
			expectedPump: &DogStatsdPump{},
			expectedErr:  nil,
		}, {
			testName:     "kafka pump",
			pmpType:      "kafka",
			expectedPump: &KafkaPump{},
			expectedErr:  nil,
		}, {
			testName:     "syslog pump",
			pmpType:      "syslog",
			expectedPump: &SyslogPump{},
			expectedErr:  nil,
		}, {
			testName:     "sql pump",
			pmpType:      "sql",
			expectedPump: &SQLPump{},
			expectedErr:  nil,
		}, {
			testName:     "sql_aggregate pump",
			pmpType:      "sql_aggregate",
			expectedPump: &SQLAggregatePump{},
			expectedErr:  nil,
		}, {
			testName:     "stdout pump",
			pmpType:      "stdout",
			expectedPump: &StdOutPump{},
			expectedErr:  nil,
		}, {
			testName:     "timestream pump",
			pmpType:      "timestream",
			expectedPump: &TimestreamPump{},
			expectedErr:  nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			pmp, err := GetPumpByName(tc.pmpType)

			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedPump, pmp)
		})
	}
}
