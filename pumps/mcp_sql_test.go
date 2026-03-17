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

func TestMCPSQLPump_Init(t *testing.T) {
	skipTestIfNoPostgres(t)
	pump := &MCPSQLPump{}

	t.Run("successful", func(t *testing.T) {
		conf := MCPSQLConf{
			SQLConf: SQLConf{
				Type:             "postgres",
				ConnectionString: getTestPostgresConnectionString(),
			},
			TableName: "test_mcp_init",
		}
		require.NoError(t, pump.Init(conf))
		t.Cleanup(func() {
			pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", conf.TableName))
		})
		assert.True(t, pump.db.Migrator().HasTable(conf.TableName))
	})

	t.Run("invalid connection details", func(t *testing.T) {
		conf := MCPSQLConf{
			SQLConf: SQLConf{
				Type:             "postgres",
				ConnectionString: "host=localhost user=gorm password=gorm DB.name=gorm port=9920 sslmode=disable",
			},
		}
		assert.Error(t, pump.Init(conf))
	})

	t.Run("should fail with unsupported type", func(t *testing.T) {
		conf := MCPSQLConf{
			SQLConf: SQLConf{ConnectionString: "random"},
		}
		assert.ErrorContains(t, pump.Init(conf), "Unsupported `config_storage.type` value:")
	})

	t.Run("invalid config", func(t *testing.T) {
		conf := map[string]interface{}{
			"table_name": 1,
		}
		assert.ErrorContains(t, pump.Init(conf), "expected type")
	})

	t.Run("decode from map", func(t *testing.T) {
		conf := map[string]interface{}{
			"table_name":        "test_mcp_map",
			"type":              "postgres",
			"table_sharding":    true,
			"connection_string": getTestPostgresConnectionString(),
		}
		require.NoError(t, pump.Init(conf))
		assert.Equal(t, "test_mcp_map", pump.Conf.TableName)
		assert.Equal(t, "postgres", pump.Conf.Type)
		assert.True(t, pump.Conf.TableSharding)
	})

	t.Run("sharded table does not create base table", func(t *testing.T) {
		conf := MCPSQLConf{
			SQLConf: SQLConf{
				Type:             "postgres",
				ConnectionString: getTestPostgresConnectionString(),
				TableSharding:    true,
			},
			TableName: "test_mcp_sharded",
		}
		require.NoError(t, pump.Init(conf))
		assert.False(t, pump.db.Migrator().HasTable(conf.TableName))
	})

	t.Run("init from env", func(t *testing.T) {
		envPrefix := fmt.Sprintf("%s_MCP_SQL%s", PUMPS_ENV_PREFIX, PUMPS_ENV_META_PREFIX) + "_%s"
		envKeyVal := map[string]string{
			"TYPE":          "postgres",
			"TABLENAME":     "test_mcp_env",
			"TABLESHARDING": "true",
		}
		for key, val := range envKeyVal {
			require.NoError(t, os.Setenv(fmt.Sprintf(envPrefix, key), val))
		}
		t.Cleanup(func() {
			for k := range envKeyVal {
				os.Unsetenv(fmt.Sprintf(envPrefix, k))
			}
		})

		conf := MCPSQLConf{
			SQLConf: SQLConf{
				Type:             "postgres",
				ConnectionString: getTestPostgresConnectionString(),
				TableSharding:    false,
			},
			TableName: "wrong-name",
		}
		require.NoError(t, pump.Init(conf))
		assert.Equal(t, "postgres", pump.Conf.Type)
		assert.Equal(t, "test_mcp_env", pump.Conf.TableName)
		assert.True(t, pump.Conf.TableSharding)
	})
}

func TestMCPSQLPump_WriteData(t *testing.T) {
	skipTestIfNoPostgres(t)
	tableName := "test_mcp_write"

	conf := MCPSQLConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
		},
		TableName: tableName,
	}

	mcpRecord := func(method, primType, primName string, code int) analytics.AnalyticsRecord {
		return analytics.AnalyticsRecord{
			TimeStamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			Method:       "POST",
			Path:         "/mcp",
			APIName:      "test-api",
			APIID:        "test-api",
			ResponseCode: code,
			OrgID:        "test-org",
			MCPStats: analytics.MCPStats{
				IsMCP:         true,
				JSONRPCMethod: method,
				PrimitiveType: primType,
				PrimitiveName: primName,
			},
		}
	}

	testCases := []struct {
		name            string
		records         []interface{}
		expectedMCPRows int
	}{
		{
			name: "writes MCP records only",
			records: []interface{}{
				mcpRecord("tools/call", "tool", "my_tool", 200),
				// non-MCP record should be skipped
				analytics.AnalyticsRecord{
					TimeStamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					APIName:      "test-api",
					APIID:        "test-api",
					ResponseCode: 200,
					OrgID:        "test-org",
				},
				mcpRecord("resources/read", "resource", "users", 200),
			},
			expectedMCPRows: 2,
		},
		{
			name:            "no MCP records writes nothing",
			records:         []interface{}{analytics.AnalyticsRecord{APIID: "test-api", ResponseCode: 200, OrgID: "test-org"}},
			expectedMCPRows: 0,
		},
		{
			name:            "empty data",
			records:         []interface{}{},
			expectedMCPRows: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pump := &MCPSQLPump{}
			require.NoError(t, pump.Init(conf))
			t.Cleanup(func() {
				pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", tableName))
			})

			err := pump.WriteData(context.Background(), tc.records)
			require.NoError(t, err)

			var count int64
			pump.db.Table(tableName).Count(&count)
			assert.Equal(t, int64(tc.expectedMCPRows), count)
		})
	}
}

func TestMCPSQLPump_Sharded(t *testing.T) {
	skipTestIfNoPostgres(t)
	tableName := "test_mcp_shard"

	conf := MCPSQLConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
			TableSharding:    true,
		},
		TableName: tableName,
	}

	pump := &MCPSQLPump{}
	require.NoError(t, pump.Init(conf))

	records := []interface{}{
		analytics.AnalyticsRecord{
			TimeStamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			APIID:        "test-api",
			APIName:      "test-api",
			OrgID:        "test-org",
			ResponseCode: 200,
			Month:        1, Day: 1, Year: 2025,
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
		},
		analytics.AnalyticsRecord{
			TimeStamp:    time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			APIID:        "test-api",
			APIName:      "test-api",
			OrgID:        "test-org",
			ResponseCode: 200,
			Month:        2, Day: 1, Year: 2025,
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t2"},
		},
	}

	expectedTables := []string{
		tableName + "_20250101",
		tableName + "_20250201",
	}
	t.Cleanup(func() {
		for _, tbl := range expectedTables {
			pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", tbl))
		}
	})

	require.NoError(t, pump.WriteData(context.Background(), records))

	for _, tbl := range expectedTables {
		assert.Truef(t, pump.db.Migrator().HasTable(tbl), "table %s should exist", tbl)
		var count int64
		pump.db.Table(tbl).Count(&count)
		assert.Equalf(t, int64(1), count, "table %s should have 1 record", tbl)
	}
}

func TestMCPSQLPump_getMCPRecords(t *testing.T) {
	pump := &MCPSQLPump{}

	records := []interface{}{
		analytics.AnalyticsRecord{
			MCPStats: analytics.MCPStats{IsMCP: true, PrimitiveType: "tool", PrimitiveName: "t1"},
		},
		analytics.AnalyticsRecord{
			MCPStats: analytics.MCPStats{IsMCP: false},
		},
		nil,
		"invalid type",
		analytics.AnalyticsRecord{
			MCPStats: analytics.MCPStats{IsMCP: true, PrimitiveType: "resource", PrimitiveName: "r1"},
		},
	}

	result := pump.getMCPRecords(records)
	assert.Len(t, result, 2)
	assert.Equal(t, "t1", result[0].PrimitiveName)
	assert.Equal(t, "r1", result[1].PrimitiveName)
}

func TestMCPSQLPump_WriteData_NoMCPRecords_NoInit(t *testing.T) {
	p := &MCPSQLPump{}
	p.log = log.WithField("prefix", MCPSQLPrefix)
	p.Conf = &MCPSQLConf{}
	// Non-MCP records produce empty mcpRecords slice → returns nil without accessing DB.
	err := p.WriteData(context.Background(), []interface{}{
		analytics.AnalyticsRecord{APIID: "api1", ResponseCode: 200},
	})
	assert.NoError(t, err)
}

func TestMCPSQLPump_WriteData_EmptyData(t *testing.T) {
	skipTestIfNoPostgres(t)
	pump := &MCPSQLPump{}
	conf := MCPSQLConf{
		SQLConf: SQLConf{
			Type:             "postgres",
			ConnectionString: getTestPostgresConnectionString(),
		},
		TableName: "test_mcp_empty",
	}
	require.NoError(t, pump.Init(conf))
	t.Cleanup(func() {
		pump.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", conf.TableName))
	})
	err := pump.WriteData(context.Background(), []interface{}{})
	assert.NoError(t, err)
}
