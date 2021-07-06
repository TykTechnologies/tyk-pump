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

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111", ResponseCode: 200, OrgID: "org123"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api111", ResponseCode: 201, OrgID: "org123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api112", ResponseCode: 500, OrgID: "org123"}

	ctx := context.TODO()
	errWrite := pmp.WriteData(ctx, keys)
	if errWrite != nil {
		t.Fatal("SQL Aggregate Pump couldn't write records with err:", errWrite)
	}

	var dbRecords []analytics.SQLAnalyticsRecordAggregate
	if err := pmp.db.Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL Aggregate Pump")
	}

	//for those records, it should have 4 rows - 3 dimensions: apiid-api111, apiid-api1112, errors-500 and total.
	assert.Len(t, dbRecords, 4)

	analizedAPI1, analizedAPI2, analizedErrors500, analizedTotal := false, false, false, false
	for _, record := range dbRecords {
		if record.Dimension == "apiid" && record.DimensionValue == "api111" {
			analizedAPI1 = true
			//assert.Equal(t,2,record.Code2x)
			assert.Equal(t, 2, record.Hits)
			assert.Equal(t, 2, record.Success)
			assert.Equal(t, 0, record.ErrorTotal)
		}
		if record.Dimension == "apiid" && record.DimensionValue == "api112" {
			analizedAPI2 = true
			//assert.Equal(t,0,record.Code2x)
			//assert.Equal(t,1,record.Code5x)
			assert.Equal(t, 1, record.Code500)
			assert.Equal(t, 1, record.Hits)
			assert.Equal(t, 0, record.Success)
			assert.Equal(t, 1, record.ErrorTotal)
		}
		if record.Dimension == "errors" && record.DimensionValue == "500" {
			analizedErrors500 = true
			//assert.Equal(t,0,record.Code2x)
			//assert.Equal(t,1,record.Code5x)
			assert.Equal(t, 1, record.Code500)
			assert.Equal(t, 1, record.Hits)
			assert.Equal(t, 0, record.Success)
			assert.Equal(t, 1, record.ErrorTotal)
		}
		if record.Dimension == "" && record.DimensionValue == "total" {
			analizedTotal = true
			//assert.Equal(t,2,record.Code2x)
			//assert.Equal(t,1,record.Code5x)
			assert.Equal(t, 1, record.Code500)
			assert.Equal(t, 3, record.Hits)
			assert.Equal(t, 2, record.Success)
			assert.Equal(t, 1, record.ErrorTotal)
		}
	}

	assert.Equal(t, true, analizedAPI1 && analizedAPI2 && analizedErrors500 && analizedTotal)
}
