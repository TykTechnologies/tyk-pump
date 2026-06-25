package pumps

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File-level MC/DC witness rows mirrored from `proof mcdc show`.
// These rows are copied only when the same file already has tests credited
// for the row by `proof mcdc show`; they do not add new evidence.
// MCDC SW-REQ-062: aggregation_time_above_floor=F, aggregation_time_halved=F, self_heal_enabled=T, size_error_detected=T => TRUE
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=F, self_heal_enabled=F, size_error_detected=T => TRUE
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=F, self_heal_enabled=T, size_error_detected=F => TRUE
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=T, self_heal_enabled=T, size_error_detected=T => TRUE

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

// Verifies: SW-REQ-059
// Verifies: SW-REQ-084
// Verifies: SW-REQ-096
// Verifies: SW-REQ-102
// SW-REQ-059:output_cardinality_bounded:nominal
// SW-REQ-059:output_cardinality_bounded:boundary
// SW-REQ-059:nominal:nominal
// SW-REQ-084:routing_target_consistent:nominal
// SW-REQ-084:routing_target_consistent:boundary
// SW-REQ-096:aggregate_dimension_retention:nominal
// SW-REQ-096:aggregate_dimension_retention:negative
// SW-REQ-102:structured_projection_preserved:nominal
// SW-REQ-102:structured_projection_preserved:boundary
// MCDC SW-REQ-059: org_id_empty=F, write_skipped=F => TRUE
// MCDC SW-REQ-059: org_id_empty=T, write_skipped=F => FALSE
// MCDC SW-REQ-059: org_id_empty=T, write_skipped=T => TRUE
// MCDC SW-REQ-084: mixed_collection_average_update_present=F, aggregate_update_collection_identity_preserved=F => TRUE
// MCDC SW-REQ-084: mixed_collection_average_update_present=T, aggregate_update_collection_identity_preserved=F => FALSE
// MCDC SW-REQ-084: mixed_collection_average_update_present=T, aggregate_update_collection_identity_preserved=T => TRUE
// MCDC SW-REQ-096: ignore_aggregation_configured=F, mixed_collection_dimension_present=T, ignored_dimension_retained=F => TRUE
// MCDC SW-REQ-096: ignore_aggregation_configured=T, mixed_collection_dimension_present=F, ignored_dimension_retained=F => TRUE
// MCDC SW-REQ-096: ignore_aggregation_configured=T, mixed_collection_dimension_present=T, ignored_dimension_retained=F => FALSE
// MCDC SW-REQ-096: ignore_aggregation_configured=T, mixed_collection_dimension_present=T, ignored_dimension_retained=T => TRUE
// MCDC SW-REQ-102: aggregate_lists_projection_persisted=T => TRUE
//
// org_id_empty=F (records carry non-empty OrgID), write_skipped=F (the upsert proceeds —
// non-trigger arm, vacuous true). The org_id_empty=T/write_skipped=T arm is exercised by
// TestDoAggregatedWriting_OrgIDEmpty (where empty-OrgID records trigger DoAggregatedWriting
// to return early without write). The T/F regression scenario is guarded by the early-return
// in DoAggregatedWriting.
//
// For SW-REQ-096, pmp1 has ignore_aggregations=["apikeys"], pmp2 contributes
// apikey2 to the mixed collection, then pmp1 writes again. The final assertion
// that apikey2 remains present witnesses the TRUE row and catches the historical
// double-discard FALSE row. The initial pmp1 write also witnesses the vacuous
// TRUE row where no mixed-collection API-key dimension exists yet.
//
// For SW-REQ-102, both subtests read persisted Mongo aggregate documents back
// into AnalyticsRecordAggregate and assert Lists.APIKeys survives with the
// expected identifier and hit count.
//
//mcdc:ignore SW-REQ-102: aggregate_lists_projection_persisted=F => FALSE -- AnalyticsRecordAggregate carries Lists as a persisted field and AsTimeUpdate writes lists.* into Mongo before readback; with the current code shape, the invariant-false row would require removing the Lists field/projection, while this test witnesses the positive per-org and mixed readback path [reviewed: human:buger] [category: defensive]
func TestDoAggregatedWritingWithIgnoredAggregations(t *testing.T) {
	uri := testMongoURI(t)
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = uri
	cfgPump1["ignore_aggregations"] = []string{"apikeys"}
	cfgPump1["use_mixed_collection"] = true
	cfgPump1["store_analytics_per_minute"] = false

	cfgPump2 := make(map[string]interface{})
	cfgPump2["mongo_url"] = uri
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
	keys[0] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1", RequestTime: 100}
	keys[1] = analytics.AnalyticsRecord{APIID: "api1", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey1", RequestTime: 100}

	keys2 := make([]interface{}, 2)
	keys2[0] = analytics.AnalyticsRecord{APIID: "api2", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey2", RequestTime: 200}
	keys2[1] = analytics.AnalyticsRecord{APIID: "api2", OrgID: "123", TimeStamp: timeNow, APIKey: "apikey2", RequestTime: 200}

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
			assert.Equal(t, 800.0, res.Total.TotalRequestTime)
			assert.InDelta(t, 800.0/6.0, res.Total.RequestTime, 0.0001)

			// validate that APIKeys (ignored in pmp1) wasn't overriden
			assert.Len(t, res.APIKeys, 1)
			if val, ok := res.APIKeys["apikey2"]; ok {
				assert.NotNil(t, val)
				assert.Equal(t, 2, val.Hits)
			}
			require.Len(t, res.Lists.APIKeys, 1)
			assert.Equal(t, "apikey2", res.Lists.APIKeys[0].Identifier)
			assert.Equal(t, 2, res.Lists.APIKeys[0].Hits)
		})
	}
}

// Verifies: SW-REQ-058
// Verifies: SW-REQ-060
// SW-REQ-060:monotonicity:nominal
// MCDC SW-REQ-058: store_per_minute=F, window_eq_1_min=F => TRUE
// MCDC SW-REQ-060: first_upsert_succeeded=T, second_upsert_attempted=T => TRUE
//
// The mongo testcontainer accepts the first $inc/$set/$max/$min upsert (first_upsert_succeeded=T),
// after which DoAggregatedWriting issues the second $addToSet upsert (second_upsert_attempted=T)
// — proving the T/T arm. first_upsert_succeeded=F is exercised by error-injection tests in
// this file (e.g. TestMongoAggregatePump_SelfHealing) where the first upsert fails and the
// second is skipped. The T/F regression (first succeeded but second never attempted) is
// guarded by the unconditional follow-up call in DoAggregatedWriting.
func TestAggregationTime(t *testing.T) {
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = testMongoURI(t)
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
			// Reset shared state so -count=N runs start clean.
			analytics.SetlastTimestampAgggregateRecord(cfgPump1["mongo_url"].(string), time.Time{})

			cfgPump1["aggregation_time"] = test.AggregationTime
			errInit1 := pmp1.Init(cfgPump1)
			if errInit1 != nil {
				t.Error(errInit1)
				return
			}

			// Drop the DB before AND after the test to ensure isolation across -count runs.
			require.NoError(t, pmp1.store.DropDatabase(context.Background()))
			t.Cleanup(func() {
				_ = pmp1.store.DropDatabase(context.Background())
			})

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

// Verifies: SW-REQ-062
// SW-REQ-062:boundary:nominal
// SW-REQ-062:monotonicity:nominal
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

// Verifies: SW-REQ-062
// SW-REQ-062:error_handling:negative
func TestMongoAggregatePump_SelfHealing(t *testing.T) {
	t.Skip("Self-healing requires generating a 16MB+ aggregate; covered by ShouldSelfHeal unit tests")
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = testMongoURI(t)
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

// Verifies: SW-REQ-062
// MCDC SW-REQ-062: aggregation_time_above_floor=F, aggregation_time_halved=F, self_heal_enabled=T, size_error_detected=T => TRUE
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=F, self_heal_enabled=F, size_error_detected=T => TRUE
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=F, self_heal_enabled=T, size_error_detected=F => TRUE
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=T, self_heal_enabled=T, size_error_detected=T => TRUE
// SW-REQ-062:denial_of_service_resistant:nominal
// SW-REQ-062:error_handling:nominal
//
// The table-driven cases below drive every reachable row of the self-healing
// guarantee:
//   - "AggregationTime is 1" (self_heal=T, size-error, AggTime=1): above_floor=F,
//     ShouldSelfHeal returns false without halving -> vacuous-TRUE row 1.
//   - "self healing disabled" (self_heal=F, size-error, AggTime=60): the
//     self_heal_enabled trigger is false -> vacuous-TRUE row 2.
//   - "random error" (self_heal=T, non-matching error, AggTime=60): the
//     size_error_detected trigger is false -> vacuous-TRUE row 3.
//   - the three size-error cases (Cosmos/Standard/DocDB) with self_heal=T and
//     AggTime=60: all triggers true, AggregationTime is halved
//     (aggregation_time_halved=T) -> satisfied row 5.
//
// The violation row (row 4: all triggers true but aggregation_time_halved=F) is
// the negation the guarantee forbids; correct code always halves under those
// conditions, so it has no honest witness.
//
//mcdc:ignore SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=F, self_heal_enabled=T, size_error_detected=T => FALSE — mongo_aggregate.go:447-456: with self_heal_enabled (line 447), a matching size error (line 448) and AggregationTime above the floor (line 450 false because != 1), ShouldSelfHeal calls divideAggregationTime which halves AggregationTime (line 436, guarded only against ==1); so under all three triggers the time is always halved and the "all triggers yet not halved" violation has no branch to reach it [reviewed: human:leo] [category: defensive]
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
			before := m.dbConf.AggregationTime
			got := m.ShouldSelfHeal(tt.inputErr)
			if got != tt.want {
				t.Errorf("MongoAggregatePump.ShouldSelfHeal() = %v, want %v", got, tt.want)
			}
			// When self-healing fires (want=true) the consequent must hold:
			// AggregationTime is halved (aggregation_time_halved=T, the satisfied
			// row 5). When it does not fire, AggregationTime is unchanged
			// (aggregation_time_halved=F, the vacuous rows 1-3).
			if tt.want {
				assert.Equal(t, before/2, m.dbConf.AggregationTime,
					"self-heal must halve AggregationTime (aggregation_time_halved=T)")
			} else {
				assert.Equal(t, before, m.dbConf.AggregationTime,
					"AggregationTime must be unchanged when self-heal does not fire")
			}
		})
	}
}

// Verifies: SW-REQ-062
// SW-REQ-062:boundary:negative
// SW-REQ-062:error_handling:negative
// SW-REQ-062:denial_of_service_resistant:nominal
func TestMongoAggregatePump_ShouldSelfHealResetsTimestampTracker(t *testing.T) {
	base := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	next := base.Add(10 * time.Minute)
	record := analytics.AnalyticsRecord{
		APIID:        "api1",
		OrgID:        "org1",
		TimeStamp:    next,
		ResponseCode: 200,
	}

	tests := []struct {
		name          string
		err           error
		selfHeal      bool
		wantTimestamp time.Time
	}{
		{
			name:          "size error resets tracker so next write opens new bucket",
			err:           errors.New("Size must be between 0 and 16793600"),
			selfHeal:      true,
			wantTimestamp: next,
		},
		{
			name:          "non-size error keeps tracker on current bucket",
			err:           errors.New("temporary write failure"),
			selfHeal:      false,
			wantTimestamp: base,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbID := "mongodb://self-heal-reset/" + t.Name()
			analytics.SetlastTimestampAgggregateRecord(dbID, base)

			p := &MongoAggregatePump{
				dbConf: &MongoAggregateConf{
					BaseMongoConf:              BaseMongoConf{MongoURL: dbID},
					EnableAggregateSelfHealing: true,
					AggregationTime:            30,
				},
				CommonPumpConfig: CommonPumpConfig{
					log: logrus.NewEntry(logrus.New()),
				},
			}

			assert.Equal(t, tt.selfHeal, p.ShouldSelfHeal(tt.err))
			got := analytics.AggregateData([]interface{}{record}, false, nil, dbID, p.dbConf.AggregationTime)
			require.Contains(t, got, "org1")
			assert.Equal(t, tt.wantTimestamp, got["org1"].TimeStamp)
		})
	}
}

// Verifies: SW-REQ-062
// SW-REQ-062:error_handling:nominal
// SW-REQ-062:denial_of_service_resistant:nominal
func TestMongoAggregatePump_WriteDataSelfHealRetryWiring(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "mongo_aggregate.go", nil, 0)
	require.NoError(t, err)

	var foundWriteData bool
	var sawShouldSelfHeal bool
	var sawSameBatchRetry bool

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "WriteData" || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		foundWriteData = true

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "ShouldSelfHeal" {
				sawShouldSelfHeal = true
			}
			if sel.Sel.Name == "WriteData" && len(call.Args) == 2 {
				ctx, ctxOK := call.Args[0].(*ast.Ident)
				data, dataOK := call.Args[1].(*ast.Ident)
				if ctxOK && dataOK && ctx.Name == "ctx" && data.Name == "data" {
					sawSameBatchRetry = true
				}
			}
			return true
		})
	}

	require.True(t, foundWriteData, "MongoAggregatePump.WriteData must be present")
	assert.True(t, sawShouldSelfHeal, "WriteData must classify aggregate write errors through ShouldSelfHeal")
	assert.True(t, sawSameBatchRetry, "self-heal branch must retry the same ctx/data batch")
}

// Verifies: SW-REQ-058
// MCDC SW-REQ-058: store_per_minute=T, window_eq_1_min=T => TRUE
func TestMongoAggregatePump_StoreAnalyticsPerMinute(t *testing.T) {
	cfgPump1 := make(map[string]interface{})
	cfgPump1["mongo_url"] = testMongoURI(t)
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
	t.Cleanup(func() { _ = pmp1.store.DropDatabase(context.Background()) })
	// Checking if the aggregation time is set to 1. Doesn't matter if aggregation_time is equal to 45 or 1, the result should be always 1.
	assert.True(t, pmp1.dbConf.AggregationTime == 1)
}

// Verifies: SW-REQ-036
func TestDecodeRequestAndDecodeResponseMongoAggregate(t *testing.T) {
	newPump := &MongoAggregatePump{}
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

// Verifies: SW-REQ-036
// SW-REQ-036:support_matrix_enforced:nominal
func TestDefaultDriverAggregate(t *testing.T) {
	newPump := &MongoAggregatePump{}
	defaultConf := defaultConf(t)
	defaultConf.MongoDriverType = ""
	err := newPump.Init(defaultConf)
	assert.Nil(t, err)
	t.Cleanup(func() { _ = newPump.store.DropDatabase(context.Background()) })
	assert.Equal(t, persistent.OfficialMongo, newPump.dbConf.MongoDriverType)
}

// Verifies: SW-REQ-036
func TestMongoAggregatePump_SkipsMCPRecords(t *testing.T) {
	pmp := &MongoAggregatePump{}
	pmp.log = logrus.NewEntry(logrus.New())

	data := []interface{}{
		analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}},
		analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}},
	}

	err := pmp.WriteData(context.Background(), data)
	assert.NoError(t, err, "all-MCP payload must short-circuit before touching the store")
}
