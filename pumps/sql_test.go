package pumps

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"gopkg.in/vmihailenco/msgpack.v2"
)

func getTestPostgresConnectionString() string {
	return os.Getenv("TYK_TEST_POSTGRES")
}

func skipTestIfNoPostgres(t *testing.T) {
	t.Helper()
	if os.Getenv("TYK_TEST_POSTGRES") == "" {
		t.Skip("Skipping test because TYK_TEST_POSTGRES environment variable is not set")
	}
}

func TestSQLInit(t *testing.T) {
	skipTestIfNoPostgres(t)
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "postgres"
	cfg["connection_string"] = getTestPostgresConnectionString()

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		pmp.db.Migrator().DropTable(analytics.SQLTable)
	}()

	assert.NotNil(t, pmp.db)
	assert.Equal(t, "postgres", pmp.db.Dialector.Name())

	// Checking with invalid type
	cfg["type"] = "invalid"
	pmp2 := SQLPump{}
	invalidDialectErr := pmp2.Init(cfg)
	assert.NotNil(t, invalidDialectErr)
}

func TestSQLWriteData(t *testing.T) {
	skipTestIfNoPostgres(t)
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "postgres"
	cfg["connection_string"] = getTestPostgresConnectionString()

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}

	defer func() {
		pmp.db.Migrator().DropTable(analytics.SQLTable)
	}()

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111", OrgID: "123", TimeStamp: time.Now()}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123", OrgID: "1234", TimeStamp: time.Now()}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321", OrgID: "12345", TimeStamp: time.Now()}

	ctx := context.TODO()
	errWrite := pmp.WriteData(ctx, keys)
	if errWrite != nil {
		t.Fatal("SQL Pump couldn't write records with err:", errWrite)
	}

	t.Run("table_records", func(t *testing.T) {
		var dbRecords []analytics.AnalyticsRecord

		table := analytics.SQLTable
		assert.Equal(t, true, pmp.db.Migrator().HasTable(table))
		err := pmp.db.Table(table).Find(&dbRecords).Error
		assert.Nil(t, err)
		assert.Equal(t, 3, len(dbRecords))
	})

	t.Run("table_content", func(t *testing.T) {
		var dbRecords []analytics.AnalyticsRecord

		table := analytics.SQLTable
		assert.Equal(t, true, pmp.db.Migrator().HasTable(table))
		err := pmp.db.Table(table).Find(&dbRecords).Error
		assert.Nil(t, err)

		for i := range keys {
			assert.Equal(t, keys[i].(analytics.AnalyticsRecord).APIID, dbRecords[i].APIID)
			assert.Equal(t, keys[i].(analytics.AnalyticsRecord).OrgID, dbRecords[i].OrgID)
		}
	})
}

func TestSQLWriteDataSharded(t *testing.T) {
	skipTestIfNoPostgres(t)
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "postgres"
	cfg["table_sharding"] = true
	cfg["batch_size"] = 20000
	cfg["connection_string"] = getTestPostgresConnectionString()

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}

	keys := make([]interface{}, 6)
	now := time.Now()
	nowPlus1 := time.Now().AddDate(0, 0, 1)
	nowPlus2 := time.Now().AddDate(0, 0, 2)

	keys[0] = analytics.AnalyticsRecord{APIID: "api111", TimeStamp: now}
	keys[1] = analytics.AnalyticsRecord{APIID: "api112", TimeStamp: now}
	keys[2] = analytics.AnalyticsRecord{APIID: "api113", TimeStamp: now}
	keys[3] = analytics.AnalyticsRecord{APIID: "api114", TimeStamp: nowPlus1}
	keys[4] = analytics.AnalyticsRecord{APIID: "api115", TimeStamp: nowPlus1}
	keys[5] = analytics.AnalyticsRecord{APIID: "api114", TimeStamp: nowPlus2}

	errWrite := pmp.WriteData(context.Background(), keys)
	if errWrite != nil {
		t.Fatal("SQL Pump couldn't write records with err:", errWrite)
	}

	tests := map[string]struct {
		date    time.Time
		RowsLen int
	}{
		"shard_1": {
			date:    now,
			RowsLen: 3,
		},
		"shard_2": {
			date:    nowPlus1,
			RowsLen: 2,
		},
		"shard_3": {
			date:    nowPlus2,
			RowsLen: 1,
		},
	}

	for testName, data := range tests {
		t.Run(testName, func(t *testing.T) {
			var dbRecords []analytics.AnalyticsRecord

			table := analytics.SQLTable + "_" + data.date.Format("20060102")
			defer func(table string) {
				pmp.db.Migrator().DropTable(table)
			}(table)
			assert.Equal(t, true, pmp.db.Migrator().HasTable(table))
			err := pmp.db.Table(table).Find(&dbRecords).Error
			assert.Nil(t, err)
			assert.Equal(t, data.RowsLen, len(dbRecords))
		})
	}

	t.Run("empty_keys", func(t *testing.T) {
		emptyKeys := make([]interface{}, 0)
		errWrite := pmp.WriteData(context.Background(), emptyKeys)
		if errWrite != nil {
			t.Fatal("SQL Pump couldn't write records with err:", errWrite)
		}

		// Check if any table has been created for the empty input case
		tables := []string{
			analytics.SQLTable + "_" + now.Format("20060102"),
			analytics.SQLTable + "_" + nowPlus1.Format("20060102"),
			analytics.SQLTable + "_" + nowPlus2.Format("20060102"),
		}
		for _, table := range tables {
			t.Run("checking_"+table, func(t *testing.T) {
				assert.Equal(t, false, pmp.db.Migrator().HasTable(table)) // No table should exist
			})
		}
	})
}

func TestSQLWriteUptimeData(t *testing.T) {
	skipTestIfNoPostgres(t)
	pmp := SQLPump{IsUptime: true}
	cfg := make(map[string]interface{})
	cfg["type"] = "postgres"
	cfg["connection_string"] = getTestPostgresConnectionString()
	cfg["table_sharding"] = false
	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		pmp.db.Migrator().DropTable(analytics.UptimeSQLTable)
	}()

	now := time.Now()
	nowPlus1 := time.Now().Add(1 * time.Hour)

	tests := map[string]struct {
		Record               analytics.UptimeReportData
		RecordsAmountToWrite int
		RowsLen              int
		HitsPerHour          int
	}{
		"first": {
			Record:               analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          3,
		},
		"second": {
			Record:               analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          6,
		},
		"third": {
			Record:               analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          9,
		},
		"fourth": {
			Record:               analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: nowPlus1},
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
			// encode the records in the way uptime pump consume them
			for i := 0; i < tests[testName].RecordsAmountToWrite; i++ {
				encoded, _ := msgpack.Marshal(tests[testName].Record)
				keys = append(keys, string(encoded))
			}

			pmp.WriteUptimeData(keys)
			table := analytics.UptimeSQLTable
			// check if the table exists
			assert.Equal(t, true, pmp.db.Migrator().HasTable(table))

			dbRecords := []analytics.UptimeReportAggregateSQL{}
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

func TestSQLWriteUptimeDataSharded(t *testing.T) {
	skipTestIfNoPostgres(t)
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "postgres"
	cfg["connection_string"] = getTestPostgresConnectionString()
	cfg["table_sharding"] = true
	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}

	keys := make([]interface{}, 6)
	now := time.Now()
	nowPlus1 := time.Now().AddDate(0, 0, 1)
	nowPlus2 := time.Now().AddDate(0, 0, 2)

	encoded, _ := msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now})
	keys[0] = string(encoded)
	keys[1] = string(encoded)
	keys[2] = string(encoded)
	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: nowPlus1})
	keys[3] = string(encoded)
	keys[4] = string(encoded)
	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: nowPlus2})
	keys[5] = string(encoded)

	pmp.WriteUptimeData(keys)

	tests := map[string]struct {
		date    time.Time
		RowsLen int
	}{
		"records day 1": {
			date:    now,
			RowsLen: 2,
		},
		"records day 2": {
			date:    nowPlus1,
			RowsLen: 2,
		},
		"records day 3": {
			date:    nowPlus2,
			RowsLen: 2,
		},
	}

	for testName, data := range tests {
		t.Run(testName, func(t *testing.T) {
			var dbRecords []analytics.UptimeReportAggregateSQL

			table := analytics.UptimeSQLTable + "_" + data.date.Format("20060102")
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

func TestSQLWriteUptimeDataAggregations(t *testing.T) {
	skipTestIfNoPostgres(t)
	pmp := SQLPump{IsUptime: true}
	cfg := make(map[string]interface{})
	cfg["type"] = "postgres"
	cfg["connection_string"] = getTestPostgresConnectionString()
	cfg["table_sharding"] = false
	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		pmp.db.Migrator().DropTable(analytics.UptimeSQLTable)
	}()

	keys := make([]interface{}, 5)
	now := time.Now()
	encoded, _ := msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", RequestTime: 10, ResponseCode: 200, TimeStamp: now})
	keys[0] = string(encoded)
	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", RequestTime: 10, ResponseCode: 500, TimeStamp: now})
	keys[1] = string(encoded)
	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", RequestTime: 10, ResponseCode: 200, TimeStamp: now})
	keys[2] = string(encoded)
	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", RequestTime: 20, ResponseCode: 200, TimeStamp: now})
	keys[3] = string(encoded)
	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", RequestTime: 20, ResponseCode: 500, TimeStamp: now})
	keys[4] = string(encoded)

	pmp.WriteUptimeData(keys)

	table := analytics.UptimeSQLTable
	dbRecords := []analytics.UptimeReportAggregateSQL{}
	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}

	assert.Len(t, dbRecords, 3)
	assert.Equal(t, "url", dbRecords[0].Dimension)
	assert.Equal(t, "url1", dbRecords[0].DimensionValue)
	assert.Equal(t, 3, dbRecords[0].Code200)
	assert.Equal(t, 2, dbRecords[0].Code500)
	assert.Equal(t, 5, dbRecords[0].Hits)
	assert.Equal(t, 3, dbRecords[0].Success)
	assert.Equal(t, 2, dbRecords[0].ErrorTotal)
	assert.Equal(t, 14.0, dbRecords[0].RequestTime)
	assert.Equal(t, 70.0, dbRecords[0].TotalRequestTime)
}

func TestDecodeRequestAndDecodeResponseSQL(t *testing.T) {
	skipTestIfNoPostgres(t)
	newPump := &SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "postgres"
	cfg["connection_string"] = getTestPostgresConnectionString()
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

func setupSQLPump(t *testing.T, tableName string, useBackground bool) *SQLPump {
	skipTestIfNoPostgres(t)
	t.Helper()
	pmp := &SQLPump{}
	pmp.log = log.WithField("prefix", "sql-pump")
	cfg := map[string]interface{}{
		"type":              "postgres",
		"connection_string": getTestPostgresConnectionString(),
	}

	assert.NoError(t, pmp.Init(cfg))
	if useBackground {
		pmp.backgroundIndexCreated = make(chan bool, 1)
	}
	assert.NoError(t, pmp.ensureTable(tableName))

	return pmp
}

func TestEnsureIndexSQL(t *testing.T) {
	skipTestIfNoPostgres(t)
	//nolint:govet
	tcs := []struct {
		testName             string
		givenTableName       string
		expectedErr          error
		pmpSetupFn           func(t *testing.T, tableName string) *SQLPump
		givenRunInBackground bool
		shouldHaveIndex      bool
	}{
		{
			testName: "index created correctly, not background",
			pmpSetupFn: func(t *testing.T, tableName string) *SQLPump {
				return setupSQLPump(t, tableName, false)
			},
			givenTableName:       "analytics_no_background",
			givenRunInBackground: false,
			expectedErr:          nil,
			shouldHaveIndex:      true,
		},
		{
			testName: "index created correctly, background",
			pmpSetupFn: func(t *testing.T, tableName string) *SQLPump {
				return setupSQLPump(t, tableName, true)
			},
			givenTableName:       "analytics_background",
			givenRunInBackground: true,
			expectedErr:          nil,
			shouldHaveIndex:      true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			pmp := tc.pmpSetupFn(t, tc.givenTableName)
			defer func() {
				err := pmp.db.Migrator().DropTable(tc.givenTableName)
				if err != nil {
					t.Errorf("Failed to drop table: %v", err)
				}
			}()
			assert.NotNil(t, pmp)

			actualErr := pmp.ensureIndex(tc.givenTableName, tc.givenRunInBackground)
			isErrExpected := tc.expectedErr != nil
			didErr := actualErr != nil
			assert.Equal(t, isErrExpected, didErr)

			if isErrExpected {
				assert.Equal(t, tc.expectedErr.Error(), actualErr.Error())
			}

			if actualErr == nil {
				if tc.givenRunInBackground {
					// wait for the background index creation to finish
					<-pmp.backgroundIndexCreated
				}

				indexToUse := indexes[0]
				indexName := pmp.buildIndexName(indexToUse.baseName, tc.givenTableName)
				hasIndex := pmp.db.Table(tc.givenTableName).Migrator().HasIndex(tc.givenTableName, indexName)
				assert.Equal(t, tc.shouldHaveIndex, hasIndex)
			}
		})
	}
}

func TestBuildIndexName(t *testing.T) {
	tests := []struct {
		indexBaseName string
		tableName     string
		expected      string
	}{
		{"idx_responsecode", "users", "users_idx_responsecode"},
		{"idx_apikey", "transactions", "transactions_idx_apikey"},
		{"idx_timestamp", "logs", "logs_idx_timestamp"},
		{"idx_apiid", "api_calls", "api_calls_idx_apiid"},
		{"idx_orgid", "organizations", "organizations_idx_orgid"},
	}

	c := &SQLPump{} // Create an instance of SQLPump.

	for _, tt := range tests {
		t.Run(tt.indexBaseName+"_"+tt.tableName, func(t *testing.T) {
			result := c.buildIndexName(tt.indexBaseName, tt.tableName)
			if result != tt.expected {
				t.Errorf("buildIndexName(%s, %s) = %s; want %s", tt.indexBaseName, tt.tableName, result, tt.expected)
			}
		})
	}
}
