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
	"gopkg.in/vmihailenco/msgpack.v2"
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

func TestConnection(t *testing.T) {
	mPump := MongoSelectivePump{}
	conf := defaultSelectiveConf()
	mPump.dbConf = &conf
	// Checking if the connection is nil before connecting
	assert.Nil(t, mPump.store)
	mPump.log = log.WithField("prefix", mongoPrefix)

	t.Run("should connect to mgo", func(t *testing.T) {
		// If connect fails, it will stop the execution with a fatal error
		mPump.connect()
		// Checking if the connection is not nil after connecting
		assert.NotNil(t, mPump.store)
		// Checking if the connection is alive
		assert.Nil(t, mPump.store.Ping(context.Background()))
	})
}

func TestEnsureIndexes(t *testing.T) {
	mPump := MongoSelectivePump{}
	conf := defaultSelectiveConf()
	mPump.dbConf = &conf
	mPump.log = log.WithField("prefix", mongoPrefix)
	mPump.connect()

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

func TestWriteData(t *testing.T) {
	mPump := MongoSelectivePump{}
	conf := defaultSelectiveConf()
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
			conf := defaultConf()
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

func TestDecodeRequestAndDecodeResponseMongoSelective(t *testing.T) {
	newPump := &MongoSelectivePump{}
	conf := defaultConf()
	err := newPump.Init(conf)
	assert.Nil(t, err)

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

func TestDefaultDriverSelective(t *testing.T) {
	newPump := &MongoSelectivePump{}
	defaultConf := defaultConf()
	defaultConf.MongoDriverType = ""
	err := newPump.Init(defaultConf)
	assert.Nil(t, err)
	assert.Equal(t, persistent.Mgo, newPump.dbConf.MongoDriverType)
}
