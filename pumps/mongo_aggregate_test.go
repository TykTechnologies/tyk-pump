package pumps

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type dummyObject struct {
	tableName string
}

func (dummyObject) GetObjectID() model.ObjectID {
	return ""
}

func (dummyObject) SetObjectID(model.ObjectID) {}

func (d dummyObject) TableName() string {
	return d.tableName
}

func TestDoAggregatedWritingWithIgnoredAggregations(t *testing.T) {
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfgPump1["ignore_aggregations"] = []string{"apikeys"}
	cfgPump1["use_mixed_collection"] = true
	cfgPump1["store_analytics_per_minute"] = false

	cfgPump2 := make(map[string]interface{})
	cfgPump2["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfgPump2["use_mixed_collection"] = true
	cfgPump2["store_analytics_per_minute"] = false

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
		err := pmp1.store.DropDatabase(context.Background())
		if err != nil {
			t.Errorf("error dropping database: %v", err)
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
			newDummyObject := dummyObject{}
			if tc.IsMixed {
				newDummyObject.tableName = analytics.AgggregateMixedCollectionName
			} else {
				var collErr error
				newDummyObject.tableName, collErr = pmp1.GetCollectionName("123")
				assert.Nil(t, collErr)
			}

			// we build the query using the timestamp as we do in aggregated analytics
			query := model.DBM{
				"orgid":     "123",
				"timestamp": time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), timeNow.Hour(), 0, 0, 0, timeNow.Location()),
			}

			res := analytics.AnalyticsRecordAggregate{}
			// fetch the results
			errFind := pmp1.store.Query(context.Background(), newDummyObject, &res, query)
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

func TestAggregationTime(t *testing.T) {
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfgPump1["ignore_aggregations"] = []string{"apikeys"}
	cfgPump1["use_mixed_collection"] = true

	pmp1 := MongoAggregatePump{}

	timeNow := time.Now()
	keys := make([]interface{}, 1)
	keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1"}

	tests := []struct {
		testName              string
		AggregationTime       int
		WantedNumberOfRecords int
	}{
		{
			testName:              "create record every 60 minutes - 180 minutes hitting the API",
			AggregationTime:       60,
			WantedNumberOfRecords: 3,
		},
		{
			testName:              "create new record every 30 minutes - 120 minutes hitting the API",
			AggregationTime:       30,
			WantedNumberOfRecords: 4,
		},
		{
			testName:              "create new record every 15 minutes - 90 minutes hitting the API",
			AggregationTime:       15,
			WantedNumberOfRecords: 6,
		},
		{
			testName:              "create new record every 7 minutes - 28 minutes hitting the API",
			AggregationTime:       7,
			WantedNumberOfRecords: 4,
		},
		{
			testName:              "create new record every 3 minutes - 24 minutes hitting the API",
			AggregationTime:       3,
			WantedNumberOfRecords: 8,
		},
		{
			testName:              "create new record every minute - 10 minutes hitting the API",
			AggregationTime:       1,
			WantedNumberOfRecords: 10,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			cfgPump1["aggregation_time"] = test.AggregationTime
			errInit1 := pmp1.Init(cfgPump1)
			if errInit1 != nil {
				t.Error(errInit1)
				return
			}

			defer func() {
				// we clean the db after we finish every test case
				defer func() {
					err := pmp1.store.DropDatabase(context.Background())
					if err != nil {
						t.Fatal(err)
					}
				}()
			}()

			ctx := context.TODO()
			for i := 0; i < test.WantedNumberOfRecords; i++ {
				for index := 0; index < test.AggregationTime; index++ {
					errWrite := pmp1.WriteData(ctx, keys)
					if errWrite != nil {
						t.Fatal("Mongo Aggregate Pump couldn't write records with err:", errWrite)
					}
				}
				timeNow = timeNow.Add(time.Minute * time.Duration(test.AggregationTime))
				keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1"}
			}

			query := model.DBM{
				"orgid": "123",
			}

			results := []analytics.AnalyticsRecordAggregate{}
			// fetch the results
			errFind := pmp1.store.Query(context.Background(), &analytics.AnalyticsRecordAggregate{
				Mixed: true,
			}, &results, query)
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

func TestMongoAggregatePump_divideAggregationTime(t *testing.T) {
	tests := []struct {
		name                   string
		currentAggregationTime int
		newAggregationTime     int
	}{
		{
			name:                   "divide 60 minutes (even number)",
			currentAggregationTime: 60,
			newAggregationTime:     30,
		},
		{
			name:                   "divide 15 minutes (odd number)",
			currentAggregationTime: 15,
			newAggregationTime:     7,
		},
		{
			name:                   "divide 1 minute (must return 1)",
			currentAggregationTime: 1,
			newAggregationTime:     1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbConf := &MongoAggregateConf{
				AggregationTime: tt.currentAggregationTime,
			}

			commonPumpConfig := CommonPumpConfig{
				log: logrus.NewEntry(logrus.New()),
			}

			m := &MongoAggregatePump{
				dbConf:           dbConf,
				CommonPumpConfig: commonPumpConfig,
			}
			m.divideAggregationTime()

			assert.Equal(t, tt.newAggregationTime, m.dbConf.AggregationTime)
		})
	}
}

func TestMongoAggregatePump_SelfHealing(t *testing.T) {
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfgPump1["ignore_aggregations"] = []string{"apikeys"}
	cfgPump1["use_mixed_collection"] = true
	cfgPump1["aggregation_time"] = 60
	cfgPump1["enable_aggregate_self_healing"] = true

	pmp1 := MongoAggregatePump{}

	errInit1 := pmp1.Init(cfgPump1)
	if errInit1 != nil {
		t.Error(errInit1)
		return
	}

	defer func() {
		// we clean the db after we finish every test case
		defer func() {
			err := pmp1.store.DropDatabase(context.Background())
			if err != nil {
				t.Fatal(err)
			}
		}()
	}()

	var count int
	var set []interface{}
	for {
		count++
		record := demo.GenerateRandomAnalyticRecord("org123", true)
		set = append(set, record)
		if count == 1000 {
			err := pmp1.WriteData(context.TODO(), set)
			if err != nil {
				// checking if the error is related to the size of the document (standard Mongo)
				contains := strings.Contains(err.Error(), "Size must be between 0 and")
				assert.True(t, contains)
				// If we get an error, is because aggregation time is equal to 1, and self healing can't divide it
				assert.Equal(t, 1, pmp1.dbConf.AggregationTime)

				// checking lastDocumentTimestamp
				ts, err := pmp1.getLastDocumentTimestamp()
				assert.Nil(t, err)
				assert.NotNil(t, ts)
				break
			}
			count = 0
		}
	}
}

func TestMongoAggregatePump_ShouldSelfHeal(t *testing.T) {
	type fields struct {
		dbConf           *MongoAggregateConf
		CommonPumpConfig CommonPumpConfig
	}

	// dbConf - EnableAggregateSelfHealing / AggregationTime / MongoURL / Log

	tests := []struct {
		fields   fields
		inputErr error
		name     string
		want     bool
	}{
		{
			name: "random error",
			fields: fields{
				dbConf: &MongoAggregateConf{
					EnableAggregateSelfHealing: true,
					AggregationTime:            60,
					BaseMongoConf: BaseMongoConf{
						MongoURL: "mongodb://localhost:27017",
					},
				},
				CommonPumpConfig: CommonPumpConfig{
					log: logrus.NewEntry(logrus.New()),
				},
			},
			inputErr: errors.New("random error"),
			want:     false,
		},
		{
			name: "CosmosSizeError error",
			fields: fields{
				dbConf: &MongoAggregateConf{
					EnableAggregateSelfHealing: true,
					AggregationTime:            60,
					BaseMongoConf: BaseMongoConf{
						MongoURL: "mongodb://localhost:27017",
					},
				},
				CommonPumpConfig: CommonPumpConfig{
					log: logrus.NewEntry(logrus.New()),
				},
			},
			inputErr: errors.New("Request size is too large"),
			want:     true,
		},
		{
			name: "StandardMongoSizeError error",
			fields: fields{
				dbConf: &MongoAggregateConf{
					EnableAggregateSelfHealing: true,
					AggregationTime:            60,
					BaseMongoConf: BaseMongoConf{
						MongoURL: "mongodb://localhost:27017",
					},
				},
				CommonPumpConfig: CommonPumpConfig{
					log: logrus.NewEntry(logrus.New()),
				},
			},
			inputErr: errors.New("Size must be between 0 and"),
			want:     true,
		},
		{
			name: "DocDBSizeError error",
			fields: fields{
				dbConf: &MongoAggregateConf{
					EnableAggregateSelfHealing: true,
					AggregationTime:            60,
					BaseMongoConf: BaseMongoConf{
						MongoURL: "mongodb://localhost:27017",
					},
				},
				CommonPumpConfig: CommonPumpConfig{
					log: logrus.NewEntry(logrus.New()),
				},
			},
			inputErr: errors.New("Resulting document after update is larger than"),
			want:     true,
		},
		{
			name: "StandardMongoSizeError error but self healing disabled",
			fields: fields{
				dbConf: &MongoAggregateConf{
					EnableAggregateSelfHealing: false,
					AggregationTime:            60,
					BaseMongoConf: BaseMongoConf{
						MongoURL: "mongodb://localhost:27017",
					},
				},
				CommonPumpConfig: CommonPumpConfig{
					log: logrus.NewEntry(logrus.New()),
				},
			},
			inputErr: errors.New("Size must be between 0 and"),
			want:     false,
		},
		{
			name: "StandardMongoSizeError error but aggregation time is 1",
			fields: fields{
				dbConf: &MongoAggregateConf{
					EnableAggregateSelfHealing: true,
					AggregationTime:            1,
					BaseMongoConf: BaseMongoConf{
						MongoURL: "mongodb://localhost:27017",
					},
				},
				CommonPumpConfig: CommonPumpConfig{
					log: logrus.NewEntry(logrus.New()),
				},
			},
			inputErr: errors.New("Size must be between 0 and"),
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MongoAggregatePump{
				dbConf:           tt.fields.dbConf,
				CommonPumpConfig: tt.fields.CommonPumpConfig,
			}
			if got := m.ShouldSelfHeal(tt.inputErr); got != tt.want {
				t.Errorf("MongoAggregatePump.ShouldSelfHeal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMongoAggregatePump_StoreAnalyticsPerMinute(t *testing.T) {
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = "mongodb://localhost:27017/tyk_analytics"
	cfgPump1["ignore_aggregations"] = []string{"apikeys"}
	cfgPump1["use_mixed_collection"] = true
	cfgPump1["store_analytics_per_minute"] = true
	cfgPump1["aggregation_time"] = 45
	pmp1 := MongoAggregatePump{}

	errInit1 := pmp1.Init(cfgPump1)
	if errInit1 != nil {
		t.Error(errInit1)
		return
	}
	// Checking if the aggregation time is set to 1. Doesn't matter if aggregation_time is equal to 45 or 1, the result should be always 1.
	assert.True(t, pmp1.dbConf.AggregationTime == 1)
}

func TestDecodeRequestAndDecodeResponseMongoAggregate(t *testing.T) {
	newPump := &MongoAggregatePump{}
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

func TestDefaultDriverAggregate(t *testing.T) {
	newPump := &MongoAggregatePump{}
	defaultConf := defaultConf()
	defaultConf.MongoDriverType = ""
	err := newPump.Init(defaultConf)
	assert.Nil(t, err)
	assert.Equal(t, persistent.Mgo, newPump.dbConf.MongoDriverType)
}
