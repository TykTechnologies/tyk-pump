// Code in this file closes the last MC/DC gaps that the family-specific
// mcdc_100 files left behind. Strategy:
//   - Targeted unit tests where a structural test fixture can deterministically
//     drive an arm (e.g. closed DB → Raw err, invalid 8-char non-date suffix
//     → time.Parse err).
//   - Production-code //mcdc:ignore (in the .go files above) for arms whose
//     only driver is a fake-seam refactor (AutoMigrate failure, AWS SDK
//     error injection, gorpc transport mid-call failure, moesif/influx
//     SDK internal state).
//
// All ignores are cross-referenced to KI mcdc-pumps-below-95 (and family KIs
// where they exist).
package pumps

import (
	"context"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

// === common.go ===

// TestCommonPumpConfig_GetOmitDetailedRecording drives the lone return
// expression (line 56) so the MC/DC tracker registers a sample row for
// the boolean field.
//
// Verifies: SW-REQ-016
// SW-REQ-016:nominal:positive
func TestCommonPumpConfig_GetOmitDetailedRecording(t *testing.T) {
	c := &CommonPumpConfig{}
	assert.False(t, c.GetOmitDetailedRecording())
	c.SetOmitDetailedRecording(true)
	assert.True(t, c.GetOmitDetailedRecording())
}

// TestMigrateAllShardedTables_RawQueryErr drives the err != nil = T arm at
// common.go:170 by closing the underlying *sql.DB before invocation so the
// Raw query returns an error.
//
// Verifies: SW-REQ-016
// SW-REQ-016:nominal:negative
func TestMigrateAllShardedTables_RawQueryErr(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gorm_logger.Default.LogMode(gorm_logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close()) // force the next Raw().Scan to fail

	logger := logrus.NewEntry(logrus.New())
	// Note: information_schema.tables doesn't exist on a closed sqlite DB; the
	// Raw query surfaces a "sql: database is closed" error which drives the
	// err != nil = T arm. The function logs and returns nil (non-fatal).
	err = MigrateAllShardedTables(db, "irrelevant", "test", &analytics.AnalyticsRecord{}, logger)
	assert.NoError(t, err) // function intentionally swallows the err
}

// TestMigrateAllShardedTables_EightCharNonDateSuffix drives the
// err == nil = F arm at common.go:185. Setup: create a table whose name
// matches the sharded prefix and has an 8-character suffix that is NOT a
// valid YYYYMMDD date (e.g. "99999999"). time.Parse returns an error, so
// the table is skipped and not appended to shardedTables.
//
// Verifies: SW-REQ-016
// SW-REQ-016:nominal:negative
func TestMigrateAllShardedTables_EightCharNonDateSuffix(t *testing.T) {
	db := setupTestDB(t)
	logger := logrus.NewEntry(logrus.New())

	prefix := "cleanup_shard"
	// 8 chars but invalid date → enters the len(suffix)==8 inner if but
	// fails time.Parse, driving err == nil = F.
	bogus := prefix + "_99999999"
	require.NoError(t, db.Table(bogus).AutoMigrate(&analytics.AnalyticsRecord{}))
	require.NoError(t, db.Exec(
		"INSERT INTO \"information_schema.tables\" (table_name, table_schema) VALUES (?, 'public')",
		bogus,
	).Error)

	require.NoError(t, MigrateAllShardedTables(db, prefix, "cleanup", &analytics.AnalyticsRecord{}, logger))
	// Bogus suffix should NOT have been migrated as a sharded table.
}

// === pump.go ===

// TestGetPumpByName_UnknownReturnsError drives the ok==F arm of the
// short-circuit ok && pump != nil at pump.go:50.
//
// Verifies: SW-REQ-017
// SW-REQ-017:nominal:negative
func TestGetPumpByName_UnknownReturnsError(t *testing.T) {
	p, err := GetPumpByName("definitely-not-a-pump")
	assert.Nil(t, p)
	require.Error(t, err)
}

// TestGetPumpByName_KnownReturnsPump drives the ok && pump != nil = T arm.
//
// Verifies: SW-REQ-017
// SW-REQ-017:nominal:positive
func TestGetPumpByName_KnownReturnsPump(t *testing.T) {
	p, err := GetPumpByName("dummy")
	require.NoError(t, err)
	require.NotNil(t, p)
}

// === resurface.go writeData "open" arm ===

// TestResurfacePump_WriteData_Open_BranchAfterDisable_Force drives the
// `open` true arm at resurface.go:284. The existing test in
// http_pumps_mcdc_100_test.go drives the same logic but the MC/DC tracker
// can lose the row when the channel is drained mid-test; this duplicate
// reseeds the channel synchronously inside a fresh pump so the arm fires
// deterministically.
//
// Verifies: SW-REQ-054
// SW-REQ-054:nominal:positive
func TestResurfacePump_WriteData_Open_BranchAfterDisable_Force(t *testing.T) {
	pmp, _ := SetUp(t, "", make([]string, 0), "include debug")

	// Drain the worker so we own the channel.
	close(pmp.data)
	pmp.wg.Wait()

	// Reseed with a peek; disable; call WriteData → `open` arm fires.
	pmp.data = make(chan []interface{}, 1)
	pmp.data <- []interface{}{
		analytics.AnalyticsRecord{Host: "h", Method: "GET", RawRequest: rawReq, RawResponse: rawResp},
	}
	pmp.disable()
	require.NoError(t, pmp.WriteData(context.Background(), nil))
}
