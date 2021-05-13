package pumps

import (
	"context"
	"os"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T){
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["dsn"] = "pmp_test.db"

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}else{
		defer func(){
			os.Remove( "pmp_test.db")
		}()
	}

	assert.NotNil(t,pmp.db)
	assert.Equal(t,"sqlite",pmp.db.Dialector.Name())


	//Checking with invalid type
	cfg["type"] = "invalid"
	pmp2 := SQLPump{}
	invalidDialectErr := pmp2.Init(cfg)
	assert.NotNil(t,invalidDialectErr)
	//TODO check how to test postgres connection - it's going to requiere to have some postgres up


}

func TestInsertRecord(t *testing.T){
	pmp := SQLPump{}
	cfg := make(map[string]interface{})
	cfg["type"] = "sqlite"
	cfg["dsn"] = "pmp_test.db"

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err: ", err)
	}else{
		defer func(){
			os.Remove( "pmp_test.db")
		}()
	}
	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111",Day: 20}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	ctx := context.TODO()
	errWrite := pmp.WriteData(ctx,keys)
	if errWrite != nil {
		t.Fatal("SQL Pump couldn't write records with err:",errWrite)
	}

	var dbRecords []analytics.AnalyticsRecord
	if err:= pmp.db.Find(&dbRecords).Error ; err != nil{
		t.Fatal("Error getting analytics records from SQL")
	}

	assert.Len(t,dbRecords,3)
	assert.Equal(t,"api111",dbRecords[0].APIID)
	//assert.Equal(t,20,dbRecords[1].Day) //TODO test it when days are saved
	assert.Equal(t,"api321",dbRecords[2].APIID)
}
