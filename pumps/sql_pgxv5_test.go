package pumps

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

// capturingLogger records every SQL statement executed through gorm.
// Used to verify that AutoMigrate does not emit spurious ALTER TABLE on a
// second run — the primary risk introduced by the MigrateColumn() changes
// in the gorm fork (commits c3933cb, 6d5ba65) that check Unique, DefaultValue,
// and Comment fields now reported by pgx/v5.
type capturingLogger struct {
	mu      sync.Mutex
	queries []string
}

func (l *capturingLogger) LogMode(gorm_logger.LogLevel) gorm_logger.Interface  { return l }
func (l *capturingLogger) Info(_ context.Context, _ string, _ ...interface{})  {}
func (l *capturingLogger) Warn(_ context.Context, _ string, _ ...interface{})  {}
func (l *capturingLogger) Error(_ context.Context, _ string, _ ...interface{}) {}
func (l *capturingLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.mu.Lock()
	l.queries = append(l.queries, sql)
	l.mu.Unlock()
}

// hasAlterTable returns true if any captured SQL contains ALTER TABLE (case-insensitive).
func (l *capturingLogger) hasAlterTable() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, q := range l.queries {
		if strings.Contains(strings.ToUpper(q), "ALTER TABLE") {
			return true
		}
	}
	return false
}

// captureSession wraps db with a capturing logger and returns both.
// The original db is not modified.
func captureSession(db *gorm.DB) (*gorm.DB, *capturingLogger) {
	cl := &capturingLogger{}
	return db.Session(&gorm.Session{Logger: cl}), cl
}

// ── 1. Migration Idempotency ──────────────────────────────────────────────────

// TestMigrationIdempotency_Postgres verifies that calling AutoMigrate twice on an
// already-migrated table does not emit any ALTER TABLE statement.
//
// This guards against the new MigrateColumn() checks (Unique / DefaultValue / Comment)
// in the gorm fork misfiring when pgx/v5 reports column metadata differently from pgx/v4.
func TestMigrationIdempotency_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	t.Run("SQLPump", func(t *testing.T) {
		pmp := SQLPump{}
		if err := pmp.Init(newSQLConfig(false)); err != nil {
			t.Fatalf("Init failed: %v", err)
		}
		t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

		captureDB, cl := captureSession(pmp.db)
		if err := captureDB.Table(analytics.SQLTable).AutoMigrate(&analytics.AnalyticsRecord{}); err != nil {
			t.Fatalf("second AutoMigrate failed: %v", err)
		}
		assert.False(t, cl.hasAlterTable(),
			"second AutoMigrate on %s must not emit ALTER TABLE (pgx/v5 ColumnType regression check)",
			analytics.SQLTable)
	})

	t.Run("SQLAggregatePump", func(t *testing.T) {
		pmp := SQLAggregatePump{}
		if err := pmp.Init(newSQLConfig(false)); err != nil {
			t.Fatalf("Init failed: %v", err)
		}
		// Init always starts a background goroutine for index creation on postgres.
		<-pmp.backgroundIndexCreated
		t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.AggregateSQLTable) })

		captureDB, cl := captureSession(pmp.db)
		if err := captureDB.Table(analytics.AggregateSQLTable).AutoMigrate(&analytics.SQLAnalyticsRecordAggregate{}); err != nil {
			t.Fatalf("second AutoMigrate failed: %v", err)
		}
		assert.False(t, cl.hasAlterTable(),
			"second AutoMigrate on %s must not emit ALTER TABLE", analytics.AggregateSQLTable)
	})

	t.Run("GraphSQLPump", func(t *testing.T) {
		analytics.GraphSQLTableName = ""
		cfg := map[string]interface{}{
			"type":              "postgres",
			"connection_string": getTestPostgresConnectionString(),
		}
		pmp := GraphSQLPump{}
		if err := pmp.Init(cfg); err != nil {
			t.Fatalf("Init failed: %v", err)
		}
		tableName := pmp.tableName
		t.Cleanup(func() {
			pmp.db.Migrator().DropTable(tableName)
			analytics.GraphSQLTableName = ""
		})

		captureDB, cl := captureSession(pmp.db)
		if err := captureDB.Table(tableName).AutoMigrate(&analytics.GraphRecord{}); err != nil {
			t.Fatalf("second AutoMigrate failed: %v", err)
		}
		assert.False(t, cl.hasAlterTable(),
			"second AutoMigrate on %s must not emit ALTER TABLE", tableName)
	})
}

// TestShardedMigrationIdempotency_Postgres verifies that running MigrateAllShardedTables
// on already-migrated shards does not emit ALTER TABLE on the second call.
func TestShardedMigrationIdempotency_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	// Use far-future dates to avoid colliding with any test that uses time.Now().
	dates := []string{"20990101", "20990102", "20990103"}
	for _, date := range dates {
		date := date // capture loop variable
		tableName := analytics.SQLTable + "_" + date
		if err := pmp.db.Table(tableName).AutoMigrate(&analytics.AnalyticsRecord{}); err != nil {
			t.Fatalf("creating sharded table %s failed: %v", tableName, err)
		}
		t.Cleanup(func() { pmp.db.Migrator().DropTable(tableName) })
	}

	// First call establishes the baseline — no captures needed.
	if err := MigrateAllShardedTables(pmp.db, analytics.SQLTable, "", &analytics.AnalyticsRecord{}, pmp.log); err != nil {
		t.Fatalf("first MigrateAllShardedTables failed: %v", err)
	}

	// Second call: capture all SQL and assert no ALTER TABLE is emitted.
	captureDB, cl := captureSession(pmp.db)
	if err := MigrateAllShardedTables(captureDB, analytics.SQLTable, "", &analytics.AnalyticsRecord{}, pmp.log); err != nil {
		t.Fatalf("second MigrateAllShardedTables failed: %v", err)
	}
	assert.False(t, cl.hasAlterTable(),
		"second MigrateAllShardedTables must not emit ALTER TABLE on already-migrated shards")
}

// ── 2. Batch Writes ───────────────────────────────────────────────────────────

// TestBatchInsertLargePayload_Postgres writes 5000 records in a single WriteData call
// (5 × batch_size=1000) and verifies count and data integrity.
func TestBatchInsertLargePayload_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	cfg := newSQLConfig(false)
	cfg["batch_size"] = 1000
	pmp := SQLPump{}
	if err := pmp.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	const total = 5000
	now := time.Now()
	keys := make([]interface{}, total)
	for i := 0; i < total; i++ {
		keys[i] = analytics.AnalyticsRecord{
			APIID:     fmt.Sprintf("batch-api-%d", i),
			OrgID:     "batch-test-pgxv5",
			TimeStamp: now,
		}
	}

	if err := pmp.WriteData(context.Background(), keys); err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	var count int64
	pmp.db.Table(analytics.SQLTable).Where("orgid = ?", "batch-test-pgxv5").Count(&count)
	assert.Equal(t, int64(total), count, "all %d records should be persisted across 5 batches", total)

	// Spot-check data integrity for first, middle, and last records.
	for _, idx := range []int{0, total / 2, total - 1} {
		expectedAPIID := fmt.Sprintf("batch-api-%d", idx)
		var rec analytics.AnalyticsRecord
		result := pmp.db.Table(analytics.SQLTable).Where("apiid = ?", expectedAPIID).First(&rec)
		assert.NoError(t, result.Error, "record at index %d should be findable", idx)
		assert.Equal(t, expectedAPIID, rec.APIID)
		assert.Equal(t, "batch-test-pgxv5", rec.OrgID)
	}
}

// ── 3. Upsert / ON CONFLICT ───────────────────────────────────────────────────

// TestUpsertOnConflict_Postgres validates clause.OnConflict behaviour in SQLAggregatePump.
//
// This is the highest-risk area for pgx/v5 changes: the pump uses named EXCLUDED column
// references in its on-conflict assignment expressions. If pgx/v5 rejects or silently
// drops the conflict clause, the second write would insert duplicate rows instead of
// merging, and the hit count would not accumulate.
func TestUpsertOnConflict_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := &SQLAggregatePump{}
	cfg := newSQLConfig(false)
	cfg["batch_size"] = 1000
	if err := pmp.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	<-pmp.backgroundIndexCreated
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.AggregateSQLTable) })

	// Fixed timestamp ensures all writes land in the same aggregation bucket.
	fixedTS := time.Date(2099, 6, 1, 10, 0, 0, 0, time.UTC)

	writeN := func(n int) {
		t.Helper()
		keys := make([]interface{}, n)
		for i := 0; i < n; i++ {
			keys[i] = analytics.AnalyticsRecord{
				OrgID:     "upsert-test-org",
				APIID:     "upsert-test-api",
				TimeStamp: fixedTS,
			}
		}
		if err := pmp.WriteData(context.Background(), keys); err != nil {
			t.Fatalf("WriteData(%d) failed: %v", n, err)
		}
	}

	// First write: 3 records → 2 aggregate rows (dimension "apiid" + "total").
	writeN(3)
	var rowCount int64
	pmp.db.Table(analytics.AggregateSQLTable).Count(&rowCount)
	assert.Equal(t, int64(2), rowCount, "first write should produce 2 aggregate rows")

	// Second write: 2 more records for the same key → ON CONFLICT must update, not insert.
	writeN(2)
	pmp.db.Table(analytics.AggregateSQLTable).Count(&rowCount)
	assert.Equal(t, int64(2), rowCount,
		"second write must upsert in-place; if row count grew, ON CONFLICT clause is broken with pgx/v5")

	// Verify accumulated hit count on the "total" dimension row.
	var totRec analytics.SQLAnalyticsRecordAggregate
	result := pmp.db.Table(analytics.AggregateSQLTable).
		Where("dimension_value = ? AND org_id = ?", "total", "upsert-test-org").
		First(&totRec)
	assert.NoError(t, result.Error)
	assert.Equal(t, 5, totRec.Hits,
		"hits should accumulate across writes: 3 (first) + 2 (second) = 5")
}

// ── 4. Connection Pool ────────────────────────────────────────────────────────

// TestConcurrentWrites_Postgres exercises the pool under genuine contention by capping
// MaxOpenConns to 5 while running 50 concurrent goroutines. Each goroutine writes 50
// records. Validates that no errors occur and all 2500 records are persisted.
func TestConcurrentWrites_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	sqlDB, err := pmp.db.DB()
	if err != nil {
		t.Fatalf("pmp.db.DB() failed: %v", err)
	}
	// Cap to 5 connections to create real pool contention.
	sqlDB.SetMaxOpenConns(5)

	const (
		workers          = 50
		recordsPerWorker = 50
	)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			now := time.Now()
			keys := make([]interface{}, recordsPerWorker)
			for j := 0; j < recordsPerWorker; j++ {
				keys[j] = analytics.AnalyticsRecord{
					APIID:     fmt.Sprintf("concurrent-api-%d-%d", workerID, j),
					OrgID:     "concurrent-test-pgxv5",
					TimeStamp: now,
				}
			}
			if writeErr := pmp.WriteData(context.Background(), keys); writeErr != nil {
				mu.Lock()
				errs = append(errs, writeErr)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	assert.Empty(t, errs, "no errors expected under concurrent writes with capped pool")

	var count int64
	pmp.db.Table(analytics.SQLTable).Where("orgid = ?", "concurrent-test-pgxv5").Count(&count)
	assert.Equal(t, int64(workers*recordsPerWorker), count,
		"all %d records should be persisted", workers*recordsPerWorker)

	stats := sqlDB.Stats()
	assert.Equal(t, 0, stats.InUse,
		"all connections should be returned to pool after completion")
}

// TestConnectionPoolStats_Postgres documents the default pool settings after the pgx/v5
// upgrade. tyk-pump intentionally does not call SetMaxOpenConns, so the default must
// remain 0 (unlimited). This test acts as a canary: if a future driver version silently
// imposes a default cap, this test will catch it.
func TestConnectionPoolStats_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	sqlDB, err := pmp.db.DB()
	if err != nil {
		t.Fatalf("pmp.db.DB() failed: %v", err)
	}

	stats := sqlDB.Stats()
	assert.Equal(t, 0, stats.MaxOpenConnections,
		"tyk-pump does not configure MaxOpenConns; driver default must remain 0 (unlimited)")
}

// ── 5. Sharded Table Lifecycle ────────────────────────────────────────────────

// TestShardedTableLifecycle_Postgres writes records spanning 5 different dates, verifies
// that 5 sharded tables are created with all 6 expected indexes, then runs
// MigrateAllShardedTables and confirms no ALTER TABLE is emitted.
func TestShardedTableLifecycle_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(true)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Drop the current-day table that Init creates when table_sharding is true.
	todayTable := analytics.SQLTable + "_" + time.Now().Format("20060102")
	t.Cleanup(func() { pmp.db.Migrator().DropTable(todayTable) })

	// Use far-future dates to avoid collision with other sharding tests.
	base := time.Date(2099, 3, 1, 0, 0, 0, 0, time.UTC)
	keys := make([]interface{}, 5)
	shardTables := make([]string, 5)
	for i := 0; i < 5; i++ {
		ts := base.AddDate(0, 0, i)
		keys[i] = analytics.AnalyticsRecord{
			APIID:     fmt.Sprintf("shard-api-%d", i),
			OrgID:     "shard-lifecycle-pgxv5",
			TimeStamp: ts,
		}
		shardTables[i] = analytics.SQLTable + "_" + ts.Format("20060102")
	}
	t.Cleanup(func() {
		for _, tbl := range shardTables {
			pmp.db.Migrator().DropTable(tbl)
		}
	})

	if err := pmp.WriteData(context.Background(), keys); err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	for _, tableName := range shardTables {
		assert.True(t, pmp.db.Migrator().HasTable(tableName),
			"sharded table %s should exist after WriteData", tableName)

		// Verify all 6 expected indexes are present.
		for _, idx := range indexes {
			idxName := pmp.buildIndexName(idx.baseName, tableName)
			assert.True(t,
				pmp.db.Migrator().HasIndex(tableName, idxName),
				"index %s on table %s should exist", idxName, tableName)
		}
	}

	// MigrateAllShardedTables on already-correct tables must emit no ALTER TABLE.
	captureDB, cl := captureSession(pmp.db)
	if err := MigrateAllShardedTables(captureDB, analytics.SQLTable, "", &analytics.AnalyticsRecord{}, pmp.log); err != nil {
		t.Fatalf("MigrateAllShardedTables failed: %v", err)
	}
	assert.False(t, cl.hasAlterTable(),
		"MigrateAllShardedTables on already-migrated shards must not emit ALTER TABLE")
}

// ── 6. Error Translation ──────────────────────────────────────────────────────

// TestDuplicateKeyError_Postgres validates the full error-translation chain introduced
// by gorm fork commit 61fd065:
//
//	pgx/v5 UniqueViolation → ErrorTranslator.Translate → gorm.ErrDuplicatedKey
func TestDuplicateKeyError_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{IsUptime: true}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.UptimeSQLTable) })

	rec := analytics.UptimeReportAggregateSQL{ID: "dup-key-pgxv5-test"}

	if err := pmp.db.Table(analytics.UptimeSQLTable).Create(&rec).Error; err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	result := pmp.db.Table(analytics.UptimeSQLTable).Create(&rec)
	assert.ErrorIs(t, result.Error, gorm.ErrDuplicatedKey,
		"inserting a duplicate primary key must be translated to gorm.ErrDuplicatedKey via ErrorTranslator")
}

// ── 7. PreferSimpleProtocol ───────────────────────────────────────────────────

// TestPreferSimpleProtocol_Postgres exercises the non-prepared-statement code path in
// pgx/v5 (PreferSimpleProtocol: true). This disables pgx's extended query protocol and
// routes all queries through the simple protocol, hitting a different code path in
// the driver that must not regress.
func TestPreferSimpleProtocol_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	cfg := newSQLConfig(false)
	cfg["postgres"] = map[string]interface{}{"prefer_simple_protocol": true}

	pmp := SQLPump{}
	if err := pmp.Init(cfg); err != nil {
		t.Fatalf("Init with prefer_simple_protocol failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	now := time.Now()
	keys := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		keys[i] = analytics.AnalyticsRecord{
			APIID:     fmt.Sprintf("simple-proto-api-%d", i),
			OrgID:     "simple-proto-test-pgxv5",
			TimeStamp: now,
		}
	}

	if err := pmp.WriteData(context.Background(), keys); err != nil {
		t.Fatalf("WriteData with prefer_simple_protocol failed: %v", err)
	}

	var count int64
	pmp.db.Table(analytics.SQLTable).Where("orgid = ?", "simple-proto-test-pgxv5").Count(&count)
	assert.Equal(t, int64(10), count,
		"10 records should be written and readable via simple protocol path")
}
