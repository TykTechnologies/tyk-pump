package pumps

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The uptime pumps are the Pinger implementations main probes for
// tyk.pump.health.
var (
	_ Pinger = (*MongoPump)(nil)
	_ Pinger = (*SQLPump)(nil)
)

func pingCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestMongoPump_Ping(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		pmp := &MongoPump{IsUptime: true}
		conf := defaultConf()
		require.NoError(t, pmp.Init(conf))

		assert.NoError(t, pmp.Ping(pingCtx(t)))
		assert.Equal(t, "mongo", pmp.StoreName())
	})

	t.Run("not connected", func(t *testing.T) {
		pmp := &MongoPump{IsUptime: true}
		assert.Error(t, pmp.Ping(pingCtx(t)))
		assert.Equal(t, "mongo", pmp.StoreName())
	})
}

// TestSQLPump_Ping_InitFailed covers the uptime pump whose Init error main
// ignores: no handle, so never healthy, and the store label falls back to the
// configured type. It needs no database.
func TestSQLPump_Ping_InitFailed(t *testing.T) {
	pmp := &SQLPump{IsUptime: true}
	err := pmp.Init(map[string]interface{}{
		"type":              "postgres",
		"connection_string": "host=127.0.0.1 port=1 user=nobody dbname=none sslmode=disable connect_timeout=1",
	})
	require.Error(t, err, "nothing listens on port 1")

	assert.Error(t, pmp.Ping(pingCtx(t)))
	assert.Equal(t, "postgres", pmp.StoreName())

	var zero SQLPump
	assert.Error(t, zero.Ping(pingCtx(t)), "a pump that was never initialised is not healthy")
	assert.Equal(t, "sql", zero.StoreName(), "the store label is never empty")
}

func TestSQLPump_Ping_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)
	testSQLPumpPing(t, newSQLConfig(false), "postgres")
}

func TestSQLPump_Ping_MySQL(t *testing.T) {
	skipTestIfNoMySQL(t)
	testSQLPumpPing(t, newMySQLConfig(false), "mysql")
}

func testSQLPumpPing(t *testing.T, cfg map[string]interface{}, store string) {
	t.Helper()
	pmp := &SQLPump{IsUptime: true}
	require.NoError(t, pmp.Init(cfg))

	assert.NoError(t, pmp.Ping(pingCtx(t)))
	assert.Equal(t, store, pmp.StoreName(), "store comes from the live dialect")

	// WriteUptimeData reassigns db to a table-scoped session; the probe must
	// keep pinging the same pool.
	pmp.db = pmp.db.Table("tyk_uptime_analytics")
	assert.NoError(t, pmp.Ping(pingCtx(t)))

	sqlDB, err := pmp.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	assert.Error(t, pmp.Ping(pingCtx(t)), "a closed pool must not read healthy even though the handle is non-nil")
	assert.Equal(t, store, pmp.StoreName())
}
