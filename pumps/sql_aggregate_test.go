package pumps

import (
	"context"
	"os"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

func TestSQLAggregateInit(t *testing.T) {
	pmp := SQLAggregatePump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = "pmp_test.db"

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Aggregate Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		os.Remove("pmp_test.db")
	}()

	assert.NotNil(t, pmp.db)
	assert.Equal(t, "sqlite", pmp.db.Dialector.Name())

	//Checking with invalid type
	cfg["type"] = "invalid"
	pmp2 := SQLAggregatePump{}
	invalidDialectErr := pmp2.Init(cfg)
	assert.NotNil(t, invalidDialectErr)
	//TODO check how to test postgres connection - it's going to requiere to have some postgres up

}

func TestSQLAggregateWriteData(t *testing.T) {
	pmp := SQLAggregatePump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = "pmp_test.db"

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump Aggregate couldn't be initialized with err: ", err)
	}

	defer func() {
		os.Remove("pmp_test.db")
	}()

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
