package pumps

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
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

}

func TestSQLWriteData(t *testing.T) {
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["connection_string"] = ""

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}

	defer func(table string) {
		pmp.db.Migrator().DropTable(table)
	}(table)

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

		table := "tyk_analytics"
		assert.Equal(t, true, pmp.db.Migrator().HasTable(table))
		err := pmp.db.Table(table).Find(&dbRecords).Error
		assert.Nil(t, err)
		assert.Equal(t, 3, len(dbRecords))

	})

	t.Run("table_content", func(t *testing.T) {
		var dbRecords []analytics.AnalyticsRecord

		table := "tyk_analytics"
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
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["table_sharding"] = true
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
		date          time.Time
		amountRecords int
	}{
		"shard_1": {
			date:          now,
			amountRecords: 3,
		},
		"shard_2": {
			date:          nowPlus1,
			amountRecords: 2,
		},
		"shard_3": {
			date:          nowPlus2,
			amountRecords: 1,
		},
	}

	for testName, data := range tests {
		t.Run(testName, func(t *testing.T) {
			var dbRecords []analytics.AnalyticsRecord

			table := "tyk_analytics_" + data.date.Format("20060102")
			defer func(table string) {
				pmp.db.Migrator().DropTable(table)
			}(table)
			assert.Equal(t, true, pmp.db.Migrator().HasTable(table))
			err := pmp.db.Table(table).Find(&dbRecords).Error
			assert.Nil(t, err)
			assert.Equal(t, data.amountRecords, len(dbRecords))
		})
	}

}
