package pumps

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func (l *capturingLogger) LogMode(gorm_logger.LogLevel) gorm_logger.Interface { return l }

func (l *capturingLogger) Info(_ context.Context, _ string, _ ...interface{}) {}

func (l *capturingLogger) Warn(_ context.Context, _ string, _ ...interface{}) {}

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
//
// Verifies: INT-REQ-007
// MCDC INT-REQ-007: expand_contract_migration=F, sql_schema_version_changed=F => TRUE
func TestMigrationIdempotency_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	t.Run("SQLPump", func(t *testing.T) {
		pmp := SQLPump{}
		if err := pmp.Init(newSQLConfig(false)); err != nil {
			t.Fatalf("Init failed: %v", err)
		}
		cleanupGormDB(t, pmp.db, analytics.SQLTable)

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
		waitForAggregateIndexReady(t, pmp.db, analytics.AggregateSQLTable, pmp.backgroundIndexCreated)
		cleanupGormDB(t, pmp.db, analytics.AggregateSQLTable)

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
//
// Verifies: INT-REQ-007
// MCDC INT-REQ-007: expand_contract_migration=F, sql_schema_version_changed=F => TRUE
func TestShardedMigrationIdempotency_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.SQLTable)

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
// Verifies: SW-REQ-040
// MCDC SW-REQ-040: day_sliced_routing=F, table_sharding=F => TRUE
func TestBatchInsertLargePayload_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	cfg := newSQLConfig(false)
	cfg["batch_size"] = 1000
	pmp := SQLPump{}
	if err := pmp.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.SQLTable)

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
// Verifies: SW-REQ-067
// MCDC SW-REQ-067: on_conflict_assignments_applied=F, row_conflict_detected=F => TRUE
// MCDC SW-REQ-067: on_conflict_assignments_applied=T, row_conflict_detected=T => TRUE
func TestUpsertOnConflict_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := &SQLAggregatePump{}
	cfg := newSQLConfig(false)
	cfg["batch_size"] = 1000
	if err := pmp.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	waitForAggregateIndexReady(t, pmp.db, analytics.AggregateSQLTable, pmp.backgroundIndexCreated)
	cleanupGormDB(t, pmp.db, analytics.AggregateSQLTable)

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
// Verifies: SW-REQ-040
// SW-REQ-040:concurrent:nominal
// SW-REQ-040:concurrent:race
// SW-REQ-040:connection_leak_free:nominal
// MCDC SW-REQ-040: day_sliced_routing=F, table_sharding=F => TRUE
func TestConcurrentWrites_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.SQLTable)

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

// TestConnectionPoolDefaults_Postgres documents the default database/sql pool
// settings after the pgx/v5 upgrade. tyk-pump intentionally does not call
// SetMaxOpenConns in Init, so MaxOpenConnections must remain 0 (unlimited);
// the idle pool should also stay bounded by the stdlib default.
//
// Verifies: SW-REQ-040
// SW-REQ-040:connection_leak_free:nominal
// MCDC SW-REQ-040: day_sliced_routing=F, table_sharding=F => TRUE
func TestConnectionPoolDefaults_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.SQLTable)

	sqlDB, err := pmp.db.DB()
	if err != nil {
		t.Fatalf("pmp.db.DB() failed: %v", err)
	}

	stats := sqlDB.Stats()
	assert.Equal(t, 0, stats.MaxOpenConnections,
		"tyk-pump does not configure MaxOpenConns; driver default must remain 0 (unlimited)")

	conns := make([]*gorm.DB, 5)
	for i := range conns {
		conns[i] = pmp.db.Session(&gorm.Session{NewDB: true})
		var one int
		if err := conns[i].Raw("SELECT 1").Scan(&one).Error; err != nil {
			t.Fatalf("warm-up query %d failed: %v", i, err)
		}
	}
	stats = sqlDB.Stats()
	assert.LessOrEqual(t, stats.Idle, 2,
		"idle connections should not exceed stdlib default MaxIdleConns=2; got %d", stats.Idle)
}

// Verifies: SW-REQ-041
// Verifies: SW-REQ-067
// SW-REQ-041:nominal:nominal
// SW-REQ-041:concurrent:nominal
// SW-REQ-041:concurrent:race
// MCDC SW-REQ-041: day_sliced_routing=F, table_sharding=F => TRUE
// SW-REQ-067:concurrent:nominal
// SW-REQ-067:concurrent:race
// MCDC SW-REQ-067: on_conflict_assignments_applied=T, row_conflict_detected=T => TRUE
func TestConcurrentAggregateWrites_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLAggregatePump{}
	require.NoError(t, pmp.Init(newSQLConfig(false)))
	cleanupGormDB(t, pmp.db, analytics.AggregateSQLTable)

	sqlDB, err := pmp.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(5)

	const (
		workers          = 12
		recordsPerWorker = 4
	)
	now := time.Now().UTC()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			keys := make([]interface{}, recordsPerWorker)
			for j := 0; j < recordsPerWorker; j++ {
				keys[j] = analytics.AnalyticsRecord{
					APIID:        "concurrent-agg-api",
					OrgID:        "concurrent-agg-org",
					ResponseCode: http.StatusOK,
					TimeStamp:    now,
				}
			}
			if writeErr := pmp.WriteData(context.Background(), keys); writeErr != nil {
				mu.Lock()
				errs = append(errs, writeErr)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Empty(t, errs, "aggregate writes should not fail under concurrent use")

	var count int64
	require.NoError(t, pmp.db.Table(analytics.AggregateSQLTable).
		Where("org_id = ?", "concurrent-agg-org").
		Count(&count).Error)
	assert.Equal(t, int64(2), count,
		"concurrent writes to the same aggregate key should upsert apiid and total rows in place")

	var total analytics.SQLAnalyticsRecordAggregate
	require.NoError(t, pmp.db.Table(analytics.AggregateSQLTable).
		Where("org_id = ? AND dimension_value = ?", "concurrent-agg-org", "total").
		First(&total).Error)
	assert.Equal(t, workers*recordsPerWorker, total.Hits,
		"concurrent ON CONFLICT updates should accumulate hits instead of overwriting")
}

// Verifies: SW-REQ-045
// SW-REQ-045:concurrent:nominal
// SW-REQ-045:concurrent:race
// SW-REQ-045:connection_leak_free:nominal
// MCDC SW-REQ-045: minute_window_used=F, store_per_minute=F => TRUE
func TestConcurrentMCPSQLAggregateWrites_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := MCPSQLAggregatePump{}
	require.NoError(t, pmp.Init(SQLAggregatePumpConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
		},
	}))
	waitForAggregateIndexReady(t, pmp.db, analytics.AggregateMCPSQLTable, pmp.backgroundIndexCreated)
	cleanupGormDB(t, pmp.db, analytics.AggregateMCPSQLTable)

	sqlDB, err := pmp.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(5)

	const (
		workers          = 12
		recordsPerWorker = 4
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
			now := time.Now().UTC()
			keys := make([]interface{}, recordsPerWorker)
			for j := 0; j < recordsPerWorker; j++ {
				keys[j] = analytics.AnalyticsRecord{
					APIID:        fmt.Sprintf("concurrent-mcp-agg-api-%d", workerID),
					APIName:      fmt.Sprintf("concurrent-mcp-agg-api-%d", workerID),
					OrgID:        fmt.Sprintf("concurrent-mcp-agg-org-%d", workerID),
					ResponseCode: http.StatusOK,
					TimeStamp:    now,
					MCPStats: analytics.MCPStats{
						IsMCP:         true,
						JSONRPCMethod: fmt.Sprintf("tools/call-%d", workerID),
						PrimitiveType: "tool",
						PrimitiveName: fmt.Sprintf("tool-%d", workerID),
					},
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
	assert.Empty(t, errs, "MCP aggregate writes should not fail under concurrent use")

	var count int64
	require.NoError(t, pmp.db.Table(analytics.AggregateMCPSQLTable).
		Where("org_id LIKE ?", "concurrent-mcp-agg-org-%").
		Count(&count).Error)
	assert.Equal(t, int64(workers*5), count,
		"each worker should produce apiid, total, methods, primitives, and names rows")

	stats := sqlDB.Stats()
	assert.Equal(t, 0, stats.InUse,
		"all connections should be returned to pool after concurrent MCP aggregate writes")
}

// ── 5. Sharded Table Lifecycle ────────────────────────────────────────────────

// TestShardedTableLifecycle_Postgres writes records spanning 5 different dates, verifies
// that 5 sharded tables are created with all 6 expected indexes, then runs
// MigrateAllShardedTables and confirms no ALTER TABLE is emitted.
// Verifies: SW-REQ-040
// SW-REQ-040:per_shard_index_created:nominal
// MCDC SW-REQ-040: day_sliced_routing=T, table_sharding=T => TRUE
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
// SW-REQ-067: Postgres duplicate-key translation for SQL upsert/error handling.
func TestDuplicateKeyError_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{IsUptime: true}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.UptimeSQLTable)

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
//
// Verifies: SW-REQ-040
// MCDC SW-REQ-040: day_sliced_routing=F, table_sharding=F => TRUE
func TestPreferSimpleProtocol_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	cfg := newSQLConfig(false)
	cfg["postgres"] = map[string]interface{}{"prefer_simple_protocol": true}

	pmp := SQLPump{}
	if err := pmp.Init(cfg); err != nil {
		t.Fatalf("Init with prefer_simple_protocol failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.SQLTable)

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

// ── 8. Nullable / empty text columns (pgx v5 pgtype changes) ──────────────────

// TestNullableColumns_Postgres writes a record whose optional text fields (APIKey,
// OauthID, Alias, RawRequest, RawResponse) are empty strings and verifies round-trip.
// pgx/v5 reworked pgtype NULL handling; this pins down that empty Go strings stay as
// empty strings in Postgres text columns rather than being coerced to NULL or errored.
//
// Verifies: INT-REQ-006
// MCDC INT-REQ-006: mapping_per_implementation=T, record_dispatched_to_backend=T => TRUE
func TestNullableColumns_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.SQLTable)

	rec := analytics.AnalyticsRecord{
		APIID:     "nullable-cols-test",
		OrgID:     "nullable-test-pgxv5",
		TimeStamp: time.Now(),
		// All of these intentionally left as zero-value empty strings:
		// APIKey, OauthID, Alias, RawRequest, RawResponse, IPAddress, Method, Host.
	}
	if err := pmp.WriteData(context.Background(), []interface{}{rec}); err != nil {
		t.Fatalf("WriteData with empty text fields failed: %v", err)
	}

	var got analytics.AnalyticsRecord
	result := pmp.db.Table(analytics.SQLTable).Where("apiid = ?", "nullable-cols-test").First(&got)
	assert.NoError(t, result.Error)
	assert.Equal(t, "", got.APIKey)
	assert.Equal(t, "", got.OauthID)
	assert.Equal(t, "", got.Alias)
	assert.Equal(t, "", got.RawRequest)
	assert.Equal(t, "", got.RawResponse)
	assert.Equal(t, "", got.IPAddress)
}

// ── 10. Time encoding (pgx v5 timestamp path) ─────────────────────────────────

// TestTimeHandling_Postgres round-trips timestamps in UTC, a non-UTC zone, and with
// sub-millisecond (microsecond) precision. pgx/v5 refactored timestamp text/binary
// encoding; this verifies no precision loss or zone drift under the pump's default
// extended-protocol path.
//
// Verifies: INT-REQ-006
// MCDC INT-REQ-006: mapping_per_implementation=T, record_dispatched_to_backend=T => TRUE
func TestTimeHandling_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.SQLTable)

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("load tz: %v", err)
	}

	cases := []struct {
		name string
		ts   time.Time
	}{
		{"UTC", time.Date(2099, 5, 10, 12, 0, 0, 0, time.UTC)},
		{"Tokyo", time.Date(2099, 5, 10, 21, 0, 0, 0, tokyo)},
		{"Microsecond", time.Date(2099, 5, 10, 12, 0, 0, 123456000, time.UTC)}, // 123456 µs
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rec := analytics.AnalyticsRecord{
				APIID:     "time-" + tc.name,
				OrgID:     "time-test-pgxv5",
				TimeStamp: tc.ts,
			}
			if err := pmp.WriteData(context.Background(), []interface{}{rec}); err != nil {
				t.Fatalf("WriteData failed: %v", err)
			}

			var got analytics.AnalyticsRecord
			result := pmp.db.Table(analytics.SQLTable).Where("apiid = ?", "time-"+tc.name).First(&got)
			assert.NoError(t, result.Error)
			// Postgres stores timestamps in UTC internally; compare as UTC instants.
			assert.True(t, tc.ts.Equal(got.TimeStamp),
				"timestamp drift: wrote %v, read %v", tc.ts, got.TimeStamp)
		})
	}
}

// ── 11. Large string columns (pgx v5 text encoding) ───────────────────────────

// TestLargePayload_Postgres writes a record whose RawRequest and RawResponse are 1 MB
// each and reads them back unchanged. Guards against any regression in pgx/v5's text
// column encoding that could truncate or corrupt large payloads.
//
// Verifies: INT-REQ-006
// MCDC INT-REQ-006: mapping_per_implementation=T, record_dispatched_to_backend=T => TRUE
func TestLargePayload_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	if err := pmp.Init(newSQLConfig(false)); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanupGormDB(t, pmp.db, analytics.SQLTable)

	const size = 1 << 20 // 1 MB
	payload := strings.Repeat("x", size)
	rec := analytics.AnalyticsRecord{
		APIID:       "large-payload-test",
		OrgID:       "large-payload-pgxv5",
		TimeStamp:   time.Now(),
		RawRequest:  payload,
		RawResponse: payload,
	}
	if err := pmp.WriteData(context.Background(), []interface{}{rec}); err != nil {
		t.Fatalf("WriteData with 1MB payload failed: %v", err)
	}

	var got analytics.AnalyticsRecord
	result := pmp.db.Table(analytics.SQLTable).Where("apiid = ?", "large-payload-test").First(&got)
	assert.NoError(t, result.Error)
	assert.Equal(t, size, len(got.RawRequest), "RawRequest length must match")
	assert.Equal(t, size, len(got.RawResponse), "RawResponse length must match")
	assert.Equal(t, payload, got.RawRequest, "RawRequest content must round-trip unchanged")
}

// Verifies: SW-REQ-040
// MCDC SW-REQ-040: day_sliced_routing=F, table_sharding=F => TRUE
func TestSQLWriteData_PreferSimpleProtocol_Month(t *testing.T) {
	skipTestIfNoPostgres(t)

	pmp := SQLPump{}
	cfg := newSQLConfig(false)
	cfg["postgres"] = map[string]interface{}{"prefer_simple_protocol": true}

	err := pmp.Init(cfg)
	if err != nil {
		t.Fatal("SQL Pump couldn't be initialized with err:", err)
	}

	defer func() {
		require.NoError(t, pmp.db.Migrator().DropTable(analytics.SQLTable))
	}()

	rec := analytics.AnalyticsRecord{
		APIID:     "api-simple-proto",
		OrgID:     "org-simple-proto",
		TimeStamp: time.Now(),
		Month:     time.May,
	}

	errWrite := pmp.WriteData(context.TODO(), []interface{}{rec})
	if errWrite != nil {
		t.Fatal("SQL Pump couldn't write records with err:", errWrite)
	}

	var dbRecords []analytics.AnalyticsRecord
	err = pmp.db.Table(analytics.SQLTable).Find(&dbRecords).Error
	if err != nil {
		t.Fatal("couldn't read records back:", err)
	}

	if assert.Equal(t, 1, len(dbRecords), "expected 1 record in DB -- insert likely failed due to pgx v5 time.Month encoding bug") {
		assert.Equal(t, time.May, dbRecords[0].Month, "month should round-trip as integer 5, not a string")
	}
}
