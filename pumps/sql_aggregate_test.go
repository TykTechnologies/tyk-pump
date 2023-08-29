package pumps

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSQLAggregateInit(t *testing.T) {
	pmp := SQLAggregatePump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = ""

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Aggregate Pump couldn't be initialized with err: ", err)
	}
	defer func(table string) {
		pmp.db.Migrator().DropTable(analytics.AggregateSQLTable)
	}(table)

	assert.NotNil(t, pmp.db)
	assert.Equal(t, "sqlite", pmp.db.Dialector.Name())
	assert.Equal(t, true, pmp.db.Migrator().HasTable(analytics.AggregateSQLTable))

	assert.Equal(t, true, pmp.db.Migrator().HasIndex(analytics.AggregateSQLTable, newAggregatedIndexName))

	// Checking with invalid type
	cfg["type"] = "invalid"
	pmp2 := SQLAggregatePump{}
	invalidDialectErr := pmp2.Init(cfg)
	assert.NotNil(t, invalidDialectErr)
	// TODO check how to test postgres connection - it's going to requiere to have some postgres up
}

func TestSQLAggregateWriteData_Sharded(t *testing.T) {
	pmp := SQLAggregatePump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["table_sharding"] = true

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump Aggregate couldn't be initialized with err: ", err)
	}

	// wait until the index is created for sqlite to avoid locking

	keys := make([]interface{}, 8)
	now := time.Now()
	nowPlus1 := time.Now().AddDate(0, 0, 1)
	nowPlus2 := time.Now().AddDate(0, 0, 2)

	keys[0] = analytics.AnalyticsRecord{OrgID: "1", ResponseCode: http.StatusInternalServerError, TimeStamp: now}
	keys[1] = analytics.AnalyticsRecord{OrgID: "1", ResponseCode: http.StatusBadRequest, TimeStamp: now}
	keys[2] = analytics.AnalyticsRecord{OrgID: "1", ResponseCode: http.StatusUnavailableForLegalReasons, TimeStamp: now}
	keys[3] = analytics.AnalyticsRecord{OrgID: "1", ResponseCode: http.StatusOK, TimeStamp: now}

	keys[4] = analytics.AnalyticsRecord{OrgID: "1", ResponseCode: http.StatusNotFound, APIID: "1", TimeStamp: nowPlus1}
	keys[5] = analytics.AnalyticsRecord{OrgID: "1", ResponseCode: http.StatusNotFound, APIID: "1", TimeStamp: nowPlus1}
	keys[6] = analytics.AnalyticsRecord{OrgID: "1", ResponseCode: http.StatusUnauthorized, APIID: "2", TimeStamp: nowPlus1}

	keys[7] = analytics.AnalyticsRecord{OrgID: "1", ResponseCode: http.StatusUnauthorized, APIID: "2", TimeStamp: nowPlus2}

	pmp.WriteData(context.TODO(), keys)

	tests := map[string]struct {
		date    time.Time
		RowsLen int
	}{
		"records day 1": {
			date:    now,
			RowsLen: 4, // 3 (from StatusInternalServerError, StatusBadRequest and StatusUnavailableForLegalReasons) + 1 (from total)

		},
		"records day 2": {
			date:    nowPlus1,
			RowsLen: 5, // 2(from apiid) + 2 (from StatusNotFound and StatusUnauthorized) + 1 (from total)
		},
		"records day 3": {
			date:    nowPlus2,
			RowsLen: 3, // 1(from apiid) + 1 (from StatusUnauthorized) + 1 (from total)
		},
	}
	for testName, data := range tests {
		t.Run(testName, func(t *testing.T) {
			var dbRecords []analytics.SQLAnalyticsRecordAggregate

			table := analytics.AggregateSQLTable + "_" + data.date.Format("20060102")
			defer func(table string) {
				pmp.db.Migrator().DropTable(table)
			}(table)
			assert.Equal(t, true, pmp.db.Migrator().HasTable(table))
			err := pmp.db.Table(table).Find(&dbRecords).Error
			assert.Nil(t, err)
			assert.Equal(t, data.RowsLen, len(dbRecords))
		})
	}
}

func TestSQLAggregateWriteData(t *testing.T) {
	pmp := &SQLAggregatePump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["batch_size"] = 2000

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump Aggregate couldn't be initialized with err: ", err)
	}
	defer func(table string) {
		pmp.db.Migrator().DropTable(analytics.AggregateSQLTable)
	}(table)

	err = pmp.ensureIndex(analytics.AggregateSQLTable, false)
	assert.Nil(t, err)

	now := time.Now()
	nowPlus1 := time.Now().Add(1 * time.Hour)

	tests := map[string]struct {
		Record               analytics.AnalyticsRecord
		RecordsAmountToWrite int
		RowsLen              int
		HitsPerHour          int
	}{
		"first": {
			Record:               analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          3,
		},
		"second": {
			Record:               analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          6,
		},
		"third": {
			Record:               analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          9,
		},
		"fourth": {
			Record:               analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", TimeStamp: nowPlus1},
			RecordsAmountToWrite: 3,
			RowsLen:              4,
			HitsPerHour:          3, // since we're going to write in a new hour, it should mean a different aggregation.
		},
	}

	testNames := []string{"first", "second", "third", "fourth"}

	for _, testName := range testNames {
		t.Run(testName, func(t *testing.T) {
			pmp := pmp
			keys := []interface{}{}

			for i := 0; i < tests[testName].RecordsAmountToWrite; i++ {
				keys = append(keys, tests[testName].Record)
			}

			pmp.WriteData(context.TODO(), keys)
			table := analytics.AggregateSQLTable
			// check if the table exists
			assert.Equal(t, true, pmp.db.Migrator().HasTable(table))

			dbRecords := []analytics.SQLAnalyticsRecordAggregate{}
			if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
				t.Fatal("Error getting analytics records from SQL")
			}

			// check amount of rows in the table
			assert.Equal(t, tests[testName].RowsLen, len(dbRecords))

			// iterate over the records and check total of hits
			for _, dbRecord := range dbRecords {
				if dbRecord.TimeStamp == tests[testName].Record.TimeStamp.Unix() && dbRecord.DimensionValue == "total" {
					assert.Equal(t, tests[testName].HitsPerHour, dbRecord.Hits)
					break
				}
			}
		})
	}
}

func TestSQLAggregateWriteDataValues(t *testing.T) {
	table := analytics.AggregateSQLTable
	now := time.Date(2019, 1, 1, 0, 0, 0, 0, time.Local)
	nowPlus10 := now.Add(10 * time.Minute)

	tcs := []struct {
		testName  string
		assertion func(*testing.T, []analytics.SQLAnalyticsRecordAggregate)
		records   [][]interface{}
	}{
		{
			testName: "only one writing",
			records: [][]interface{}{
				{
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 500, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 20, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 20, Upstream: 20}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 20, ResponseCode: 500, TimeStamp: now, Latency: analytics.Latency{Total: 20, Upstream: 30}},
				},
			},
			assertion: func(t *testing.T, dbRecords []analytics.SQLAnalyticsRecordAggregate) {
				assert.Equal(t, 3, len(dbRecords))
				assert.Equal(t, "apiid", dbRecords[0].Dimension)
				assert.Equal(t, "api1", dbRecords[0].DimensionValue)
				assert.Equal(t, 2, dbRecords[0].Code500)
				assert.Equal(t, 5, dbRecords[0].Hits)
				assert.Equal(t, 3, dbRecords[0].Success)
				assert.Equal(t, 2, dbRecords[0].ErrorTotal)
				assert.Equal(t, 14.0, dbRecords[0].RequestTime)
				assert.Equal(t, 70.0, dbRecords[0].TotalRequestTime)
				assert.Equal(t, float64(14), dbRecords[0].Latency)
				assert.Equal(t, int64(70), dbRecords[0].TotalLatency)
				assert.Equal(t, float64(16), dbRecords[0].UpstreamLatency)
				assert.Equal(t, int64(80), dbRecords[0].TotalUpstreamLatency)
				assert.Equal(t, int64(20), dbRecords[0].MaxLatency)
				assert.Equal(t, int64(10), dbRecords[0].MinUpstreamLatency)
				// checking if it has total dimension
				assert.Equal(t, "total", dbRecords[2].DimensionValue)
				assert.Equal(t, 5, dbRecords[2].Hits)
				assert.Equal(t, now.Format(time.RFC3339), dbRecords[0].LastTime.Format(time.RFC3339))
			},
		},
		{
			testName: "two writings - on conflict",
			records: [][]interface{}{
				{
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 500, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 20, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 20, Upstream: 20}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 20, ResponseCode: 500, TimeStamp: now, Latency: analytics.Latency{Total: 20, Upstream: 30}},
				},
				{
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 200, TimeStamp: nowPlus10, Latency: analytics.Latency{Total: 10, Upstream: 5}},
					analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 500, TimeStamp: nowPlus10, Latency: analytics.Latency{Total: 30, Upstream: 10}},
				},
			},
			assertion: func(t *testing.T, dbRecords []analytics.SQLAnalyticsRecordAggregate) {
				assert.Equal(t, 3, len(dbRecords))
				assert.Equal(t, "apiid", dbRecords[0].Dimension)
				assert.Equal(t, "api1", dbRecords[0].DimensionValue)
				assert.Equal(t, 3, dbRecords[0].Code500)
				assert.Equal(t, 7, dbRecords[0].Hits)
				assert.Equal(t, 4, dbRecords[0].Success)
				assert.Equal(t, 3, dbRecords[0].ErrorTotal)
				assert.Equal(t, 12.857142857142858, dbRecords[0].RequestTime)
				assert.Equal(t, 90.0, dbRecords[0].TotalRequestTime)
				assert.Equal(t, 15.714285714285714, dbRecords[0].Latency)
				assert.Equal(t, int64(110), dbRecords[0].TotalLatency)
				assert.Equal(t, 13.571428571428571, dbRecords[0].UpstreamLatency)
				assert.Equal(t, int64(95), dbRecords[0].TotalUpstreamLatency)
				assert.Equal(t, int64(30), dbRecords[0].MaxLatency)
				assert.Equal(t, int64(5), dbRecords[0].MinUpstreamLatency)
				assert.Equal(t, nowPlus10.Minute(), dbRecords[0].LastTime.Minute(), "last time incorrect")
				assert.Equal(t, "total", dbRecords[2].DimensionValue)
				assert.Equal(t, 7, dbRecords[2].Hits)
				assert.Equal(t, nowPlus10.Format("2006-01-02 15:04:05-07:00"), dbRecords[0].LastTime.Format("2006-01-02 15:04:05-07:00"))
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			// Configure and Initialise pump first
			dbRecords := []analytics.SQLAnalyticsRecordAggregate{}

			pmp := &SQLAggregatePump{}
			cfg := make(map[string]interface{})
			cfg["type"] = "sqlite"
			cfg["batch_size"] = 1

			err := pmp.Init(cfg)
			if err != nil {
				t.Fatal("SQL Pump Aggregate couldn't be initialized with err: ", err)
			}
			defer func(pmp *SQLAggregatePump) {
				err := pmp.db.Migrator().DropTable(analytics.AggregateSQLTable)
				if err != nil {
					t.Error(err)
				}
			}(pmp)
			// Write the analytics records
			for i := range tc.records {
				err = pmp.WriteData(context.TODO(), tc.records[i])
				if err != nil {
					t.Fatal(err.Error())
				}
			}

			// Fetch the analytics records from the db
			if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
				t.Fatal("Error getting analytics records from SQL")
				return
			}

			// Validate
			tc.assertion(t, dbRecords)
		})
	}
}

func TestDecodeRequestAndDecodeResponseSQLAggregate(t *testing.T) {
	newPump := &SQLAggregatePump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = ""
	cfg["table_sharding"] = true
	err := newPump.Init(cfg)
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

func TestEnsureIndex(t *testing.T) {
	//nolint:govet
	tcs := []struct {
		testName             string
		givenTableName       string
		expectedErr          error
		pmpSetupFn           func(tableName string) *SQLAggregatePump
		givenRunInBackground bool
		shouldHaveIndex      bool
	}{
		{
			testName: "index created correctly, not background",
			pmpSetupFn: func(tableName string) *SQLAggregatePump {
				pmp := &SQLAggregatePump{}
				cfg := &SQLAggregatePumpConf{}
				cfg.Type = "sqlite"
				cfg.ConnectionString = ""
				pmp.SQLConf = cfg

				pmp.log = log.WithField("prefix", "sql-aggregate-pump")
				dialect, errDialect := Dialect(&pmp.SQLConf.SQLConf)
				if errDialect != nil {
					return nil
				}
				db, err := gorm.Open(dialect, &gorm.Config{
					AutoEmbedd:  true,
					UseJSONTags: true,
					Logger:      logger.Default.LogMode(logger.Info),
				})
				if err != nil {
					return nil
				}
				pmp.db = db

				if err := pmp.ensureTable(tableName); err != nil {
					return nil
				}

				return pmp
			},
			givenTableName:       "test",
			givenRunInBackground: false,
			expectedErr:          nil,
			shouldHaveIndex:      true,
		},
		{
			testName: "index created correctly, background",
			pmpSetupFn: func(tableName string) *SQLAggregatePump {
				pmp := &SQLAggregatePump{}
				cfg := &SQLAggregatePumpConf{}
				cfg.Type = "sqlite"
				cfg.ConnectionString = ""
				pmp.SQLConf = cfg

				pmp.log = log.WithField("prefix", "sql-aggregate-pump")
				dialect, errDialect := Dialect(&pmp.SQLConf.SQLConf)
				if errDialect != nil {
					return nil
				}
				db, err := gorm.Open(dialect, &gorm.Config{
					AutoEmbedd:  true,
					UseJSONTags: true,
					Logger:      logger.Default.LogMode(logger.Info),
				})
				if err != nil {
					return nil
				}
				pmp.db = db

				pmp.backgroundIndexCreated = make(chan bool, 1)

				if err := pmp.ensureTable(tableName); err != nil {
					return nil
				}

				return pmp
			},
			givenTableName:       "test2",
			givenRunInBackground: true,
			expectedErr:          nil,
			shouldHaveIndex:      true,
		},
		{
			testName: "index created on non existing table, not background",
			pmpSetupFn: func(tableName string) *SQLAggregatePump {
				pmp := &SQLAggregatePump{}
				cfg := &SQLAggregatePumpConf{}
				cfg.Type = "sqlite"
				cfg.ConnectionString = ""
				pmp.SQLConf = cfg

				pmp.log = log.WithField("prefix", "sql-aggregate-pump")
				dialect, errDialect := Dialect(&pmp.SQLConf.SQLConf)
				if errDialect != nil {
					return nil
				}
				db, err := gorm.Open(dialect, &gorm.Config{
					AutoEmbedd:  true,
					UseJSONTags: true,
					Logger:      logger.Default.LogMode(logger.Info),
				})
				if err != nil {
					return nil
				}
				pmp.db = db

				return pmp
			},
			givenTableName:       "test3",
			givenRunInBackground: false,
			expectedErr:          errors.New("no such table: main.test3"),
			shouldHaveIndex:      false,
		},
		{
			testName: "omit_index_creation enabled",
			pmpSetupFn: func(tableName string) *SQLAggregatePump {
				pmp := &SQLAggregatePump{}
				cfg := &SQLAggregatePumpConf{}
				cfg.Type = "sqlite"
				cfg.ConnectionString = ""
				cfg.OmitIndexCreation = true
				pmp.SQLConf = cfg

				pmp.log = log.WithField("prefix", "sql-aggregate-pump")
				dialect, errDialect := Dialect(&pmp.SQLConf.SQLConf)
				if errDialect != nil {
					return nil
				}
				db, err := gorm.Open(dialect, &gorm.Config{
					AutoEmbedd:  true,
					UseJSONTags: true,
					Logger:      logger.Default.LogMode(logger.Info),
				})
				if err != nil {
					return nil
				}
				pmp.db = db

				if err := pmp.ensureTable(tableName); err != nil {
					return nil
				}
				return pmp
			},
			givenTableName:       "test3",
			givenRunInBackground: false,
			expectedErr:          nil,
			shouldHaveIndex:      false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			pmp := tc.pmpSetupFn(tc.givenTableName)
			assert.NotNil(t, pmp)

			actualErr := pmp.ensureIndex(tc.givenTableName, tc.givenRunInBackground)

			if actualErr == nil {
				if tc.givenRunInBackground {
					// wait for the background index creation to finish
					<-pmp.backgroundIndexCreated
				} else {
					hasIndex := pmp.db.Table(tc.givenTableName).Migrator().HasIndex(tc.givenTableName, newAggregatedIndexName)
					assert.Equal(t, tc.shouldHaveIndex, hasIndex)
				}
			} else {
				assert.Equal(t, tc.expectedErr.Error(), actualErr.Error())
			}
		})
	}
}
