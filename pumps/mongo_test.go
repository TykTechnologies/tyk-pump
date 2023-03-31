package pumps

import (
	"context"
	"encoding/base64"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TykTechnologies/storage/persistent/id"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

func newPump() Pump {
	return (&MongoPump{}).New()
}

func TestMongoPump_capCollection_Enabled(t *testing.T) {
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
	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)

	mPump.dbConf = &conf
	record := analytics.AnalyticsRecord{
		OrgID: "test-org",
		APIID: "test-api",
	}
	records := []interface{}{record, record}
	dbObject := createDBObject(conf.CollectionName)
	mPump.connect()
	defer func() {
		// we clean the db after we finish every test case
		defer func() {
			err := mPump.store.Drop(context.Background(), dbObject)
			if err != nil {
				t.Fatal(err)
			}
		}()
	}()

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
			ExpectedIndexes:      1, // 1 index corresponding to _id
			OmitIndexCreation:    true,
			dbType:               StandardMongo,
		},
		{
			testName:             "not omitting index creation but mongo collection already exists - StandardMongo",
			shouldDropCollection: false,
			ExpectedIndexes:      1, // 1 index corresponding to _id
			OmitIndexCreation:    false,
			dbType:               StandardMongo,
		},
		{
			testName:             "not omitting index creation but mongo collection doesn't exists - StandardMongo",
			shouldDropCollection: true,
			ExpectedIndexes:      4, // 1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               StandardMongo,
		},
		{
			testName:             "omitting index creation - DocDB",
			shouldDropCollection: true,
			ExpectedIndexes:      1, // 1 index corresponding to _id
			OmitIndexCreation:    true,
			dbType:               AWSDocumentDB,
		},
		{
			testName:             "not omitting index creation but mongo collection already exists - DocDB",
			shouldDropCollection: false,
			ExpectedIndexes:      4, // 1 index corresponding to _id + 3 from tyk
			OmitIndexCreation:    false,
			dbType:               AWSDocumentDB,
		},
		{
			testName:             "not omitting index creation but mongo collection doesn't exists - DocDB",
			shouldDropCollection: true,
			ExpectedIndexes:      4, // 1 index corresponding to _id + 3 from tyk
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
			defer func() {
				err := mPump.store.CleanIndexes(context.Background(), dbObject)
				if err != nil {
					t.Fatal(err)
				}
			}()

			// Drop collection if it exists
			if tc.shouldDropCollection {
				if HasTable(t, mPump, dbObject) {
					err := mPump.store.Drop(context.Background(), dbObject)
					if err != nil {
						t.Error("there shouldn't be an error dropping database", err)
					}
				}
			} else {
				// Create collection if it doesn't exist
				CreateCollectionIfNeeded(t, mPump, dbObject)
			}

			if err := mPump.ensureIndexes(dbObject.TableName()); err != nil {
				t.Error("there shouldn't be an error ensuring indexes", err)
			}

			err := mPump.WriteData(context.Background(), records)
			if err != nil {
				t.Error("there shouldn't be an error writing data", err)
			}
			// Before getting indexes, we must ensure that the collection exists to avoid an unexpected error
			CreateCollectionIfNeeded(t, mPump, dbObject)

			indexes, errIndexes := mPump.store.GetIndexes(context.Background(), dbObject)
			if errIndexes != nil {
				t.Error("error getting indexes:", errIndexes)
			}

			if len(indexes) != tc.ExpectedIndexes {
				t.Errorf("wanted %v index but got %v indexes", tc.ExpectedIndexes, len(indexes))
			}
		})
	}
}

func CreateCollectionIfNeeded(t *testing.T, mPump *MongoPump, dbObject id.DBObject) {
	t.Helper()
	if !HasTable(t, mPump, dbObject) {
		err := mPump.store.Migrate(context.Background(), []id.DBObject{dbObject})
		if err != nil {
			t.Error("there shouldn't be an error migrating database", err)
		}
	}
}

func HasTable(t *testing.T, mPump *MongoPump, dbObject id.DBObject) bool {
	t.Helper()
	hasTable, err := mPump.store.HasTable(context.Background(), dbObject.TableName())
	if err != nil {
		t.Error("there shouldn't be an error checking if table exists", err)
	}

	return hasTable
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

			set := mPump.AccumulateSet(data, false)

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

func TestMongoPump_AccumulateSetIgnoreDocSize(t *testing.T) {
	bloat := base64.StdEncoding.EncodeToString(make([]byte, 2048))
	pump := newPump()
	conf := defaultConf()
	conf.MaxDocumentSizeBytes = 2048
	mPump, ok := pump.(*MongoPump)
	assert.True(t, ok)
	mPump.dbConf = &conf
	mPump.log = log.WithField("prefix", mongoPrefix)

	dataSet := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		record := analytics.AnalyticsRecord{}
		if i%2 == 0 {
			record.Tags = []string{analytics.PredefinedTagGraphAnalytics}
			record.RawRequest = bloat
			record.RawResponse = bloat
			record.ApiSchema = bloat
		}
		dataSet[i] = record
	}

	accumulated := mPump.AccumulateSet(dataSet, true)
	for _, x := range accumulated {
		for _, y := range x {
			rec, ok := y.(*analytics.AnalyticsRecord)
			assert.True(t, ok)
			if rec.IsGraphRecord() {
				assert.NotEmpty(t, rec.RawRequest)
				assert.NotEmpty(t, rec.RawResponse)
			}
		}
	}
}

func TestGetBlurredURL(t *testing.T) {
	tcs := []struct {
		testName           string
		givenURL           string
		expectedBlurredURL string
	}{
		{
			testName:           "mongodb:username:password@",
			givenURL:           "mongodb:username:password@localhost:27107/mydatabasename",
			expectedBlurredURL: "***:***@localhost:27107/mydatabasename",
		},
		{
			testName:           "no user or password",
			givenURL:           "mongodb://localhost:27017/test",
			expectedBlurredURL: "mongodb://localhost:27017/test",
		},
		{
			testName:           "no mongodb:// but user and password",
			givenURL:           "mongodb:username:password@localhost:27107/mydatabasename",
			expectedBlurredURL: "***:***@localhost:27107/mydatabasename",
		},

		{
			testName:           "complex url",
			givenURL:           "mongodb://user:pass@mongo-HZNP-0.j.com,mongo-HZNP-1.j.com,mongo-HZNP-2.j.com/tyk?replicaSet=RS1",
			expectedBlurredURL: "***:***@mongo-HZNP-0.j.com,mongo-HZNP-1.j.com,mongo-HZNP-2.j.com/tyk?replicaSet=RS1",
		},
		{
			testName:           "complex password username",
			givenURL:           "mongodb://myDBReader:D1fficultP%40ssw0rd@mongodb0.example.com:27017/?authSource=admin",
			expectedBlurredURL: "***:***@mongodb0.example.com:27017/?authSource=admin",
		},

		{
			testName:           "cluster",
			givenURL:           "mongodb://mongos0.example.com:27017,mongos1.example.com:27017,mongos2.example.com:27017",
			expectedBlurredURL: "mongodb://mongos0.example.com:27017,mongos1.example.com:27017,mongos2.example.com:27017",
		},

		{
			testName:           "cluster+complex password username",
			givenURL:           "mongodb://us3r-n4m!:p4_ssw:0rd@mongo-HZNP-0.j.com,mongo-HZNP-1.j.com,mongo-HZNP-2.j.com/tyk?replicaSet=RS1",
			expectedBlurredURL: "***:***@mongo-HZNP-0.j.com,mongo-HZNP-1.j.com,mongo-HZNP-2.j.com/tyk?replicaSet=RS1",
		},
		{
			testName:           "CosmoDB",
			givenURL:           "mongodb://contoso123:0Fc3IolnL12312asdfawejunASDFasdfYXX2t8a97kghVcUzcDv98hawelufhawefafnoQRGwNj2nMPL1Y9qsIr9Srdw==@contoso123.documents.azure.com:10255/mydatabase?ssl=true",
			expectedBlurredURL: "***:***@contoso123.documents.azure.com:10255/mydatabase?ssl=true",
		},
		{
			testName:           "DocDB",
			givenURL:           "mongodb://UserName:Password@sample-cluster-instance.cluster-corlsfccjozr.us-east-1.docdb.amazonaws.com:27017?replicaSet=rs0&ssl_ca_certs=rds-combined-ca-bundle.pem",
			expectedBlurredURL: "***:***@sample-cluster-instance.cluster-corlsfccjozr.us-east-1.docdb.amazonaws.com:27017?replicaSet=rs0&ssl_ca_certs=rds-combined-ca-bundle.pem",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			conf := BaseMongoConf{
				MongoURL: tc.givenURL,
			}
			actualBlurredURL := conf.GetBlurredURL()
			assert.Equal(t, tc.expectedBlurredURL, actualBlurredURL)
		})
	}
}

// func TestMongoPump_SessionConsistency(t *testing.T) {
// 	pump := newPump()
// 	conf := defaultConf()

// 	mPump, ok := pump.(*MongoPump)
// 	assert.True(t, ok)
// 	mPump.dbConf = &conf

// 	tests := []struct {
// 		testName            string
// 		sessionConsistency  string
// 		expectedSessionMode mgo.Mode
// 	}{
// 		{
// 			testName:            "should set session mode to strong",
// 			sessionConsistency:  "strong",
// 			expectedSessionMode: mgo.Strong,
// 		},
// 		{
// 			testName:            "should set session mode to monotonic",
// 			sessionConsistency:  "monotonic",
// 			expectedSessionMode: mgo.Monotonic,
// 		},
// 		{
// 			testName:            "should set session mode to eventual",
// 			sessionConsistency:  "eventual",
// 			expectedSessionMode: mgo.Eventual,
// 		},
// 		{
// 			testName:            "should set session mode to strong by default",
// 			sessionConsistency:  "",
// 			expectedSessionMode: mgo.Strong,
// 		},
// 	}

// 	for _, test := range tests {
// 		t.Run(test.testName, func(t *testing.T) {
// 			mPump.dbConf.MongoSessionConsistency = test.sessionConsistency
// 			mPump.connect()
// 			assert.Equal(t, test.expectedSessionMode, mPump.dbSession.Mode())
// 		})
// 	}
// }
