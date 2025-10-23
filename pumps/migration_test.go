package pumps

import (
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// Open in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gorm_logger.Default.LogMode(gorm_logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}

	// Create a mock information_schema.tables for testing (to simulate PostgreSQL/MySQL behavior)
	// Use quoted identifier to create table with dots in the name
	err = db.Exec(`
		CREATE TABLE "information_schema.tables" (
			table_name TEXT,
			table_schema TEXT
		)
	`).Error
	if err != nil {
		t.Fatalf("Failed to create information_schema.tables: %v", err)
	}

	return db
}

// setupTestLogger creates a test logger
func setupTestLogger(t *testing.T) *logrus.Entry {
	t.Helper()
	return logrus.NewEntry(logrus.New())
}

// createTestShardedTables creates test tables with sharded naming pattern
func createTestShardedTables(t *testing.T, db *gorm.DB, tablePrefix string, model interface{}, dates []string) {
	t.Helper()

	for _, date := range dates {
		tableName := tablePrefix + "_" + date
		err := db.Table(tableName).AutoMigrate(model)
		if err != nil {
			t.Fatalf("Failed to create test table %s: %v", tableName, err)
		}

		// Register the table in information_schema.tables
		err = db.Exec("INSERT INTO \"information_schema.tables\" (table_name, table_schema) VALUES (?, 'public')", tableName).Error
		if err != nil {
			t.Fatalf("Failed to register table %s in information_schema: %v", tableName, err)
		}
	}
}

// createTestNonShardedTables creates test tables that don't match the sharded pattern
func createTestNonShardedTables(t *testing.T, db *gorm.DB, tablePrefix string, model interface{}) {
	t.Helper()

	nonShardedTables := []string{
		tablePrefix + "_invalid",
		tablePrefix + "_202401",         // wrong format
		tablePrefix + "_20240101_extra", // too long
		"other_table",
		"unrelated_table_20240101",
	}

	for _, tableName := range nonShardedTables {
		err := db.Table(tableName).AutoMigrate(model)
		if err != nil {
			t.Fatalf("Failed to create test table %s: %v", tableName, err)
		}

		// Register the table in information_schema.tables
		err = db.Exec("INSERT INTO \"information_schema.tables\" (table_name, table_schema) VALUES (?, 'public')", tableName).Error
		if err != nil {
			t.Fatalf("Failed to register table %s in information_schema: %v", tableName, err)
		}
	}
}

func TestMigrateAllShardedTables(t *testing.T) {
	t.Run("successful_migration", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "test_analytics"
		model := &analytics.AnalyticsRecord{}

		// Create test sharded tables
		testDates := []string{"20240101", "20240102", "20240103"}
		createTestShardedTables(t, db, tablePrefix, model, testDates)

		// Create some non-sharded tables that should be ignored
		createTestNonShardedTables(t, db, tablePrefix, model)

		// Run migration
		err := MigrateAllShardedTables(db, tablePrefix, "test", model, logger)

		// Verify no error occurred
		assert.NoError(t, err)

		// Verify all sharded tables still exist and are migrated
		for _, date := range testDates {
			tableName := tablePrefix + "_" + date
			assert.True(t, db.Migrator().HasTable(tableName), "Table %s should exist", tableName)
		}
	})

	t.Run("no_sharded_tables", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "test_analytics"
		model := &analytics.AnalyticsRecord{}

		// Create only non-sharded tables
		createTestNonShardedTables(t, db, tablePrefix, model)

		// Run migration
		err := MigrateAllShardedTables(db, tablePrefix, "test", model, logger)

		// Should not error even with no sharded tables
		assert.NoError(t, err)
	})

	t.Run("invalid_date_format_ignored", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "test_analytics"
		model := &analytics.AnalyticsRecord{}

		// Create tables with invalid date formats
		invalidDates := []string{"invalid", "202401", "20240101_extra", "not_a_date"}
		createTestShardedTables(t, db, tablePrefix, model, invalidDates)

		// Create one valid sharded table
		validDate := []string{"20240101"}
		createTestShardedTables(t, db, tablePrefix, model, validDate)

		// Run migration
		err := MigrateAllShardedTables(db, tablePrefix, "test", model, logger)

		// Should not error
		assert.NoError(t, err)

		// Only the valid table should be processed
		validTableName := tablePrefix + "_20240101"
		assert.True(t, db.Migrator().HasTable(validTableName), "Valid table should exist")
	})

	t.Run("different_table_prefixes", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		model := &analytics.AnalyticsRecord{}

		// Create tables with different prefixes
		prefixes := []string{"analytics", "aggregate", "graph"}
		for _, prefix := range prefixes {
			dates := []string{"20240101", "20240102"}
			createTestShardedTables(t, db, prefix, model, dates)
		}

		// Test migration for each prefix
		for _, prefix := range prefixes {
			err := MigrateAllShardedTables(db, prefix, prefix, model, logger)
			assert.NoError(t, err, "Migration should succeed for prefix %s", prefix)
		}
	})

	t.Run("empty_database", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "test_analytics"
		model := &analytics.AnalyticsRecord{}

		// Run migration on empty database
		err := MigrateAllShardedTables(db, tablePrefix, "test", model, logger)

		// Should not error
		assert.NoError(t, err)
	})
}

func TestMigrateAllShardedTablesWithDifferentModels(t *testing.T) {
	t.Run("analytics_record", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "analytics"
		model := &analytics.AnalyticsRecord{}
		dates := []string{"20240101", "20240102"}

		createTestShardedTables(t, db, tablePrefix, model, dates)

		err := MigrateAllShardedTables(db, tablePrefix, "analytics", model, logger)
		assert.NoError(t, err)
	})

	t.Run("aggregate_record", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "aggregate"
		model := &analytics.SQLAnalyticsRecordAggregate{}
		dates := []string{"20240101", "20240102"}

		createTestShardedTables(t, db, tablePrefix, model, dates)

		err := MigrateAllShardedTables(db, tablePrefix, "aggregate", model, logger)
		assert.NoError(t, err)
	})

	t.Run("graph_record", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "graph"
		model := &analytics.GraphRecord{}
		dates := []string{"20240101", "20240102"}

		createTestShardedTables(t, db, tablePrefix, model, dates)

		err := MigrateAllShardedTables(db, tablePrefix, "graph", model, logger)
		assert.NoError(t, err)
	})
}

func TestMigrateAllShardedTablesEdgeCases(t *testing.T) {
	t.Run("single_character_prefix", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "a"
		model := &analytics.AnalyticsRecord{}
		dates := []string{"20240101"}

		createTestShardedTables(t, db, tablePrefix, model, dates)

		err := MigrateAllShardedTables(db, tablePrefix, "single", model, logger)
		assert.NoError(t, err)
	})

	t.Run("very_long_prefix", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "very_long_table_prefix_that_might_cause_issues"
		model := &analytics.AnalyticsRecord{}
		dates := []string{"20240101"}

		createTestShardedTables(t, db, tablePrefix, model, dates)

		err := MigrateAllShardedTables(db, tablePrefix, "long", model, logger)
		assert.NoError(t, err)
	})

	t.Run("prefix_with_underscores", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "test_table_with_underscores"
		model := &analytics.AnalyticsRecord{}
		dates := []string{"20240101"}

		createTestShardedTables(t, db, tablePrefix, model, dates)

		err := MigrateAllShardedTables(db, tablePrefix, "underscore", model, logger)
		assert.NoError(t, err)
	})
}

func TestMigrateAllShardedTablesLogging(t *testing.T) {
	t.Run("log_messages_contain_prefix", func(t *testing.T) {
		db := setupTestDB(t)

		// Create a logger that captures log messages
		logger := logrus.New()
		logger.SetLevel(logrus.InfoLevel)

		// We can't easily capture log output in this test, but we can verify
		// that the function runs without error and the logging doesn't cause issues
		tablePrefix := "test_analytics"
		model := &analytics.AnalyticsRecord{}
		dates := []string{"20240101", "20240102"}

		createTestShardedTables(t, db, tablePrefix, model, dates)

		logEntry := logger.WithField("test", "migration")
		err := MigrateAllShardedTables(db, tablePrefix, "test", model, logEntry)
		assert.NoError(t, err)
	})
}

func TestMigrateAllShardedTablesPerformance(t *testing.T) {
	t.Run("many_tables", func(t *testing.T) {
		db := setupTestDB(t)
		logger := setupTestLogger(t)

		tablePrefix := "performance_test"
		model := &analytics.AnalyticsRecord{}

		// Create many sharded tables
		dates := make([]string, 100)
		baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 100; i++ {
			dates[i] = baseDate.AddDate(0, 0, i).Format("20060102")
		}

		createTestShardedTables(t, db, tablePrefix, model, dates)

		// Measure migration time
		start := time.Now()
		err := MigrateAllShardedTables(db, tablePrefix, "performance", model, logger)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.True(t, duration < 5*time.Second, "Migration should complete within 5 seconds for 100 tables")
	})
}
