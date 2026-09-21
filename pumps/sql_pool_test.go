package pumps

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	logrus_test "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeConnector yields a *sql.DB that never dials a database. The pool settings under
// test are stored on the *sql.DB itself, so no live connection is needed to observe
// them via Stats(). Uses only the standard library, so these tests also pass with
// CGO_ENABLED=0.
type fakeConnector struct{}

func (fakeConnector) Connect(context.Context) (driver.Conn, error) { return nil, driver.ErrBadConn }

func (fakeConnector) Driver() driver.Driver { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, driver.ErrBadConn }

// fakeDBProvider satisfies sqlDBProvider with a pre-built *sql.DB or a canned error.
type fakeDBProvider struct {
	db  *sql.DB
	err error
}

func (f fakeDBProvider) DB() (*sql.DB, error) { return f.db, f.err }

func newPoolTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := sql.OpenDB(fakeConnector{})
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newPoolTestLogger() (*logrus.Entry, *logrus_test.Hook) {
	logger, hook := logrus_test.NewNullLogger()
	return logrus.NewEntry(logger), hook
}

func TestApplyConnectionPoolSettings(t *testing.T) {
	t.Run("configured values are applied", func(t *testing.T) {
		db := newPoolTestDB(t)
		entry, _ := newPoolTestLogger()

		err := applyConnectionPoolSettings(db, PostgresConfig{
			MaxOpenConnections:    7,
			MaxIdleConnections:    3,
			ConnectionMaxLifetime: "30m",
			ConnectionMaxIdleTime: "5m",
		}, entry)

		require.NoError(t, err)
		assert.Equal(t, 7, db.Stats().MaxOpenConnections)
	})

	t.Run("zero and empty values are no-ops", func(t *testing.T) {
		db := newPoolTestDB(t)
		entry, _ := newPoolTestLogger()

		err := applyConnectionPoolSettings(db, PostgresConfig{}, entry)

		require.NoError(t, err)
		// 0 == unlimited: the database/sql default is preserved.
		assert.Equal(t, 0, db.Stats().MaxOpenConnections)
	})

	t.Run("invalid lifetime duration errors", func(t *testing.T) {
		db := newPoolTestDB(t)
		entry, _ := newPoolTestLogger()

		err := applyConnectionPoolSettings(db, PostgresConfig{ConnectionMaxLifetime: "not-a-duration"}, entry)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection_max_lifetime")
	})

	t.Run("invalid idle time duration errors", func(t *testing.T) {
		db := newPoolTestDB(t)
		entry, _ := newPoolTestLogger()

		err := applyConnectionPoolSettings(db, PostgresConfig{ConnectionMaxIdleTime: "3 hours"}, entry)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection_max_idle_time")
	})

	t.Run("idle greater than open warns and still applies", func(t *testing.T) {
		db := newPoolTestDB(t)
		entry, hook := newPoolTestLogger()

		err := applyConnectionPoolSettings(db, PostgresConfig{
			MaxOpenConnections: 2,
			MaxIdleConnections: 10,
		}, entry)

		require.NoError(t, err)
		assert.Equal(t, 2, db.Stats().MaxOpenConnections)
		require.Len(t, hook.Entries, 1)
		assert.Equal(t, logrus.WarnLevel, hook.Entries[0].Level)
		assert.Contains(t, hook.Entries[0].Message, "max_idle_connections")
	})

	t.Run("a nil logger is tolerated", func(t *testing.T) {
		db := newPoolTestDB(t)

		err := applyConnectionPoolSettings(db, PostgresConfig{
			MaxOpenConnections: 2,
			MaxIdleConnections: 10,
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, 2, db.Stats().MaxOpenConnections)
	})
}

func TestApplyPoolSettings(t *testing.T) {
	entry, _ := newPoolTestLogger()

	t.Run("postgres applies the settings", func(t *testing.T) {
		db := newPoolTestDB(t)
		conf := &SQLConf{Type: "postgres", Postgres: PostgresConfig{MaxOpenConnections: 7}}

		err := applyPoolSettings(fakeDBProvider{db: db}, conf, entry)

		require.NoError(t, err)
		assert.Equal(t, 7, db.Stats().MaxOpenConnections)
	})

	t.Run("mysql short-circuits before touching the connection", func(t *testing.T) {
		conf := &SQLConf{
			Type: "mysql",
			// Deliberately invalid: it must never be read for a non-postgres type.
			Postgres: PostgresConfig{MaxOpenConnections: 7, ConnectionMaxLifetime: "garbage"},
		}

		err := applyPoolSettings(fakeDBProvider{err: errors.New("DB() should not be called")}, conf, entry)

		assert.NoError(t, err)
	})

	t.Run("a DB() error is propagated", func(t *testing.T) {
		wantErr := errors.New("no underlying connection")
		conf := &SQLConf{Type: "postgres", Postgres: PostgresConfig{MaxOpenConnections: 7}}

		err := applyPoolSettings(fakeDBProvider{err: wantErr}, conf, entry)

		assert.ErrorIs(t, err, wantErr)
	})
}

func TestPostgresConfigValidate(t *testing.T) {
	t.Run("empty config is valid", func(t *testing.T) {
		assert.NoError(t, PostgresConfig{}.Validate())
	})

	t.Run("valid durations pass", func(t *testing.T) {
		assert.NoError(t, PostgresConfig{
			ConnectionMaxLifetime: "30m",
			ConnectionMaxIdleTime: "5m",
		}.Validate())
	})

	t.Run("invalid lifetime is rejected", func(t *testing.T) {
		err := PostgresConfig{ConnectionMaxLifetime: "30 minutes"}.Validate()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection_max_lifetime")
		assert.Contains(t, err.Error(), "30 minutes")
	})

	t.Run("invalid idle time is rejected", func(t *testing.T) {
		err := PostgresConfig{ConnectionMaxIdleTime: "5 mins"}.Validate()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection_max_idle_time")
	})

	// database/sql accepts negatives but inverts their meaning: SetMaxOpenConns(-1)
	// is unlimited and a negative lifetime never expires, so an operator who thinks
	// they capped the pool would silently get no cap at all.
	negatives := map[string]PostgresConfig{
		"max_open_connections":     {MaxOpenConnections: -1},
		"max_idle_connections":     {MaxIdleConnections: -1},
		"connection_max_lifetime":  {ConnectionMaxLifetime: "-30m"},
		"connection_max_idle_time": {ConnectionMaxIdleTime: "-5m"},
	}
	for field, cfg := range negatives {
		t.Run("negative "+field+" is rejected", func(t *testing.T) {
			err := cfg.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), field)
			assert.Contains(t, err.Error(), "negative")
		})
	}
}

// TestOpenGormDBRejectsNegativePoolValues covers the full Init path: a negative value
// must fail before a connection is attempted, so this needs no database.
func TestOpenGormDBRejectsNegativePoolValues(t *testing.T) {
	logger := setupTestLogger(t)
	conf := &SQLConf{
		Type:             "postgres",
		ConnectionString: "host=localhost port=9920 user=gorm password=gorm dbname=gorm sslmode=disable",
		Postgres:         PostgresConfig{MaxOpenConnections: -1},
	}

	db, err := OpenGormDB(conf, logger)

	require.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "max_open_connections")
}

// poolTestMeta is the pump `meta` block used by the live pool tests. Supplying it as a
// map rather than a struct literal exercises the mapstructure tags, which is the path
// a real pump.conf takes.
func poolTestMeta() map[string]interface{} {
	return map[string]interface{}{
		"type":              "postgres",
		"connection_string": getTestPostgresConnectionString(),
		"table_sharding":    false,
		"postgres": map[string]interface{}{
			"max_open_connections":     3,
			"max_idle_connections":     1,
			"connection_max_lifetime":  "30m",
			"connection_max_idle_time": "5m",
		},
	}
}

// TestConnectionPoolSettings_Postgres_Live asserts that every SQL pump honours the
// configured connection-pool limits. The GraphSQLAggregatePump case is the important
// one: it used to open gorm inline instead of going through OpenGormDB, so anyone who
// reintroduces a bespoke open path there will fail here.
func TestConnectionPoolSettings_Postgres_Live(t *testing.T) {
	skipTestIfNoPostgres(t)

	cases := []struct {
		name   string
		open   func(t *testing.T, conf map[string]interface{}) *gorm.DB
		tables []string
	}{
		{
			name: "sql",
			open: func(t *testing.T, conf map[string]interface{}) *gorm.DB {
				p := &SQLPump{}
				require.NoError(t, p.Init(conf))
				return p.db
			},
			tables: []string{analytics.SQLTable},
		},
		{
			name: "sql_aggregate",
			open: func(t *testing.T, conf map[string]interface{}) *gorm.DB {
				p := &SQLAggregatePump{}
				require.NoError(t, p.Init(conf))
				return p.db
			},
			tables: []string{analytics.AggregateSQLTable},
		},
		{
			name: "sql_graph",
			open: func(t *testing.T, conf map[string]interface{}) *gorm.DB {
				p := &GraphSQLPump{}
				require.NoError(t, p.Init(conf))
				return p.db
			},
			tables: []string{GraphSQLTable},
		},
		{
			name: "sql_graph_aggregate",
			open: func(t *testing.T, conf map[string]interface{}) *gorm.DB {
				p := &GraphSQLAggregatePump{}
				require.NoError(t, p.Init(conf))
				return p.db
			},
			tables: []string{analytics.AggregateGraphSQLTable},
		},
		{
			name: "sql_mcp",
			open: func(t *testing.T, conf map[string]interface{}) *gorm.DB {
				p := &MCPSQLPump{}
				require.NoError(t, p.Init(conf))
				return p.db
			},
			tables: []string{MCPSQLTable},
		},
		{
			name: "sql_mcp_aggregate",
			open: func(t *testing.T, conf map[string]interface{}) *gorm.DB {
				p := &MCPSQLAggregatePump{}
				require.NoError(t, p.Init(conf))
				return p.db
			},
			tables: []string{analytics.AggregateMCPSQLTable},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.open(t, poolTestMeta())
			require.NotNil(t, db)
			t.Cleanup(func() {
				for _, table := range tc.tables {
					_ = db.Migrator().DropTable(table)
				}
			})

			sqlDB, err := db.DB()
			require.NoError(t, err)
			assert.Equal(t, 3, sqlDB.Stats().MaxOpenConnections,
				"%s pump must honour postgres.max_open_connections", tc.name)
		})
	}
}

// TestConnectionPoolSettingsFromEnv asserts the four options can be supplied as
// environment variables. envconfig derives the names from the Go field names, not the
// json/mapstructure tags, so the nested PostgresConfig fields appear as
// TYK_PMP_PUMPS_SQL_META_POSTGRES_<FIELDNAME>.
func TestConnectionPoolSettingsFromEnv(t *testing.T) {
	t.Setenv("TYK_PMP_PUMPS_SQL_META_POSTGRES_MAXOPENCONNECTIONS", "7")
	t.Setenv("TYK_PMP_PUMPS_SQL_META_POSTGRES_MAXIDLECONNECTIONS", "2")
	t.Setenv("TYK_PMP_PUMPS_SQL_META_POSTGRES_CONNECTIONMAXLIFETIME", "30m")
	t.Setenv("TYK_PMP_PUMPS_SQL_META_POSTGRES_CONNECTIONMAXIDLETIME", "5m")

	pmp := &SQLPump{SQLConf: &SQLConf{}}
	pmp.log = log.WithField("prefix", SQLPrefix)

	processPumpEnvVars(pmp, pmp.log, pmp.SQLConf, SQLDefaultENV)

	assert.Equal(t, 7, pmp.SQLConf.Postgres.MaxOpenConnections)
	assert.Equal(t, 2, pmp.SQLConf.Postgres.MaxIdleConnections)
	assert.Equal(t, "30m", pmp.SQLConf.Postgres.ConnectionMaxLifetime)
	assert.Equal(t, "5m", pmp.SQLConf.Postgres.ConnectionMaxIdleTime)
}

// TestConnectionPoolCapUnderLoad_Postgres proves the configured cap is enforced against
// a live server: the pool is bounded by configuration alone (no direct SetMaxOpenConns
// call), and the observed number of open connections never exceeds it while concurrent
// writers contend for the pool.
func TestConnectionPoolCapUnderLoad_Postgres(t *testing.T) {
	skipTestIfNoPostgres(t)

	const maxOpen = 3

	conf := poolTestMeta()
	conf["postgres"].(map[string]interface{})["max_open_connections"] = maxOpen

	pmp := &SQLPump{}
	require.NoError(t, pmp.Init(conf))
	t.Cleanup(func() { _ = pmp.db.Migrator().DropTable(analytics.SQLTable) })

	sqlDB, err := pmp.db.DB()
	require.NoError(t, err)
	require.Equal(t, maxOpen, sqlDB.Stats().MaxOpenConnections)

	const (
		workers          = 20
		recordsPerWorker = 20
	)

	// Sample the pool while the writers contend for it.
	done := make(chan struct{})
	observed := make(chan int, 1)
	go func() {
		peak := 0
		for {
			select {
			case <-done:
				observed <- peak
				return
			default:
				if open := sqlDB.Stats().OpenConnections; open > peak {
					peak = open
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()

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
			records := make([]interface{}, recordsPerWorker)
			for j := 0; j < recordsPerWorker; j++ {
				records[j] = analytics.AnalyticsRecord{
					APIID:     fmt.Sprintf("pool-cap-api-%d-%d", workerID, j),
					OrgID:     "pool-cap-test",
					TimeStamp: now,
				}
			}
			if writeErr := pmp.WriteData(context.Background(), records); writeErr != nil {
				mu.Lock()
				errs = append(errs, writeErr)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(done)
	peak := <-observed

	t.Logf("peak open connections observed: %d (cap %d)", peak, maxOpen)
	assert.Empty(t, errs, "no errors expected while the pool is capped by configuration")
	assert.LessOrEqual(t, peak, maxOpen, "the pool must never exceed max_open_connections")

	var count int64
	pmp.db.Table(analytics.SQLTable).Where("orgid = ?", "pool-cap-test").Count(&count)
	assert.Equal(t, int64(workers*recordsPerWorker), count, "all records should still be persisted")
}
