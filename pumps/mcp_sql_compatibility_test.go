package pumps

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This independently specifies the pre-context SQL shape; none of the four new columns exists.
type legacyMCPSQLRecord struct {
	JSONRPCMethod   string                    `gorm:"column:jsonrpc_method"`
	PrimitiveType   string                    `gorm:"column:primitive_type"`
	PrimitiveName   string                    `gorm:"column:primitive_name"`
	AnalyticsRecord analytics.AnalyticsRecord `gorm:"embedded;embeddedPrefix:analytics_"`
}

func TestMCPSQLPostgresContextCompatibility(t *testing.T) {
	skipTestIfNoPostgres(t)
	for _, migrateLegacy := range []bool{false, true} {
		name := "fresh"
		if migrateLegacy {
			name = "legacy-migration"
		}
		t.Run(name, func(t *testing.T) {
			tableName := "test_mcp_context_" + model.NewObjectID().Hex()
			previous := analytics.MCPSQLTableName
			t.Cleanup(func() { analytics.MCPSQLTableName = previous })
			conf := MCPSQLConf{SQLConf: SQLConf{Type: "postgres", ConnectionString: getTestPostgresConnectionString()}, TableName: tableName}
			db, err := OpenGormDB(&conf.SQLConf, log.WithField("prefix", "mcp-compatibility"))
			require.NoError(t, err)
			connection, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, connection.Close()) })
			t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(tableName)) })
			fields := []string{"effective_protocol_version", "declared_protocol_version", "protocol_version_source", "jsonrpc_error_code"}
			if migrateLegacy {
				require.NoError(t, db.Table(tableName).AutoMigrate(&legacyMCPSQLRecord{}))
				for _, field := range fields {
					assert.False(t, db.Migrator().HasColumn(tableName, field))
				}
				legacy := legacyMCPSQLRecord{JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "legacy_tool", AnalyticsRecord: analytics.AnalyticsRecord{APIID: "legacy-api", OrgID: "legacy-org", ResponseCode: 200}}
				require.NoError(t, db.Table(tableName).Create(&legacy).Error)
			}
			pump := &MCPSQLPump{}
			require.NoError(t, pump.Init(conf))
			pumpConnection, err := pump.db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pumpConnection.Close()) })
			var columns []struct {
				ColumnName    string
				DataType      string
				IsNullable    string
				ColumnDefault sql.NullString
			}
			require.NoError(t, db.Raw("SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name IN ?", tableName, fields).Scan(&columns).Error)
			require.Len(t, columns, 4)
			for _, column := range columns {
				assert.Equal(t, "YES", column.IsNullable, column.ColumnName)
				assert.False(t, column.ColumnDefault.Valid, column.ColumnName)
				expectedType := "text"
				if column.ColumnName == "jsonrpc_error_code" {
					expectedType = "bigint"
				}
				assert.Equal(t, expectedType, column.DataType, column.ColumnName)
			}
			if migrateLegacy {
				var legacy analytics.MCPRecord
				require.NoError(t, db.Table(tableName).Where("analytics_apiid = ?", "legacy-api").Take(&legacy).Error)
				assert.Equal(t, "legacy-api", legacy.AnalyticsRecord.APIID)
				assert.Equal(t, "legacy-org", legacy.AnalyticsRecord.OrgID)
				assert.Equal(t, "tools/call", legacy.JSONRPCMethod)
				assert.Equal(t, "tool", legacy.PrimitiveType)
				assert.Equal(t, "legacy_tool", legacy.PrimitiveName)
				assert.Empty(t, legacy.EffectiveProtocolVersion)
				assert.Empty(t, legacy.DeclaredProtocolVersion)
				assert.Empty(t, legacy.ProtocolVersionSource)
				assert.Zero(t, legacy.JSONRPCErrorCode)
				for _, field := range fields {
					var count int64
					require.NoError(t, db.Table(tableName).Where("analytics_apiid = ?", "legacy-api").Where(fmt.Sprintf("%s IS NULL", field)).Count(&count).Error)
					assert.Equal(t, int64(1), count, field)
				}
			}
			cases := append(loadMCPContextCases(t), loadMCPSignedCodeCases(t)...)
			records := make([]interface{}, 0, len(cases)+1)
			for _, tc := range cases {
				records = append(records, tc.Record)
			}
			records = append(records, analytics.AnalyticsRecord{APIID: "ordinary-rest", ResponseCode: 200})
			require.NoError(t, pump.WriteData(context.Background(), records))
			var results []analytics.MCPRecord
			require.NoError(t, db.Table(tableName).Where("analytics_orgid = ?", "mcp-compatibility").Find(&results).Error)
			require.Len(t, results, len(cases))
			byAPI := make(map[string]analytics.MCPRecord, len(results))
			for _, record := range results {
				require.NotContains(t, byAPI, record.AnalyticsRecord.APIID)
				byAPI[record.AnalyticsRecord.APIID] = record
			}
			for _, tc := range cases {
				t.Run(tc.Name, func(t *testing.T) {
					require.Contains(t, byAPI, tc.Record.APIID)
					assertMCPContextRecord(t, &tc.Record, byAPI[tc.Record.APIID])
				})
			}
			var restCount int64
			require.NoError(t, db.Table(tableName).Where("analytics_apiid = ?", "ordinary-rest").Count(&restCount).Error)
			assert.Zero(t, restCount)
		})
	}
}
