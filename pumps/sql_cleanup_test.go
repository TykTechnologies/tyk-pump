package pumps

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func cleanupGormDB(t *testing.T, db *gorm.DB, tables ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, table := range tables {
			_ = db.Migrator().DropTable(table)
		}
		closeGormDB(t, db)
	})
}

func closeGormDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	_ = sqlDB.Close()
}

func waitForAggregateIndexReady(t *testing.T, db *gorm.DB, table string, ch <-chan bool) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		indexName := fmt.Sprintf("%s_%s", table, newAggregatedIndexName)
		require.True(t, db.Migrator().HasIndex(table, indexName),
			"background aggregate index creation should complete or already exist")
	}
}
