package mongo

import (
	"context"
	"strconv"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
)

func TestMongoPump_capCollection_Enabled(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	conf := defaultConf()

	mPump := &Pump{}
	mPump.dbConf = &conf
	mPump.dbConf.CollectionCapEnable = false
	mPump.Log = logger.GetLogger().WithField("prefix", mongoPrefix)

	mPump.connect()

	if ok := mPump.capCollection(); ok {
		t.Error("successfully capped collection when disabled in conf")
	}
}

func TestMongoPumpOmitIndexCreation(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	conf := defaultConf()

	mPump := &Pump{}
	mPump.dbConf = &conf
	record := analytics.AnalyticsRecord{
		OrgID: "test-org",
		APIID: "test-api",
	}
	records := []interface{}{record, record}

	tcs := []struct {
		testName             string
		shouldDropCollection bool
		Indexes              int
		OmitIndexCreation    bool
		dbType               MongoType
	}{
		{
			testName:             "omitting index creation - StandardMongo",
			shouldDropCollection: true,
			Indexes:              1, //1 index corresponding to _id
			OmitIndexCreation:    true,
			dbType:               StandardMongo,
		},
		{
			testName:             "not omitting index creation but mongo collection already exists - StandardMongo",
			shouldDropCollection: false,
			Indexes:              1, //1 index corresponding to _id
			OmitIndexCreation:    false,
			dbType:               StandardMongo,
		},
		{
			testName:             "not omitting index creation but mongo collection doesn't exists - StandardMongo",
			shouldDropCollection: true,
			Indexes:              4, //1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               StandardMongo,
		},
		{
			testName:             "omitting index creation - DocDB",
			shouldDropCollection: true,
			Indexes:              1, //1 index corresponding to _id
			OmitIndexCreation:    true,
			dbType:               AWSDocumentDB,
		},
		{
			testName:             "not omitting index creation but mongo collection already exists - DocDB",
			shouldDropCollection: false,
			Indexes:              4, //1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               AWSDocumentDB,
		},
		{
			testName:             "not omitting index creation but mongo collection doesn't exists - DocDB",
			shouldDropCollection: true,
			Indexes:              4, //1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               AWSDocumentDB,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			mPump.dbConf.OmitIndexCreation = tc.OmitIndexCreation
			mPump.dbConf.MongoDBType = tc.dbType
			mPump.Log = logger.GetLogger().WithField("prefix", mongoPrefix)
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

			if len(indexes) != tc.Indexes {
				t.Errorf("wanted %v index but got %v indexes", tc.Indexes, len(indexes))
			}
		})
	}
}

func TestMongoPump_capCollection_Exists(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	c.InsertDoc()

	conf := defaultConf()

	mPump := &Pump{}
	mPump.dbConf = &conf
	mPump.Log = logger.GetLogger().WithField("prefix", mongoPrefix)

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

	conf := defaultConf()

	mPump := &Pump{}
	mPump.dbConf = &conf
	mPump.Log = logger.GetLogger().WithField("prefix", mongoPrefix)

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

	conf := defaultConf()

	mPump := &Pump{}
	mPump.dbConf = &conf
	mPump.Log = logger.GetLogger().WithField("prefix", mongoPrefix)

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

	conf := defaultConf()

	mPump := &Pump{}
	mPump.dbConf = &conf
	mPump.Log = logger.GetLogger().WithField("prefix", mongoPrefix)

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
	conf := defaultConf()
	conf.MaxInsertBatchSizeBytes = 5120

	numRecords := 100
	// assumed from sizeBytes in AccumulateSet
	const dataSize = 1024
	totalData := dataSize * numRecords

	mPump := &Pump{}
	mPump.dbConf = &conf
	mPump.Log = logger.GetLogger().WithField("prefix", mongoPrefix)

	record := analytics.AnalyticsRecord{}
	data := make([]interface{}, 0)

	for i := 0; i < numRecords; i++ {
		data = append(data, record)
	}

	set := mPump.AccumulateSet(data)

	if len(set) != totalData/conf.MaxInsertBatchSizeBytes {
		t.Errorf("expected accumulator chunks to equal %d, got %d", totalData/conf.MaxInsertBatchSizeBytes, len(set))
	}
}
