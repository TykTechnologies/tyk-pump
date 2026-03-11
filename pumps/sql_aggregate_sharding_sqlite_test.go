package pumps

import (
	"context"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLAggregateWriteData_Sharding_WithSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		AutoEmbedd:  true,
		UseJSONTags: true,
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	pmp := &SQLAggregatePump{
		db: db,
		SQLConf: &SQLAggregatePumpConf{
			SQLConf: SQLConf{
				Type:          "sqlite",
				TableSharding: true,
			},
			OmitIndexCreation: true,
		},
	}
	pmp.log = log.WithField("prefix", "test-pump")

	now := time.Now()
	shardedTable := analytics.AggregateSQLTable + "_" + now.Format("20060102")
	defaultTable := analytics.AggregateSQLTable

	keys := []interface{}{
		analytics.AnalyticsRecord{
			OrgID:        "1",
			APIID:        "api1",
			ResponseCode: 200,
			TimeStamp:    now,
		},
	}

	err = pmp.WriteData(context.TODO(), keys)
	assert.NoError(t, err)

	// Check if data is in the sharded table
	var shardedRecords []analytics.SQLAnalyticsRecordAggregate
	err = db.Table(shardedTable).Find(&shardedRecords).Error
	assert.NoError(t, err)
	assert.Len(t, shardedRecords, 2, "data should be in the sharded table")

	// Check if data is in the default table (it shouldn't be)
	var defaultRecords []analytics.SQLAnalyticsRecordAggregate
	err = db.Table(defaultTable).Find(&defaultRecords).Error
	assert.Error(t, err, "default table should not exist") // Expecting an error because the table should not have been created
	assert.Len(t, defaultRecords, 0)
}
