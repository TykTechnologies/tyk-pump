package pumps

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPSQLAggregatePump_Init(t *testing.T) {
	skipTestIfNoPostgres(t)
	tableName := analytics.AggregateMCPSQLTable
	pump := &MCPSQLAggregatePump{}

	t.Run("successful", func(t *testing.T) {
		conf := SQLAggregatePumpConf{
			SQLConf: SQLConf{
				Type:             "postgres",
				ConnectionString: getTestPostgresConnectionString(),
			},
		}
		require.NoError(t, pump.Init(conf))
		t.Cleanup(func() {
			pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", tableName))
		})
		assert.True(t, pump.db.Migrator().HasTable(tableName))
	})

	t.Run("invalid connection details", func(t *testing.T) {
		conf := SQLConf{
			Type:             "postgres",
			ConnectionString: "host=localhost user=gorm password=gorm DB.name=gorm port=9920 sslmode=disable",
		}
		assert.Error(t, pump.Init(conf))
	})

	t.Run("should fail with unsupported type", func(t *testing.T) {
		conf := SQLConf{ConnectionString: "random"}
		assert.ErrorContains(t, pump.Init(conf), "Unsupported `config_storage.type` value:")
	})

	t.Run("invalid config", func(t *testing.T) {
		conf := map[string]interface{}{
			"connection_string": 1,
		}
		assert.ErrorContains(t, pump.Init(conf), "expected type")
	})

	t.Run("decode from map", func(t *testing.T) {
		conf := map[string]interface{}{
			"type":              "postgres",
			"table_sharding":    true,
			"connection_string": getTestPostgresConnectionString(),
		}
		require.NoError(t, pump.Init(conf))
		assert.Equal(t, "postgres", pump.SQLConf.Type)
		assert.True(t, pump.SQLConf.TableSharding)
	})

	t.Run("sharded table does not create base table", func(t *testing.T) {
		conf := SQLAggregatePumpConf{
			SQLConf: SQLConf{
				Type:             "postgres",
				ConnectionString: getTestPostgresConnectionString(),
				TableSharding:    true,
			},
		}
		require.NoError(t, pump.Init(conf))
		assert.False(t, pump.db.Migrator().HasTable(tableName))
	})

	t.Run("init from env", func(t *testing.T) {
		envPrefix := fmt.Sprintf("%s_SQLMCPAGGREGATE%s", PUMPS_ENV_PREFIX, PUMPS_ENV_META_PREFIX) + "_%s"
		envKeyVal := map[string]string{
			"TYPE":              "postgres",
			"TABLESHARDING":     "true",
			"CONNECTION_STRING": getTestPostgresConnectionString(),
		}
		for key, val := range envKeyVal {
			require.NoError(t, os.Setenv(fmt.Sprintf(envPrefix, key), val))
		}
		t.Cleanup(func() {
			for k := range envKeyVal {
				os.Unsetenv(fmt.Sprintf(envPrefix, k))
			}
		})

		conf := SQLAggregatePumpConf{
			SQLConf: SQLConf{
				Type:             "postgres",
				ConnectionString: getTestPostgresConnectionString(),
				TableSharding:    false,
			},
		}
		require.NoError(t, pump.Init(conf))
		assert.Equal(t, "postgres", pump.SQLConf.Type)
		assert.True(t, pump.SQLConf.TableSharding)
	})
}

func TestMCPSQLAggregatePump_WriteData(t *testing.T) {
	skipTestIfNoPostgres(t)
	tableName := analytics.AggregateMCPSQLTable

	conf := SQLAggregatePumpConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
		},
	}

	sampleRecord := analytics.AnalyticsRecord{
		TimeStamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Method:       "POST",
		Path:         "/mcp",
		APIName:      "test-api",
		APIID:        "test-api",
		ResponseCode: 200,
		Day:          1,
		Month:        1,
		Year:         2025,
		Hour:         0,
		OrgID:        "test-org",
		MCPStats: analytics.MCPStats{
			IsMCP:         true,
			JSONRPCMethod: "tools/call",
			PrimitiveType: "tool",
			PrimitiveName: "my_tool",
		},
	}

	type expectedRecord struct {
		orgID     string
		dimension string
		name      string
		hits      int
		success   int
		errCount  int
		apiID     string
	}

	testCases := []struct {
		name            string
		recordGenerator func() []interface{}
		expected        []expectedRecord
	}{
		{
			name: "aggregates MCP records by dimension",
			recordGenerator: func() []interface{} {
				records := make([]interface{}, 3)
				for i := range records {
					records[i] = sampleRecord
				}
				return records
			},
			expected: []expectedRecord{
				{orgID: "test-org", dimension: "methods", name: "tools/call", hits: 3, success: 3, errCount: 0, apiID: "test-api"},
				{orgID: "test-org", dimension: "primitives", name: "tool", hits: 3, success: 3, errCount: 0, apiID: "test-api"},
				{orgID: "test-org", dimension: "names", name: "tool_my_tool", hits: 3, success: 3, errCount: 0, apiID: "test-api"},
			},
		},
		{
			name: "skips non-MCP records",
			recordGenerator: func() []interface{} {
				nonMCP := analytics.AnalyticsRecord{
					TimeStamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					APIID:        "test-api",
					OrgID:        "test-org",
					ResponseCode: 200,
				}
				return []interface{}{sampleRecord, nonMCP, sampleRecord}
			},
			expected: []expectedRecord{
				{orgID: "test-org", dimension: "methods", name: "tools/call", hits: 2, success: 2, errCount: 0, apiID: "test-api"},
				{orgID: "test-org", dimension: "primitives", name: "tool", hits: 2, success: 2, errCount: 0, apiID: "test-api"},
				{orgID: "test-org", dimension: "names", name: "tool_my_tool", hits: 2, success: 2, errCount: 0, apiID: "test-api"},
			},
		},
		{
			name: "tracks errors",
			recordGenerator: func() []interface{} {
				errRecord := sampleRecord
				errRecord.ResponseCode = 500
				return []interface{}{sampleRecord, errRecord}
			},
			expected: []expectedRecord{
				{orgID: "test-org", dimension: "methods", name: "tools/call", hits: 2, success: 1, errCount: 1, apiID: "test-api"},
				{orgID: "test-org", dimension: "primitives", name: "tool", hits: 2, success: 1, errCount: 1, apiID: "test-api"},
				{orgID: "test-org", dimension: "names", name: "tool_my_tool", hits: 2, success: 1, errCount: 1, apiID: "test-api"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pump := MCPSQLAggregatePump{}
			require.NoError(t, pump.Init(conf))
			t.Cleanup(func() {
				pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", tableName))
			})

			records := tc.recordGenerator()
			require.NoError(t, pump.WriteData(context.Background(), records))

			for _, exp := range tc.expected {
				var resp []analytics.MCPSQLAnalyticsRecordAggregate
				tx := pump.db.Table(tableName).Where(
					"org_id = ? AND dimension = ? AND dimension_value = ? AND counter_hits = ? AND counter_success = ? AND counter_error = ? AND api_id = ?",
					exp.orgID, exp.dimension, exp.name, exp.hits, exp.success, exp.errCount, exp.apiID,
				).Find(&resp)
				require.NoError(t, tx.Error)
				if len(resp) < 1 {
					t.Errorf("missing record: api_id=%s, dimension=%s, dimension_value=%s, hits=%d, success=%d, error=%d",
						exp.apiID, exp.dimension, exp.name, exp.hits, exp.success, exp.errCount)
				}
			}
		})
	}
}

func TestMCPSQLAggregatePump_WriteData_Sharded(t *testing.T) {
	skipTestIfNoPostgres(t)
	tableName := analytics.AggregateMCPSQLTable

	pump := MCPSQLAggregatePump{}
	require.NoError(t, pump.Init(SQLAggregatePumpConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
			TableSharding:    true,
		},
	}))

	record1 := analytics.AnalyticsRecord{
		TimeStamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		APIID:     "test-api", APIName: "test-api", OrgID: "test-org",
		ResponseCode: 200, Day: 1, Month: 1, Year: 2025,
		MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
	}
	record2 := analytics.AnalyticsRecord{
		TimeStamp: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		APIID:     "test-api", APIName: "test-api", OrgID: "test-org",
		ResponseCode: 200, Day: 1, Month: 2, Year: 2025,
		MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t2"},
	}

	firstShard := tableName + "_20250101"
	secondShard := tableName + "_20250201"
	t.Cleanup(func() {
		pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", firstShard))
		pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", secondShard))
	})

	assert.False(t, pump.db.Migrator().HasTable(tableName))
	require.NoError(t, pump.WriteData(context.Background(), []interface{}{record1, record2}))
	assert.Truef(t, pump.db.Migrator().HasTable(firstShard), "table %s should exist", firstShard)
	assert.Truef(t, pump.db.Migrator().HasTable(secondShard), "table %s should exist", secondShard)

	for _, tbl := range []string{firstShard, secondShard} {
		var recs []analytics.MCPSQLAnalyticsRecordAggregate
		require.NoError(t, pump.db.Table(tbl).Find(&recs).Error)
		assert.NotEmptyf(t, recs, "table %s should contain records", tbl)
	}
}

func TestMCPSQLAggregatePump_aggregationTimeMinutes(t *testing.T) {
	t.Run("default is 60 minutes", func(t *testing.T) {
		pump := MCPSQLAggregatePump{SQLConf: &SQLAggregatePumpConf{}}
		assert.Equal(t, 60, pump.aggregationTimeMinutes())
	})

	t.Run("per minute when configured", func(t *testing.T) {
		pump := MCPSQLAggregatePump{SQLConf: &SQLAggregatePumpConf{StoreAnalyticsPerMinute: true}}
		assert.Equal(t, 1, pump.aggregationTimeMinutes())
	})
}

func TestMCPSQLAggregatePump_WriteData_EmptyData_NoInit(t *testing.T) {
	pump := &MCPSQLAggregatePump{}
	pump.log = log.WithField("prefix", mcpSQLAggregatePrefix)
	pump.SQLConf = &SQLAggregatePumpConf{}
	// Empty data → returns nil immediately without accessing DB.
	err := pump.WriteData(context.Background(), []interface{}{})
	assert.NoError(t, err)
}

func newMCPSQLAggregatePumpWithSQLite(t *testing.T, batchSize int, sharding bool) *MCPSQLAggregatePump {
	t.Helper()
	db := setupTestDB(t)
	tableName := analytics.AggregateMCPSQLTable

	require.NoError(t, db.Table(tableName).AutoMigrate(&analytics.MCPSQLAnalyticsRecordAggregate{}))

	pump := &MCPSQLAggregatePump{
		db: db,
		SQLConf: &SQLAggregatePumpConf{
			SQLConf: SQLConf{BatchSize: batchSize, TableSharding: sharding},
		},
	}
	pump.log = log.WithField("prefix", mcpSQLAggregatePrefix)
	return pump
}

func TestMCPSQLAggregatePump_ensureMCPAggregateShardedTable_SQLite(t *testing.T) {
	pump := newMCPSQLAggregatePumpWithSQLite(t, 100, true)

	table := pump.ensureMCPAggregateShardedTable("20250615")
	expected := analytics.AggregateMCPSQLTable + "_20250615"
	assert.Equal(t, expected, table)
	assert.True(t, pump.db.Migrator().HasTable(expected), "shard table should be created")

	// Calling again should not error (table already exists)
	table2 := pump.ensureMCPAggregateShardedTable("20250615")
	assert.Equal(t, expected, table2)
}

func TestMCPSQLAggregatePump_WriteData_SkipsNonMCP_SQLite(t *testing.T) {
	pump := newMCPSQLAggregatePumpWithSQLite(t, 100, false)
	ts := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	tableName := analytics.AggregateMCPSQLTable

	// All non-MCP records
	data := []interface{}{
		analytics.AnalyticsRecord{TimeStamp: ts, APIID: "api1", OrgID: "org1", ResponseCode: 200},
		analytics.AnalyticsRecord{TimeStamp: ts, APIID: "api2", OrgID: "org1", ResponseCode: 200},
	}

	require.NoError(t, pump.WriteData(context.Background(), data))

	var count int64
	pump.db.Table(tableName).Count(&count)
	assert.Zero(t, count, "non-MCP records should not produce any aggregate rows")
}

func TestMCPSQLAggregatePump_WriteData_EmptyData(t *testing.T) {
	skipTestIfNoPostgres(t)
	pump := MCPSQLAggregatePump{}
	require.NoError(t, pump.Init(SQLAggregatePumpConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
		},
	}))
	err := pump.WriteData(context.Background(), []interface{}{})
	assert.NoError(t, err)
}

func TestMCPSQLAggregatePump_WriteData_Upsert(t *testing.T) {
	skipTestIfNoPostgres(t)
	tableName := analytics.AggregateMCPSQLTable

	pump := MCPSQLAggregatePump{}
	require.NoError(t, pump.Init(SQLAggregatePumpConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
		},
	}))
	t.Cleanup(func() {
		pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", tableName))
	})

	rec := analytics.AnalyticsRecord{
		TimeStamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		APIID:     "test-api", APIName: "test-api", OrgID: "test-org",
		ResponseCode: 200, Day: 1, Month: 1, Year: 2025,
		MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
	}

	// First write: 2 records
	require.NoError(t, pump.WriteData(context.Background(), []interface{}{rec, rec}))

	// Second write: 1 more record — upsert should accumulate
	require.NoError(t, pump.WriteData(context.Background(), []interface{}{rec}))

	var resp []analytics.MCPSQLAnalyticsRecordAggregate
	tx := pump.db.Table(tableName).Where("dimension = ? AND dimension_value = ?", "names", "tool_t1").Find(&resp)
	require.NoError(t, tx.Error)
	require.Len(t, resp, 1)
	assert.Equal(t, 3, resp[0].Counter.Hits, "upsert should accumulate hits: 2 + 1 = 3")
}

func TestMCPSQLAggregatePump_WriteData_SmallBatchSize(t *testing.T) {
	skipTestIfNoPostgres(t)
	tableName := analytics.AggregateMCPSQLTable

	pump := MCPSQLAggregatePump{}
	require.NoError(t, pump.Init(SQLAggregatePumpConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
			BatchSize:        1, // force 1-record batches to exercise batch loop
		},
	}))
	t.Cleanup(func() {
		pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", tableName))
	})

	rec := analytics.AnalyticsRecord{
		TimeStamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		APIID:     "test-api", APIName: "test-api", OrgID: "test-org",
		ResponseCode: 200, Day: 1, Month: 1, Year: 2025,
		MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
	}

	require.NoError(t, pump.WriteData(context.Background(), []interface{}{rec}))

	// 3 dimensions: methods, primitives, names — each inserted in its own batch
	var count int64
	pump.db.Table(tableName).Count(&count)
	assert.Equal(t, int64(3), count, "batch size 1 should still write all 3 dimensions")
}

func TestMCPSQLAggregatePump_WriteData_MultipleAPIs(t *testing.T) {
	skipTestIfNoPostgres(t)
	tableName := analytics.AggregateMCPSQLTable

	pump := MCPSQLAggregatePump{}
	require.NoError(t, pump.Init(SQLAggregatePumpConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
		},
	}))
	t.Cleanup(func() {
		pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", tableName))
	})

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	records := []interface{}{
		analytics.AnalyticsRecord{
			TimeStamp: ts, APIID: "api-1", APIName: "api-1", OrgID: "org1",
			ResponseCode: 200, Day: 1, Month: 1, Year: 2025,
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
		},
		analytics.AnalyticsRecord{
			TimeStamp: ts, APIID: "api-2", APIName: "api-2", OrgID: "org1",
			ResponseCode: 200, Day: 1, Month: 1, Year: 2025,
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "resources/read", PrimitiveType: "resource", PrimitiveName: "r1"},
		},
	}

	require.NoError(t, pump.WriteData(context.Background(), records))

	// Each API produces 3 dimensions = 6 total rows
	var count int64
	pump.db.Table(tableName).Count(&count)
	assert.Equal(t, int64(6), count, "2 APIs × 3 dimensions = 6 rows")

	// Verify API-specific data
	var api1Recs []analytics.MCPSQLAnalyticsRecordAggregate
	pump.db.Table(tableName).Where("api_id = ?", "api-1").Find(&api1Recs)
	assert.Len(t, api1Recs, 3)

	var api2Recs []analytics.MCPSQLAnalyticsRecordAggregate
	pump.db.Table(tableName).Where("api_id = ?", "api-2").Find(&api2Recs)
	assert.Len(t, api2Recs, 3)
}
