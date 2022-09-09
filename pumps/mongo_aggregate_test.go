package pumps

import (
	"context"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestDoAggregatedWritingWithIgnoredAggregations(t *testing.T) {
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfgPump1["ignore_aggregations"] = []string{"apikeys"}
	cfgPump1["use_mixed_collection"] = true

	cfgPump2 := make(map[string]interface{})
	cfgPump2["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfgPump2["use_mixed_collection"] = true

	pmp1 := MongoAggregatePump{}
	pmp2 := MongoAggregatePump{}

	errInit1 := pmp1.Init(cfgPump1)
	if errInit1 != nil {
		t.Error(errInit1)
		return
	}
	errInit2 := pmp2.Init(cfgPump2)
	if errInit2 != nil {
		t.Error(errInit2)
		return
	}

	timeNow := time.Now()
	keys := make([]interface{}, 2)
	keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1"}

	keys2 := make([]interface{}, 2)
	keys2[0] = analytics.AnalyticsRecord{APIID: "api2", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey2"}
	keys2[1] = analytics.AnalyticsRecord{APIID: "api2", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey2"}

	ctx := context.TODO()
	errWrite := pmp1.WriteData(ctx, keys)
	if errWrite != nil {
		t.Fatal("Mongo Aggregate Pump couldn't write records with err:", errWrite)
	}
	errWrite2 := pmp2.WriteData(ctx, keys2)
	if errWrite2 != nil {
		t.Fatal("Mongo Aggregate Pump couldn't write records with err:", errWrite2)
	}
	errWrite3 := pmp1.WriteData(ctx, keys)
	if errWrite != nil {
		t.Fatal("Mongo Aggregate Pump couldn't write records with err:", errWrite3)
	}

	defer func() {
		//we clean the db after we finish the test
		//we use pmp1 session since it should be the same
		sess := pmp1.dbSession.Copy()
		defer sess.Close()

		if err := sess.DB("").DropDatabase(); err != nil {
			panic(err)
		}
	}()

	tcs := []struct {
		testName string
		IsMixed  bool
	}{
		{
			testName: "not_mixed_collection",
			IsMixed:  false,
		},
		{
			testName: "mixed_collection",
			IsMixed:  true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			collectionName := ""
			if tc.IsMixed {
				collectionName = analytics.AgggregateMixedCollectionName
			} else {
				var collErr error
				collectionName, collErr = pmp1.GetCollectionName("123")
				assert.Nil(t, collErr)
			}

			thisSession := pmp1.dbSession.Copy()
			defer thisSession.Close()

			analyticsCollection := thisSession.DB("").C(collectionName)

			//we build the query using the timestamp as we do in aggregated analytics
			query := bson.M{
				"orgid":     "123",
				"timestamp": time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), timeNow.Hour(), timeNow.Minute(), 0, 0, timeNow.Location()),
			}

			res := analytics.AnalyticsRecordAggregate{}
			// fetch the results
			errFind := analyticsCollection.Find(query).One(&res)
			assert.Nil(t, errFind)

			// double check that the res is not nil
			assert.NotNil(t, res)

			// validate totals
			assert.NotNil(t, res.Total)
			assert.Equal(t, 6, res.Total.Hits)

			// validate that APIKeys (ignored in pmp1) wasn't overriden
			assert.Len(t, res.APIKeys, 1)
			if val, ok := res.APIKeys["apikey2"]; ok {
				assert.NotNil(t, val)
				assert.Equal(t, 2, val.Hits)
			}
		})
	}
}

func TestAnalyticsStoredPerMinute(t *testing.T) {
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfgPump1["ignore_aggregations"] = []string{"apikeys"}
	cfgPump1["use_mixed_collection"] = true

	pmp1 := MongoAggregatePump{}

	timeNow := time.Now()
	keys := make([]interface{}, 1)
	keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1"}

	tests := []struct {
		testName                 string
		AnalyticsStoredPerMinute int
		WantedNumberOfRecords    int
	}{
		{
			testName:                 "create record every 60 minutes - 180 minutes hitting the API",
			AnalyticsStoredPerMinute: 60,
			WantedNumberOfRecords:    3,
		},
		{
			testName:                 "create new record every 30 minutes - 120 minutes hitting the API",
			AnalyticsStoredPerMinute: 30,
			WantedNumberOfRecords:    4,
		},
		{
			testName:                 "create new record every 15 minutes - 90 minutes hitting the API",
			AnalyticsStoredPerMinute: 15,
			WantedNumberOfRecords:    6,
		},
		{
			testName:                 "create new record every 7 minutes - 28 minutes hitting the API",
			AnalyticsStoredPerMinute: 7,
			WantedNumberOfRecords:    4,
		},
		{
			testName:                 "create new record every 3 minutes - 24 minutes hitting the API",
			AnalyticsStoredPerMinute: 3,
			WantedNumberOfRecords:    8,
		},
		{
			testName:                 "create new record every minute - 10 minutes hitting the API",
			AnalyticsStoredPerMinute: 1,
			WantedNumberOfRecords:    10,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			cfgPump1["analytics_stored_per_minute"] = test.AnalyticsStoredPerMinute
			errInit1 := pmp1.Init(cfgPump1)
			if errInit1 != nil {
				t.Error(errInit1)
				return
			}

			defer func() {
				// we clean the db after we finish every test case
				sess := pmp1.dbSession.Copy()
				defer sess.Close()

				if err := sess.DB("").DropDatabase(); err != nil {
					panic(err)
				}
			}()

			ctx := context.TODO()
			for i := 0; i < test.WantedNumberOfRecords; i++ {
				for index := 0; index < test.AnalyticsStoredPerMinute; index++ {
					errWrite := pmp1.WriteData(ctx, keys)
					if errWrite != nil {
						t.Fatal("Mongo Aggregate Pump couldn't write records with err:", errWrite)
					}
				}
				timeNow = timeNow.Add(time.Minute * time.Duration(test.AnalyticsStoredPerMinute))
				keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1"}
			}

			collectionName := analytics.AgggregateMixedCollectionName

			thisSession := pmp1.dbSession.Copy()
			defer thisSession.Close()

			analyticsCollection := thisSession.DB("").C(collectionName)

			// we build the query using the timestamp as we do in aggregated analytics
			query := bson.M{
				"orgid": "123",
			}

			results := []analytics.AnalyticsRecordAggregate{}
			// fetch the results
			errFind := analyticsCollection.Find(query).All(&results)
			assert.Nil(t, errFind)

			// double check that the res is not nil
			assert.NotNil(t, results)

			// checking if we have the correct number of records
			assert.Len(t, results, test.WantedNumberOfRecords)

			// validate totals
			for _, res := range results {
				assert.NotNil(t, res.Total)
			}
		})
	}
}
