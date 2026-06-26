package pumps

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/vmihailenco/msgpack.v2"
)

// SW-REQ-035:boundary:nominal
// SW-REQ-092:document_size_accounting_exact:boundary
func TestMongoSelectivePump_AccumulateSet(t *testing.T) {
	run := func(recordsGenerator func(numRecords int) []interface{}, expectedRecordsCount, maxDocumentSizeBytes int) func(t *testing.T) {
		return func(t *testing.T) {
			mPump := MongoSelectivePump{}
			conf := defaultSelectiveConf(nil)
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
			set := mPump.AccumulateSet(data, analytics.SQLTable) // SQLTable = "tyk_analytics"

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

// SW-REQ-035:output_cardinality_bounded:negative
// SW-REQ-035:boundary:negative
func TestMongoSelectivePump_AccumulateSet_DropsTCPErrorRecords(t *testing.T) {
	mPump := MongoSelectivePump{
		dbConf: &MongoSelectiveConf{
			MaxInsertBatchSizeBytes: 10 * MiB,
			MaxDocumentSizeBytes:    10 * MiB,
		},
	}
	mPump.log = log.WithField("prefix", mongoPrefix)

	httpRecord := analytics.AnalyticsRecord{
		OrgID:        "org-1",
		APIID:        "api-1",
		ResponseCode: 200,
	}
	tcpErrorRecord := analytics.AnalyticsRecord{
		OrgID:        "org-1",
		APIID:        "api-1",
		ResponseCode: -1,
	}

	set := mPump.AccumulateSet([]interface{}{tcpErrorRecord, httpRecord}, "z_tyk_analyticz_org-1")
	require.Len(t, set, 1)
	require.Len(t, set[0], 1)

	got, ok := set[0][0].(*analytics.AnalyticsRecord)
	require.True(t, ok)
	assert.Equal(t, 200, got.ResponseCode)
	assert.Equal(t, "z_tyk_analyticz_org-1", got.CollectionName)
}

// Verifies: SW-REQ-092
// SW-REQ-092:document_size_accounting_exact:nominal
// SW-REQ-092:document_size_accounting_exact:boundary
// MCDC SW-REQ-092: raw_request_and_response_counted_once=F, selective_document_size_estimated=F => TRUE
// MCDC SW-REQ-092: raw_request_and_response_counted_once=F, selective_document_size_estimated=T => FALSE
// MCDC SW-REQ-092: raw_request_and_response_counted_once=T, selective_document_size_estimated=T => TRUE
func TestMongoSelectivePump_GetItemSizeBytes_CountsRawRequestAndResponseOnce(t *testing.T) {
	tcs := []struct {
		name        string
		rawRequest  string
		rawResponse string
		maxBytes    int
		wantSize    int
	}{
		{
			name:        "request and response count once at exact threshold",
			rawRequest:  "rr",
			rawResponse: "ss",
			maxBytes:    1028,
			wantSize:    1028,
		},
		{
			name:        "response bytes are included in overflow",
			rawResponse: "ss",
			maxBytes:    1025,
			wantSize:    -1,
		},
		{
			name:       "request is not counted twice",
			rawRequest: "rr",
			maxBytes:   1026,
			wantSize:   1026,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mPump := MongoSelectivePump{
				dbConf: &MongoSelectiveConf{MaxDocumentSizeBytes: tc.maxBytes},
			}
			mPump.log = log.WithField("prefix", mongoPrefix)
			record := &analytics.AnalyticsRecord{
				RawRequest:  tc.rawRequest,
				RawResponse: tc.rawResponse,
			}

			assert.Equal(t, tc.wantSize, mPump.getItemSizeBytes(record))
		})
	}
}

// TestMongoSelectivePump_AccumulateSet_FinalOversizeDropsPendingBatch_KI
// reproduces mongo-selective-final-skipped-record-drops-pending-batch.
// Verifies: KI:mongo-selective-final-skipped-record-drops-pending-batch
// Reproduces: mongo-selective-final-skipped-record-drops-pending-batch
func TestMongoSelectivePump_AccumulateSet_FinalOversizeDropsPendingBatch_KI(t *testing.T) {
	mPump := MongoSelectivePump{
		dbConf: &MongoSelectiveConf{
			MaxInsertBatchSizeBytes: 10 * MiB,
			MaxDocumentSizeBytes:    1024,
		},
	}
	mPump.log = log.WithField("prefix", mongoPrefix)

	valid := analytics.AnalyticsRecord{}
	oversize := analytics.AnalyticsRecord{RawResponse: "x"}

	set := mPump.AccumulateSet([]interface{}{valid, oversize}, analytics.SQLTable)

	recordsCount := 0
	for _, setEntry := range set {
		recordsCount += len(setEntry)
	}
	assert.Equal(t, 0, recordsCount, "current KI: final skipped item prevents flushing the pending valid record")
}

// SW-REQ-035:nominal:nominal
func TestEnsureIndexes(t *testing.T) {
	mPump := MongoSelectivePump{}
	conf := defaultSelectiveConf(t)
	mPump.dbConf = &conf
	mPump.log = log.WithField("prefix", mongoPrefix)
	mPump.connect()
	t.Cleanup(func() { _ = mPump.store.DropDatabase(context.Background()) })

	// _id, apiid_1, expireAt_1, logBrowserIndex are the current indexes
	numberOfCreatedIndexes := 4

	t.Run("should ensure indexes", func(t *testing.T) {
		defer func() {
			assert.NoError(t, mPump.store.DropDatabase(context.Background()))
		}()
		collectionName := "index_test"
		obj := dbObject{
			tableName: collectionName,
		}

		err := mPump.ensureIndexes(collectionName)
		assert.NoError(t, err)

		// Checking if the indexes are created
		indexes, err := mPump.store.GetIndexes(context.Background(), obj)
		assert.NoError(t, err)
		assert.NotNil(t, indexes)

		// Checking if the indexes are created with the correct name
		fmt.Printf("indexes: %#v\n", indexes)
		assert.Len(t, indexes, numberOfCreatedIndexes)
		assert.Equal(t, "_id_", indexes[0].Name)
		assert.Equal(t, "apiid_1", indexes[1].Name)
		assert.Equal(t, "expireAt_1", indexes[2].Name)
		assert.Equal(t, "logBrowserIndex", indexes[3].Name)

		// Checking if the indexes are created with the correct keys
		assert.Len(t, indexes[0].Keys, 1)
		assert.Len(t, indexes[1].Keys, 1)
		assert.Len(t, indexes[2].Keys, 1)
		assert.Len(t, indexes[3].Keys, 4) // 4 keys because of the compound index: timestamp, apiid, apikey, responsecode
	})
	t.Run("should ensure one less index using CosmosDB", func(t *testing.T) {
		defer func() {
			mPump.dbConf.MongoDBType = StandardMongo
			assert.NoError(t, mPump.store.DropDatabase(context.Background()))
		}()
		mPump.dbConf.MongoDBType = CosmosDB
		collectionName := "index_test_cosmosdb"
		obj := dbObject{
			tableName: collectionName,
		}

		err := mPump.ensureIndexes(obj.TableName())
		assert.NoError(t, err)

		// Checking if the indexes are created
		indexes, err := mPump.store.GetIndexes(context.Background(), obj)
		assert.NoError(t, err)
		assert.NotNil(t, indexes)

		// Checking if the indexes are created with the correct name
		assert.Len(t, indexes, numberOfCreatedIndexes-1)
		assert.Equal(t, "_id_", indexes[0].Name)
		assert.Equal(t, "apiid_1", indexes[1].Name)
		assert.Equal(t, "logBrowserIndex", indexes[2].Name)

		// Checking if the indexes are created with the correct keys
		assert.Len(t, indexes[0].Keys, 1)
		assert.Len(t, indexes[1].Keys, 1)
		assert.Len(t, indexes[2].Keys, 4) // 4 keys because of the compound index: timestamp, apiid, apikey, responsecode
	})

	t.Run("should not ensure indexes because of omit index creation setting", func(t *testing.T) {
		defer func() {
			conf.OmitIndexCreation = false
			assert.NoError(t, mPump.store.DropDatabase(context.Background()))
		}()
		collectionName := "index_test"
		obj := dbObject{
			tableName: collectionName,
		}
		conf.OmitIndexCreation = true

		err := mPump.ensureIndexes(collectionName)
		assert.NoError(t, err)

		// Since the indexes were not created, the collection does not exist, and an error is expected
		indexes, err := mPump.store.GetIndexes(context.Background(), obj)
		assert.Error(t, err)
		assert.Nil(t, indexes)
	})

	t.Run("should not ensure indexes because the collection already exists", func(t *testing.T) {
		defer func() {
			assert.NoError(t, mPump.store.DropDatabase(context.Background()))
		}()
		collectionName := "index_test"
		obj := dbObject{
			tableName: collectionName,
		}
		// Creating the collection
		err := mPump.store.Migrate(context.Background(), []model.DBObject{obj})
		assert.NoError(t, err)

		// Creating the indexes
		err = mPump.ensureIndexes(collectionName)
		assert.NoError(t, err)

		// Checking if the indexes are created
		indexes, err := mPump.store.GetIndexes(context.Background(), obj)
		assert.NoError(t, err)
		assert.NotNil(t, indexes)

		// Checking if the default _id index is created
		assert.Len(t, indexes, 1)
		assert.Equal(t, "_id_", indexes[0].Name)
	})
}

// Verifies: SW-REQ-035
// SW-REQ-035:output_cardinality_bounded:nominal
// MCDC SW-REQ-035: org_id_present=F, record_routed_to_org_collection=F => TRUE
// MCDC SW-REQ-035: org_id_present=T, record_routed_to_org_collection=T => TRUE
func TestWriteData(t *testing.T) {
	mPump := MongoSelectivePump{}
	conf := defaultSelectiveConf(t)
	mPump.dbConf = &conf
	mPump.log = log.WithField("prefix", mongoPrefix)
	mPump.connect()
	defer func() {
		assert.NoError(t, mPump.store.DropDatabase(context.Background()))
	}()

	t.Run("should write 3 records", func(t *testing.T) {
		defer func() {
			assert.NoError(t, mPump.store.DropDatabase(context.Background()))
		}()
		data := []interface{}{
			analytics.AnalyticsRecord{
				APIID: "123",
				OrgID: "abc",
			},
			analytics.AnalyticsRecord{
				APIID: "456",
				OrgID: "abc",
			},
			analytics.AnalyticsRecord{
				APIID: "789",
				OrgID: "abc",
			},
		}
		err := mPump.WriteData(context.Background(), data)
		assert.NoError(t, err)

		var results []analytics.AnalyticsRecord
		colName, colErr := mPump.GetCollectionName("abc")
		assert.NoError(t, colErr)
		d := dummyObject{
			tableName: colName,
		}
		err = mPump.store.Query(context.Background(), &d, &results, nil)
		assert.NoError(t, err)
		assert.Len(t, results, 3)
		assert.Equal(t, "123", results[0].APIID)
		assert.Equal(t, "456", results[1].APIID)
		assert.Equal(t, "789", results[2].APIID)
	})

	t.Run("should not write data because the collection does not exist", func(t *testing.T) {
		defer func() {
			assert.NoError(t, mPump.store.DropDatabase(context.Background()))
		}()
		// data with empty orgID
		data := []interface{}{
			analytics.AnalyticsRecord{
				APIID: "123",
			},
		}
		err := mPump.WriteData(context.Background(), data)
		assert.NoError(t, err)

		var results []analytics.AnalyticsRecord
		err = mPump.store.Query(context.Background(), &analytics.AnalyticsRecord{}, &results, nil)
		assert.NoError(t, err)

		// No data should be written
		assert.Len(t, results, 0)
	})
}

// Verifies: SYS-REQ-021
// MCDC SYS-REQ-021: uptime_data_consumed=T, uptime_forwarded=T => TRUE
func TestWriteUptimeDataMongoSelective(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name                 string
		Record               interface{}
		RecordsAmountToWrite int
	}{
		{
			name:                 "write 3 uptime records",
			Record:               &analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now},
			RecordsAmountToWrite: 3,
		},
		{
			name:                 "write 6 uptime records",
			Record:               &analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now},
			RecordsAmountToWrite: 6,
		},
		{
			name:                 "length of records is 0",
			Record:               &analytics.UptimeReportData{},
			RecordsAmountToWrite: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newPump := &MongoSelectivePump{}
			conf := defaultConf(t)
			err := newPump.Init(conf)
			assert.Nil(t, err)

			keys := []interface{}{}
			for i := 0; i < test.RecordsAmountToWrite; i++ {
				encoded, err := msgpack.Marshal(test.Record)
				assert.Nil(t, err)
				keys = append(keys, string(encoded))
			}

			newPump.WriteUptimeData(keys)

			defer func() {
				// clean up the table
				err := newPump.store.DropDatabase(context.Background())
				assert.Nil(t, err)
			}()

			dbRecords := []analytics.UptimeReportData{}
			err = newPump.store.Query(context.Background(), &analytics.UptimeReportData{}, &dbRecords, model.DBM{})
			assert.NoError(t, err)

			// check amount of rows in the table
			assert.Equal(t, test.RecordsAmountToWrite, len(dbRecords))
		})
	}
}

// Verifies: INT-REQ-004
// MCDC INT-REQ-004: contract_honoured=T, pump_methods_called=T => TRUE
func TestDecodeRequestAndDecodeResponseMongoSelective(t *testing.T) {
	newPump := &MongoSelectivePump{}
	conf := defaultConf(t)
	err := newPump.Init(conf)
	assert.Nil(t, err)
	t.Cleanup(func() { _ = newPump.store.DropDatabase(context.Background()) })

	// checking if the default values are false
	assert.False(t, newPump.GetDecodedRequest())
	assert.False(t, newPump.GetDecodedResponse())

	// trying to set the values to true
	newPump.SetDecodingRequest(true)
	newPump.SetDecodingResponse(true)

	// checking if the values are still false as expected because this pump doesn't support decoding requests/responses
	assert.False(t, newPump.GetDecodedRequest())
	assert.False(t, newPump.GetDecodedResponse())
}

// SW-REQ-035:support_matrix_enforced:nominal
// SW-REQ-035:nominal:nominal
func TestDefaultDriverSelective(t *testing.T) {
	newPump := &MongoSelectivePump{}
	defaultConf := defaultConf(t)
	defaultConf.MongoDriverType = ""
	err := newPump.Init(defaultConf)
	assert.Nil(t, err)
	t.Cleanup(func() { _ = newPump.store.DropDatabase(context.Background()) })
	assert.Equal(t, persistent.OfficialMongo, newPump.dbConf.MongoDriverType)
}
