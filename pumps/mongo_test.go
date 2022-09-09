package pumps

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

func newPump() Pump {
	return (&MongoPump{}).New()
}

func TestMongoPump_capCollection_Enabled(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf
	mPump.dbConf.CollectionCapEnable = false
	mPump.log = log.WithField("prefix", mongoPrefix)

	mPump.connect()

	if ok := mPump.capCollection(); ok {
		t.Error("successfully capped collection when disabled in conf")
	}
}

func TestMongoPumpOmitIndexCreation(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf
	record := analytics.AnalyticsRecord{
		OrgID: "test-org",
		APIID: "test-api",
	}
	records := []interface{}{record, record}

	tcs := []struct {
		testName             string
		shouldDropCollection bool
		ExpectedIndexes      int
		OmitIndexCreation    bool
		dbType               MongoType
	}{
		{
			testName:             "omitting index creation - StandardMongo",
			shouldDropCollection: true,
			ExpectedIndexes:      1, //1 index corresponding to _id
			OmitIndexCreation:    true,
			dbType:               StandardMongo,
		},
		{
			testName:             "not omitting index creation but mongo collection already exists - StandardMongo",
			shouldDropCollection: false,
			ExpectedIndexes:      1, //1 index corresponding to _id
			OmitIndexCreation:    false,
			dbType:               StandardMongo,
		},
		{
			testName:             "not omitting index creation but mongo collection doesn't exists - StandardMongo",
			shouldDropCollection: true,
			ExpectedIndexes:      4, //1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               StandardMongo,
		},
		{
			testName:             "omitting index creation - DocDB",
			shouldDropCollection: true,
			ExpectedIndexes:      1, //1 index corresponding to _id
			OmitIndexCreation:    true,
			dbType:               AWSDocumentDB,
		},
		{
			testName:             "not omitting index creation but mongo collection already exists - DocDB",
			shouldDropCollection: false,
			ExpectedIndexes:      4, //1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               AWSDocumentDB,
		},
		{
			testName:             "not omitting index creation but mongo collection doesn't exists - DocDB",
			shouldDropCollection: true,
			ExpectedIndexes:      4, //1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               AWSDocumentDB,
		},
		{
			testName:             "omitting index creation - CosmosDB",
			shouldDropCollection: true,
			ExpectedIndexes:      1, // 1 index corresponding to _id
			OmitIndexCreation:    true,
			dbType:               CosmosDB,
		},
		{
			testName:             "not omitting index creation but mongo collection already exists - CosmosDB",
			shouldDropCollection: false,
			ExpectedIndexes:      4, // 1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               CosmosDB,
		},
		{
			testName:             "not omitting index creation but mongo collection doesn't exists - CosmosDB",
			shouldDropCollection: true,
			ExpectedIndexes:      4, // 1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               CosmosDB,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			mPump.dbConf.OmitIndexCreation = tc.OmitIndexCreation
			mPump.dbConf.MongoDBType = tc.dbType
			mPump.log = log.WithField("prefix", mongoPrefix)
			mPump.connect()
			defer c.CleanIndexes()

			if tc.shouldDropCollection {
				c.CleanDb()
			}

			if err := mPump.ensureIndexes(); err != nil {
				t.Error("there shouldn't be an error ensuring indexes", err)
			}

			mPump.WriteData(context.Background(), records)
			indexes, errIndexes := c.GetIndexes()
			if errIndexes != nil {
				t.Error("error getting indexes:", errIndexes)
			}

			if len(indexes) != tc.ExpectedIndexes {
				t.Errorf("wanted %v index but got %v indexes", tc.ExpectedIndexes, len(indexes))
			}
		})
	}
}

func TestMongoPump_capCollection_Exists(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	c.InsertDoc()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf
	mPump.log = log.WithField("prefix", mongoPrefix)

	mPump.dbConf.CollectionCapEnable = true

	mPump.connect()

	if ok := mPump.capCollection(); ok {
		t.Error("successfully capped collection when already exists")
	}
}

func TestMongoPump_capCollection_Not64arch(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	if strconv.IntSize >= 64 {
		t.Skip("skipping as >= 64bit arch")
	}

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf
	mPump.log = log.WithField("prefix", mongoPrefix)

	mPump.dbConf.CollectionCapEnable = true

	mPump.connect()

	if ok := mPump.capCollection(); ok {
		t.Error("should not be able to cap collection when running < 64bit architecture")
	}
}

func TestMongoPump_capCollection_SensibleDefaultSize(t *testing.T) {

	if strconv.IntSize < 64 {
		t.Skip("skipping as < 64bit arch")
	}

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf
	mPump.log = log.WithField("prefix", mongoPrefix)

	mPump.dbConf.CollectionCapEnable = true
	mPump.dbConf.CollectionCapMaxSizeBytes = 0

	mPump.connect()

	if ok := mPump.capCollection(); !ok {
		t.Fatal("should have capped collection")
	}

	colStats := c.GetCollectionStats()

	defSize := 5
	if colStats["maxSize"].(int64) != int64(defSize*GiB) {
		t.Errorf("wrong sized capped collection created. Expected (%d), got (%d)", mPump.dbConf.CollectionCapMaxSizeBytes, colStats["maxSize"])
	}
}

func TestMongoPump_capCollection_OverrideSize(t *testing.T) {

	if strconv.IntSize < 64 {
		t.Skip("skipping as < 64bit arch")
	}

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf
	mPump.log = log.WithField("prefix", mongoPrefix)

	mPump.dbConf.CollectionCapEnable = true
	mPump.dbConf.CollectionCapMaxSizeBytes = GiB

	mPump.connect()

	if ok := mPump.capCollection(); !ok {
		t.Error("should have capped collection")
		t.FailNow()
	}

	colStats := c.GetCollectionStats()

	if colStats["maxSize"].(int64) != int64(mPump.dbConf.CollectionCapMaxSizeBytes) {
		t.Errorf("wrong sized capped collection created. Expected (%d), got (%d)", mPump.dbConf.CollectionCapMaxSizeBytes, colStats["maxSize"])
	}
}

func TestMongoPump_AccumulateSet(t *testing.T) {
	run := func(recordsGenerator func(numRecords int) []interface{}, expectedRecordsCount int) func(t *testing.T) {
		return func(t *testing.T) {
			pump := newPump()
			conf := defaultConf()
			conf.MaxInsertBatchSizeBytes = 5120

			numRecords := 100

			mPump := pump.(*MongoPump)
			mPump.dbConf = &conf
			mPump.log = log.WithField("prefix", mongoPrefix)

			data := recordsGenerator(numRecords)
			expectedGraphRecordSkips := 0
			for _, recordData := range data {
				record := recordData.(analytics.AnalyticsRecord)
				if record.IsGraphRecord() {
					expectedGraphRecordSkips++
				}
			}

			// assumed from sizeBytes in AccumulateSet
			const dataSize = 1024
			totalData := dataSize * (numRecords - expectedGraphRecordSkips)

			set := mPump.AccumulateSet(data)

			recordsCount := 0
			for _, setEntry := range set {
				recordsCount += len(setEntry)
			}
			assert.Equal(t, expectedRecordsCount, recordsCount)

			if len(set) != totalData/conf.MaxInsertBatchSizeBytes {
				t.Errorf("expected accumulator chunks to equal %d, got %d", totalData/conf.MaxInsertBatchSizeBytes, len(set))
			}
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
	))

	t.Run("should skip all graph analytics records", run(
		func(numRecords int) []interface{} {
			data := make([]interface{}, 0)
			for i := 0; i < numRecords; i++ {
				record := analytics.AnalyticsRecord{}
				if i%2 == 0 {
					record.Tags = []string{analytics.PredefinedTagGraphAnalytics}
				}
				data = append(data, record)
			}
			return data
		},
		50,
	))
}
