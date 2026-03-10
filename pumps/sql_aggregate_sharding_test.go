package pumps

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/TykTechnologies/tyk-pump/analytics"
	logrus "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSQLAggregateWriteData_Sharding_GormV2_Bug(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a gorm database", err)
	}

	pmp := &SQLAggregatePump{
		db:     gormDB,
		dbType: "postgres",
		SQLConf: &SQLAggregatePumpConf{
			SQLConf: SQLConf{
				TableSharding: true,
				BatchSize:     10,
			},
		},
	}
	pmp.log = logrus.WithField("prefix", "test-pump")

	now := time.Now()
	table := analytics.AggregateSQLTable + "_" + now.Format("20060102")

	keys := []interface{}{
		analytics.AnalyticsRecord{OrgID: "1", TimeStamp: now},
	}

	// Mock has table check (return 0 to trigger table creation)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = $1 AND table_type = $2`)).
		WithArgs(table, "BASE TABLE").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Mock create table
	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE "` + table + `"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock has index check
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM pg_indexes WHERE tablename = $1 AND indexname = $2 AND schemaname = CURRENT_SCHEMA()`)).
		WithArgs(table, table+"_idx_dimension").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Mock create index
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("CREATE INDEX %s IF NOT EXISTS %s ON %s (dimension, timestamp, org_id, dimension_value)", "CONCURRENTLY", table+"_idx_dimension", table))).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "` + table + `"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = pmp.WriteData(context.TODO(), keys)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err, "there were unfulfilled expectations")
}
