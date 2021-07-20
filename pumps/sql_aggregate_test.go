package pumps

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
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

	//Checking with invalid type
	cfg["type"] = "invalid"
	pmp2 := SQLAggregatePump{}
	invalidDialectErr := pmp2.Init(cfg)
	assert.NotNil(t, invalidDialectErr)
	//TODO check how to test postgres connection - it's going to requiere to have some postgres up

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
	pmp := SQLAggregatePump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump Aggregate couldn't be initialized with err: ", err)
	}
	defer func(table string) {
		pmp.db.Migrator().DropTable(analytics.AggregateSQLTable)
	}(table)

	now := time.Now()
	nowPlus1 := time.Now().Add(1 * time.Hour)

	tests := map[string]struct {
		Record               analytics.AnalyticsRecord
		RecordsAmountToWrite int
		RowsLen              int
		HitsPerHour          int
	}{
		"first iteration": {
			Record:               analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          3,
		},
		"second iteration": {
			Record:               analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          6,
		},
		"third iteration": {
			Record:               analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", TimeStamp: now},
			RecordsAmountToWrite: 3,
			RowsLen:              2,
			HitsPerHour:          9,
		},
		"fourth iteration": {
			Record:               analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", TimeStamp: nowPlus1},
			RecordsAmountToWrite: 3,
			RowsLen:              4,
			HitsPerHour:          3, //since we're going to write in a new hour, it should mean a different aggregation.
		},
	}

	for testName, testValue := range tests {
		t.Run(testName, func(t *testing.T) {
			pmp := pmp
			keys := []interface{}{}

			for i := 0; i < testValue.RecordsAmountToWrite; i++ {
				keys = append(keys, testValue.Record)
			}

			pmp.WriteData(context.TODO(), keys)
			table := analytics.AggregateSQLTable
			//check if the table exists
			assert.Equal(t, true, pmp.db.Migrator().HasTable(table))

			dbRecords := []analytics.SQLAnalyticsRecordAggregate{}
			if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
				t.Fatal("Error getting analytics records from SQL")
			}

			//check amount of rows in the table
			assert.Equal(t, testValue.RowsLen, len(dbRecords))

			//iterate over the records and check total of hits
			for _, dbRecord := range dbRecords {
				if dbRecord.TimeStamp == testValue.Record.TimeStamp.Unix() && dbRecord.DimensionValue == "total" {
					assert.Equal(t, testValue.HitsPerHour, dbRecord.Hits)
					break
				}
			}

		})
	}
}

func TestSQLAggregateWriteDataValues(t *testing.T) {
	pmp := SQLAggregatePump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump Aggregate couldn't be initialized with err: ", err)
	}
	defer func(table string) {
		//pmp.db.Migrator().DropTable(analytics.AggregateSQLTable)
	}(table)

	now := time.Now()
	keys := make([]interface{}, 5)
	keys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}}
	keys[1] = analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 500, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}}
	keys[2] = analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 10}}
	keys[3] = analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 20, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 20, Upstream: 20}}
	keys[4] = analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 20, ResponseCode: 500, TimeStamp: now, Latency: analytics.Latency{Total: 20, Upstream: 30}}

	err = pmp.WriteData(context.TODO(), keys)
	if err != nil {
		t.Fatal(err.Error())
	}
	table := analytics.AggregateSQLTable
	dbRecords := []analytics.SQLAnalyticsRecordAggregate{}
	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
		return
	}

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

	//We check again to validate the ON CONFLICT CLAUSES
	newKeys := make([]interface{}, 2)
	newKeys[0] = analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 200, TimeStamp: now, Latency: analytics.Latency{Total: 10, Upstream: 5}}
	newKeys[1] = analytics.AnalyticsRecord{OrgID: "1", APIID: "api1", RequestTime: 10, ResponseCode: 500, TimeStamp: now, Latency: analytics.Latency{Total: 30, Upstream: 10}}

	err = pmp.WriteData(context.TODO(), newKeys)
	if err != nil {
		t.Fatal(err.Error())
	}
	dbRecords = []analytics.SQLAnalyticsRecordAggregate{}
	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}

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

}
