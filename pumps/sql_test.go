package pumps

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"gopkg.in/vmihailenco/msgpack.v2"
)

func TestSQLInit(t *testing.T) {
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = "pmp_test.db"

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		os.Remove("pmp_test.db")
	}()

	assert.NotNil(t, pmp.db)
	assert.Equal(t, "sqlite", pmp.db.Dialector.Name())

	//Checking with invalid type
	cfg["type"] = "invalid"
	pmp2 := SQLPump{}
	invalidDialectErr := pmp2.Init(cfg)
	assert.NotNil(t, invalidDialectErr)
	//TODO check how to test postgres connection - it's going to requiere to have some postgres up

}

func TestSQLWriteData(t *testing.T) {
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = "pmp_test.db"

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		os.Remove("pmp_test.db")
	}()

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111", Day: 20}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	ctx := context.TODO()
	errWrite := pmp.WriteData(ctx, keys)
	if errWrite != nil {
		t.Fatal("SQL Pump couldn't write records with err:", errWrite)
	}

	var dbRecords []analytics.AnalyticsRecord
	if err := pmp.db.Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}

	assert.Len(t, dbRecords, 3)
	assert.Equal(t, "api111", dbRecords[0].APIID)
	//assert.Equal(t,20,dbRecords[1].Day) //TODO test it when days are saved
	assert.Equal(t, "api321", dbRecords[2].APIID)

}

func TestSQLWriteDataSharded(t *testing.T) {
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = "pmp_test.db"
	cfg["table_sharding"] = true
	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		os.Remove("pmp_test.db")
	}()

	keys := make([]interface{}, 5)
	now := time.Now()
	nowPlus1 := time.Now().AddDate(0, 0, 1)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111", TimeStamp: now}
	keys[1] = analytics.AnalyticsRecord{APIID: "api112", TimeStamp: now}
	keys[2] = analytics.AnalyticsRecord{APIID: "api113", TimeStamp: now}
	keys[3] = analytics.AnalyticsRecord{APIID: "api114", TimeStamp: nowPlus1}
	keys[4] = analytics.AnalyticsRecord{APIID: "api115", TimeStamp: nowPlus1}

	ctx := context.TODO()
	errWrite := pmp.WriteData(ctx, keys)
	if errWrite != nil {
		t.Fatal("SQL Pump couldn't write records with err:", errWrite)
	}
	table := "tyk_analytics_" + now.Format("20060102")
	assert.Equal(t, true, pmp.db.Migrator().HasTable(table))

	var dbRecords []analytics.AnalyticsRecord

	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}
	assert.Len(t, dbRecords, 3)

	tablePlus5 := "tyk_analytics_" + nowPlus1.Format("20060102")
	assert.Equal(t, true, pmp.db.Migrator().HasTable(tablePlus5))
	if err := pmp.db.Table(tablePlus5).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}
	assert.Len(t, dbRecords, 2)

}

func TestSQLWriteUptimeData(t *testing.T) {
	pmp := SQLPump{IsUptime: true}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = "pmp_test.db"
	cfg["table_sharding"] = false
	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		os.Remove("pmp_test.db")
	}()

	keys := make([]interface{}, 3)
	now := time.Now()
	nowPlus1 := time.Now().Add(2 * time.Hour)

	encoded, _ := msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now})
	keys[0] = string(encoded)
	keys[1] = string(encoded)
	keys[2] = string(encoded)

	pmp.WriteUptimeData(keys)
	table := "tyk_uptime_analytics"
	dbRecords := []analytics.UptimeReportAggregateSQL{}

	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}
	assert.Len(t, dbRecords, 2)

	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now})
	keys[0] = string(encoded)
	keys[1] = string(encoded)
	keys[2] = string(encoded)
	pmp.WriteUptimeData(keys)

	dbRecords = []analytics.UptimeReportAggregateSQL{}

	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}
	assert.Len(t, dbRecords, 2)

	assert.Equal(t, "total", dbRecords[1].DimensionValue)
	assert.Equal(t, 6, dbRecords[1].Hits)

	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: nowPlus1})
	keys[0] = string(encoded)
	keys[1] = string(encoded)
	keys[2] = string(encoded)

	pmp.WriteUptimeData(keys)

	dbRecords = []analytics.UptimeReportAggregateSQL{}

	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}
	assert.Len(t, dbRecords, 4)

}

func TestSQLWriteUptimeDataSharded(t *testing.T) {
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = "pmp_test.db"
	cfg["table_sharding"] = true
	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		os.Remove("pmp_test.db")
	}()

	keys := make([]interface{}, 5)
	now := time.Now()
	nowPlus1 := time.Now().AddDate(0, 0, 1)
	encoded, _ := msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: now})
	keys[0] = string(encoded)
	keys[1] = string(encoded)
	keys[2] = string(encoded)
	encoded, _ = msgpack.Marshal(analytics.UptimeReportData{OrgID: "1", URL: "url1", TimeStamp: nowPlus1})
	keys[3] = string(encoded)
	keys[4] = string(encoded)

	pmp.WriteUptimeData(keys)

	table := "tyk_uptime_analytics_" + now.Format("20060102")
	assert.Equal(t, true, pmp.db.Migrator().HasTable(table))

	var dbRecords []analytics.UptimeReportAggregateSQL

	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}
	assert.Len(t, dbRecords, 2)

	tablePlus5 := "tyk_uptime_analytics_" + nowPlus1.Format("20060102")
	assert.Equal(t, true, pmp.db.Migrator().HasTable(tablePlus5))
	if err := pmp.db.Table(tablePlus5).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}
	assert.Len(t, dbRecords, 2)

}

func TestSQLWriteUptimeDataAggregations(t *testing.T) {
	pmp := SQLPump{IsUptime: true}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = "pmp_test.db"
	cfg["table_sharding"] = false
	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}
	defer func() {
		os.Remove("pmp_test.db")
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
	table := "tyk_uptime_analytics"
	dbRecords := []analytics.UptimeReportAggregateSQL{}

	if err := pmp.db.Table(table).Find(&dbRecords).Error; err != nil {
		t.Fatal("Error getting analytics records from SQL")
	}

	fmt.Printf("%+v\n", dbRecords[0])

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
