package pumps

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func getTestMySQLConnectionString() string {
	return os.Getenv("TYK_TEST_MYSQL")
}

func skipTestIfNoMySQL(t *testing.T) {
	t.Helper()
	if os.Getenv("TYK_TEST_MYSQL") == "" {
		t.Skip("Skipping test because TYK_TEST_MYSQL environment variable is not set")
	}
}

func newMySQLConfig(sharded bool) map[string]interface{} {
	cfg := make(map[string]interface{})
	cfg["type"] = "mysql"
	cfg["connection_string"] = getTestMySQLConnectionString()
	cfg["table_sharding"] = sharded
	return cfg
}

// ── 1. Basic Init & Write ─────────────────────────────────────────────────────

// TestMySQLInit verifies that the pump initialises correctly against MySQL after the
// driver upgrade (gorm.io/driver/mysql v1.0.3 → v1.3.2, go-sql-driver/mysql v1.5 → v1.6).
func TestMySQLInit(t *testing.T) {
	skipTestIfNoMySQL(t)

	pmp := SQLPump{}
	if err := pmp.Init(newMySQLConfig(false)); err != nil {
		t.Fatalf("MySQL Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	assert.NotNil(t, pmp.db)
	assert.Equal(t, "mysql", pmp.db.Dialector.Name())
	assert.True(t, pmp.db.Migrator().HasTable(analytics.SQLTable),
		"analytics table should exist after Init")
}

// TestMySQLWriteData writes 100 records and verifies count plus data integrity for three
// spot-checked records.
func TestMySQLWriteData(t *testing.T) {
	skipTestIfNoMySQL(t)

	pmp := SQLPump{}
	if err := pmp.Init(newMySQLConfig(false)); err != nil {
		t.Fatalf("MySQL Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	const total = 100
	now := time.Now()
	keys := make([]interface{}, total)
	for i := 0; i < total; i++ {
		keys[i] = analytics.AnalyticsRecord{
			APIID:     fmt.Sprintf("mysql-api-%d", i),
			OrgID:     "mysql-write-test",
			TimeStamp: now,
			ExpireAt:  now.Add(24 * time.Hour),
		}
	}

	if err := pmp.WriteData(context.Background(), keys); err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	var count int64
	pmp.db.Table(analytics.SQLTable).Where("orgid = ?", "mysql-write-test").Count(&count)
	assert.Equal(t, int64(total), count, "all %d records should be persisted", total)

	// Spot-check first, middle, and last records.
	for _, idx := range []int{0, total / 2, total - 1} {
		expectedAPIID := fmt.Sprintf("mysql-api-%d", idx)
		var rec analytics.AnalyticsRecord
		result := pmp.db.Table(analytics.SQLTable).Where("apiid = ?", expectedAPIID).First(&rec)
		assert.NoError(t, result.Error, "record at index %d should be findable", idx)
		assert.Equal(t, expectedAPIID, rec.APIID)
		assert.Equal(t, "mysql-write-test", rec.OrgID)
	}
}

// ── 2. Migration Idempotency ──────────────────────────────────────────────────

// TestMigrationIdempotency_MySQL verifies that the new MigrateColumn() checks
// (Unique / DefaultValue / Comment, added in gorm fork commit c3933cb) do not emit
// spurious ALTER TABLE statements on MySQL.
//
// This is independent of the pgx/v5 upgrade — MySQL was bumped separately
// (gorm.io/driver/mysql v1.0.3 → v1.3.2). MySQL's ColumnType implementation may report
// these metadata fields differently, so the check must be validated for MySQL too.
func TestMigrationIdempotency_MySQL(t *testing.T) {
	skipTestIfNoMySQL(t)

	pmp := SQLPump{}
	if err := pmp.Init(newMySQLConfig(false)); err != nil {
		t.Fatalf("MySQL Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	captureDB, cl := captureSession(pmp.db)
	if err := captureDB.Table(analytics.SQLTable).AutoMigrate(&analytics.AnalyticsRecord{}); err != nil {
		t.Fatalf("second AutoMigrate on MySQL failed: %v", err)
	}
	assert.False(t, cl.hasAlterTable(),
		"second AutoMigrate on MySQL must not emit ALTER TABLE — MigrateColumn() Unique/DefaultValue/Comment checks must not misfire")
}

// ── 3. DB.Connection() Method ─────────────────────────────────────────────────

// TestMySQLConnectionMethod validates the DB.Connection() method cherry-picked in gorm
// fork commit 95e095f, which was required because gorm.io/driver/mysql v1.3.2 calls
// this method internally. If the method is missing or broken, the mysql driver fails
// to initialise or panics on the first real query.
func TestMySQLConnectionMethod(t *testing.T) {
	skipTestIfNoMySQL(t)

	pmp := SQLPump{}
	if err := pmp.Init(newMySQLConfig(false)); err != nil {
		t.Fatalf("MySQL Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	now := time.Now()
	rec := analytics.AnalyticsRecord{
		APIID:     "connection-method-test",
		OrgID:     "mysql-conn-test",
		TimeStamp: now,
		ExpireAt:  now.Add(24 * time.Hour),
	}

	// DB.Connection() executes its callback on a single dedicated connection from the pool,
	// returning it when the callback completes.
	err := pmp.db.Connection(func(tx *gorm.DB) error {
		return tx.Table(analytics.SQLTable).Create(&rec).Error
	})
	assert.NoError(t, err, "DB.Connection() must work with gorm.io/driver/mysql v1.3.2")

	var found analytics.AnalyticsRecord
	pmp.db.Table(analytics.SQLTable).Where("apiid = ?", "connection-method-test").First(&found)
	assert.Equal(t, "connection-method-test", found.APIID,
		"record written via DB.Connection() should be persisted and queryable")
	assert.Equal(t, "mysql-conn-test", found.OrgID)
}

// ── 4. Strict-mode zero-date guardrail ────────────────────────────────────────

// TestMySQLStrictMode_ZeroExpireAt documents the pump's behaviour when an analytics
// record arrives with a zero-value ExpireAt against a strict-mode MySQL (NO_ZERO_DATE,
// on by default since MySQL 5.7). Today the gateway always calls SetExpiry() before
// emitting, so ExpireAt is never zero in production — but this test acts as a
// guardrail: if a future gateway refactor forgets to set it, this test shows the
// failure mode early rather than at a customer site. go-sql-driver/mysql v1.6.0
// tightened zero-date handling further, so this is worth pinning for the upgrade.
func TestMySQLStrictMode_ZeroExpireAt(t *testing.T) {
	skipTestIfNoMySQL(t)

	pmp := SQLPump{}
	if err := pmp.Init(newMySQLConfig(false)); err != nil {
		t.Fatalf("MySQL Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	rec := analytics.AnalyticsRecord{
		APIID:     "zero-expireat-test",
		OrgID:     "mysql-zero-expireat",
		TimeStamp: time.Now(),
		// ExpireAt deliberately omitted — zero time.Time{}.
	}
	err := pmp.WriteData(context.Background(), []interface{}{rec})

	// We do not assert success. Under strict-mode MySQL this will fail with
	// "Error 1292: Incorrect datetime value: '0000-00-00' for column 'expireAt'".
	// Under a loose-mode MySQL it may succeed. The point is to surface the
	// strict-mode behaviour explicitly so operators are aware.
	if err != nil {
		t.Logf("zero ExpireAt rejected by strict-mode MySQL (expected): %v", err)
		assert.Contains(t, err.Error(), "expireAt",
			"error should reference the expireAt column so the cause is obvious")
	} else {
		t.Log("zero ExpireAt accepted — MySQL is not in strict mode or NO_ZERO_DATE is disabled")
	}
}

// ── 5. Date / time handling across driver v1.5 → v1.6 ─────────────────────────

// TestMySQLDriverV16_DateHandling round-trips a timestamp through the analytics table
// to confirm that go-sql-driver/mysql v1.6.0's tightened date parsing has not altered
// how tyk-pump's TimeStamp column is written and read. v1.6.0 changed parseTime
// defaults and rejectReadOnly handling; regressions here would show as time drift or
// scan errors on reads.
func TestMySQLDriverV16_DateHandling(t *testing.T) {
	skipTestIfNoMySQL(t)

	pmp := SQLPump{}
	if err := pmp.Init(newMySQLConfig(false)); err != nil {
		t.Fatalf("MySQL Init failed: %v", err)
	}
	t.Cleanup(func() { pmp.db.Migrator().DropTable(analytics.SQLTable) })

	// MySQL DATETIME has second-level precision by default unless the column is
	// declared with a fractional-second precision. Use a second-aligned value to
	// avoid false failures from truncation.
	ts := time.Date(2099, 6, 15, 14, 30, 45, 0, time.UTC)
	rec := analytics.AnalyticsRecord{
		APIID:     "mysql-v16-date-test",
		OrgID:     "mysql-v16-date",
		TimeStamp: ts,
		ExpireAt:  ts.Add(24 * time.Hour),
	}
	if err := pmp.WriteData(context.Background(), []interface{}{rec}); err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	var got analytics.AnalyticsRecord
	result := pmp.db.Table(analytics.SQLTable).Where("apiid = ?", "mysql-v16-date-test").First(&got)
	assert.NoError(t, result.Error)
	assert.True(t, ts.Equal(got.TimeStamp.UTC()),
		"TimeStamp round-trip drifted: wrote %v, read %v", ts, got.TimeStamp)
}
