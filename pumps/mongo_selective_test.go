package pumps

import (
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2"
)

func TestMongoSelectivePump_AccumulateSet(t *testing.T) {
	run := func(recordsGenerator func(numRecords int) []interface{}, expectedRecordsCount, maxDocumentSizeBytes int) func(t *testing.T) {
		return func(t *testing.T) {
			mPump := MongoSelectivePump{}
			conf := defaultSelectiveConf()
			conf.MaxDocumentSizeBytes = maxDocumentSizeBytes

			numRecords := 100

			mPump.dbConf = &conf
			mPump.log = log.WithField("prefix", mongoPrefix)

			data := recordsGenerator(numRecords)
			expectedGraphRecordSkips := 0
			for _, recordData := range data {
				record, ok := recordData.(analytics.AnalyticsRecord)
				if !ok {
					continue
				}
				if record.IsGraphRecord() {
					expectedGraphRecordSkips++
				}
			}
			set := mPump.AccumulateSet(data)

			recordsCount := 0
			for _, setEntry := range set {
				recordsCount += len(setEntry)
			}
			assert.Equal(t, expectedRecordsCount, recordsCount)
		}
	}

	t.Run("should accumulate all records", run(
		func(numRecords int) []interface{} {
			record := analytics.AnalyticsRecord{}
			data := make([]interface{}, 0)
			for i := 0; i < numRecords; i++ {
				data = append(data, record)
			}
			return data
		},
		100,
		5120,
	))

	t.Run("should accumulate 0 records because maxDocumentSizeBytes < 1024", run(
		func(numRecords int) []interface{} {
			record := analytics.AnalyticsRecord{}
			data := make([]interface{}, 0)
			for i := 0; i < numRecords; i++ {
				data = append(data, record)
			}
			return data
		},
		0,
		100,
	))

	t.Run("should accumulate 0 records because the length of the data (1500) is > 1024", run(
		func(numRecords int) []interface{} {
			record := analytics.AnalyticsRecord{}
			record.RawResponse = "1"
			data := make([]interface{}, 0)
			for i := 0; i < 1500; i++ {
				data = append(data, record)
			}
			return data
		},
		0,
		1024,
	))

	t.Run("should accumulate 99 records because one of the 100 records exceeds the limit of 1024", run(
		func(numRecords int) []interface{} {
			data := make([]interface{}, 0)
			for i := 0; i < 100; i++ {
				record := analytics.AnalyticsRecord{}
				if i == 94 {
					record.RawResponse = "1"
				}
				data = append(data, record)
			}
			return data
		},
		99,
		1024,
	))
}

func TestMongoSelectivePump_SessionConsistency(t *testing.T) {
	mPump := MongoSelectivePump{}
	conf := defaultSelectiveConf()
	mPump.dbConf = &conf

	tests := []struct {
		testName            string
		sessionConsistency  string
		expectedSessionMode mgo.Mode
	}{
		{
			testName:            "should set session mode to strong",
			sessionConsistency:  "strong",
			expectedSessionMode: mgo.Strong,
		},
		{
			testName:            "should set session mode to monotonic",
			sessionConsistency:  "monotonic",
			expectedSessionMode: mgo.Monotonic,
		},
		{
			testName:            "should set session mode to eventual",
			sessionConsistency:  "eventual",
			expectedSessionMode: mgo.Eventual,
		},
		{
			testName:            "should set session mode to strong by default",
			sessionConsistency:  "",
			expectedSessionMode: mgo.Strong,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			mPump.dbConf.MongoSessionConsistency = test.sessionConsistency
			mPump.connect()
			assert.Equal(t, test.expectedSessionMode, mPump.dbSession.Mode())
		})
	}
}
