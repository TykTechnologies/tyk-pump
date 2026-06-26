//go:build race

package pumps

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Verifies: SW-REQ-064
// Verifies: SW-REQ-065
// Verifies: SW-REQ-066
// Verifies: KI:sql-aggregate-sharded-shared-db-race
// Reproduces: sql-aggregate-sharded-shared-db-race
func TestSQLAggregatePump_WriteDataShardedSharedDBRace_KI(t *testing.T) {
	if os.Getenv("TYK_PUMP_SQL_AGGREGATE_SHARDED_DB_RACE_CHILD") == "1" {
		runSQLAggregatePumpWriteDataShardedSharedDBRaceChild(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestSQLAggregatePump_WriteDataShardedSharedDBRace_KI$")
	cmd.Env = append(os.Environ(), "TYK_PUMP_SQL_AGGREGATE_SHARDED_DB_RACE_CHILD=1")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "KI active: race-instrumented child should report shared SQL aggregate db state races")
	text := string(output)
	assert.Contains(t, text, "DATA RACE", "child output should contain race detector output:\n%s", text)
	assert.Contains(t, text, "sql_aggregate.go", "race should originate in SQL aggregate shared DB mutation:\n%s", text)
}

func runSQLAggregatePumpWriteDataShardedSharedDBRaceChild(t *testing.T) {
	t.Helper()

	dbPath := t.TempDir() + "/sql-aggregate-race.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		AutoEmbedd:  true,
		UseJSONTags: true,
		Logger:      logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	day1 := time.Date(2099, 11, 1, 10, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)
	for _, day := range []time.Time{day1, day2} {
		table := analytics.AggregateSQLTable + "_" + day.Format("20060102")
		require.NoError(t, db.Table(table).AutoMigrate(&analytics.SQLAnalyticsRecordAggregate{}))
	}

	pump := &SQLAggregatePump{
		SQLConf: &SQLAggregatePumpConf{
			SQLConf: SQLConf{
				Type:          "sqlite",
				TableSharding: true,
				BatchSize:     SQLDefaultQueryBatchSize,
			},
			OmitIndexCreation: true,
		},
		db:     db,
		dbType: "sqlite",
	}
	pump.log = log.WithField("prefix", SQLAggregatePumpPrefix)

	records := []interface{}{
		sqlAggregateRaceRecord("race-a", "race-org", day1),
		sqlAggregateRaceRecord("race-b", "race-org", day1.Add(time.Minute)),
		sqlAggregateRaceRecord("race-c", "race-org", day2),
	}

	const workers = 8
	const iterations = 40
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				_ = pump.WriteData(context.Background(), records)
			}
		}()
	}
	close(start)
	wg.Wait()
}

func sqlAggregateRaceRecord(apiID, orgID string, ts time.Time) analytics.AnalyticsRecord {
	return analytics.AnalyticsRecord{
		APIID:        apiID,
		APIName:      "SQL aggregate race KI",
		OrgID:        orgID,
		Path:         "/" + strings.ToLower(apiID),
		Method:       "GET",
		ResponseCode: 200,
		TimeStamp:    ts,
		ExpireAt:     ts.Add(time.Hour),
	}
}
