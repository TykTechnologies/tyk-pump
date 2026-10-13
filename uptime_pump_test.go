package main

import (
	"os"
	"testing"

	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitialiseUptimePump covers the uptime pump's initialisation outcomes. The
// failure case matters because a pump whose Init returned an error is only
// half-constructed: writing to it dereferences a nil DB handle, and in the
// non-sharded branch loops forever on a zero batch size. It must be dropped, not kept.
func TestInitialiseUptimePump(t *testing.T) {
	original := SystemConfig
	t.Cleanup(func() {
		SystemConfig = original
		UptimePump = nil
	})

	t.Run("invalid connection pool config drops the pump", func(t *testing.T) {
		UptimePump = nil
		SystemConfig = TykPumpConfiguration{}
		SystemConfig.UptimePumpConfig.UptimeType = "sql"
		SystemConfig.UptimePumpConfig.SQLConf = pumps.SQLConf{
			Type:             "postgres",
			ConnectionString: "host=localhost port=9920 user=gorm password=gorm dbname=gorm sslmode=disable",
			// Rejected before any connection is attempted, so this needs no database.
			Postgres: pumps.PostgresConfig{ConnectionMaxLifetime: "30 minutes"},
		}

		initialiseUptimePump()

		assert.Nil(t, UptimePump, "a pump that failed to initialise must not be kept")
	})

	t.Run("unsupported sql type drops the pump", func(t *testing.T) {
		UptimePump = nil
		SystemConfig = TykPumpConfiguration{}
		SystemConfig.UptimePumpConfig.UptimeType = "sql"
		SystemConfig.UptimePumpConfig.SQLConf = pumps.SQLConf{Type: "not-a-database"}

		initialiseUptimePump()

		assert.Nil(t, UptimePump)
	})

	t.Run("negative pool value drops the pump", func(t *testing.T) {
		UptimePump = nil
		SystemConfig = TykPumpConfiguration{}
		SystemConfig.UptimePumpConfig.UptimeType = "sql"
		SystemConfig.UptimePumpConfig.SQLConf = pumps.SQLConf{
			Type:             "postgres",
			ConnectionString: "host=localhost port=9920 user=gorm password=gorm dbname=gorm sslmode=disable",
			Postgres:         pumps.PostgresConfig{MaxOpenConnections: -1},
		}

		initialiseUptimePump()

		assert.Nil(t, UptimePump)
	})
}

// TestInitialiseUptimePumpSQL checks the success path end to end against a live
// database, including that the configured pool limit reaches the uptime pump's own
// connection pool — it is a seventh pool, separate from every analytics pump.
func TestInitialiseUptimePumpSQL(t *testing.T) {
	dsn := os.Getenv("TYK_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("Skipping test because TYK_TEST_POSTGRES environment variable is not set")
	}

	original := SystemConfig
	t.Cleanup(func() {
		SystemConfig = original
		UptimePump = nil
	})

	UptimePump = nil
	SystemConfig = TykPumpConfiguration{}
	SystemConfig.UptimePumpConfig.UptimeType = "sql"
	SystemConfig.UptimePumpConfig.SQLConf = pumps.SQLConf{
		Type:             "postgres",
		ConnectionString: dsn,
		Postgres:         pumps.PostgresConfig{MaxOpenConnections: 3},
	}

	initialiseUptimePump()

	require.NotNil(t, UptimePump)
	sqlPump, ok := UptimePump.(*pumps.SQLPump)
	require.True(t, ok)
	assert.True(t, sqlPump.IsUptime)
	assert.Equal(t, 3, sqlPump.SQLConf.Postgres.MaxOpenConnections)
}
