package pumps

import (
	"context"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	MCPSQLPrefix = "MCPSQL-Pump"
	MCPSQLTable  = "tyk_analytics_mcp"
)

var MCPSQLDefaultENV = PUMPS_ENV_PREFIX + "_MCP_SQL" + PUMPS_ENV_META_PREFIX

// MCPSQLConf holds the configuration for the MCP SQL pump.
type MCPSQLConf struct {
	// TableName specifies the SQL table name for MCP analytics records.
	// In sharding mode, this is the table prefix.
	TableName string `json:"table_name" mapstructure:"table_name"`

	SQLConf `mapstructure:",squash"`
}

// MCPSQLPump writes raw MCP analytics records to a dedicated SQL table.
type MCPSQLPump struct {
	db        *gorm.DB
	Conf      *MCPSQLConf
	tableName string
	CommonPumpConfig
}

func (g *MCPSQLPump) GetName() string {
	return "MCP SQL Pump"
}

func (g *MCPSQLPump) New() Pump {
	return &MCPSQLPump{}
}

func (g *MCPSQLPump) Init(conf interface{}) error {
	g.Conf = &MCPSQLConf{}
	g.log = log.WithField("prefix", MCPSQLPrefix)

	if err := mapstructure.Decode(conf, g.Conf); err != nil {
		g.log.Error("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(g, g.log, g.Conf, MCPSQLDefaultENV)

	db, err := OpenGormDB(&g.Conf.SQLConf, g.log)
	if err != nil {
		return err
	}
	g.db = db

	if g.Conf.BatchSize == 0 {
		g.Conf.BatchSize = SQLDefaultQueryBatchSize
	}

	g.tableName = MCPSQLTable
	if name := g.Conf.TableName; name != "" {
		g.tableName = name
	}

	analytics.MCPSQLTableName = g.tableName

	migrateShardedTables := func() error {
		return MigrateAllShardedTables(g.db, g.tableName, "mcp", &analytics.MCPRecord{}, g.log)
	}

	if err := HandleTableMigration(g.db, &g.Conf.SQLConf, g.tableName, &analytics.MCPRecord{}, g.log, migrateShardedTables); err != nil {
		return err
	}
	g.db = g.db.Table(g.tableName)

	if g.db.Error != nil {
		g.log.WithError(g.db.Error).Error("error initializing pump")
		return g.db.Error
	}

	g.log.Debug("pump initialized and table set up")
	return nil
}

func (g *MCPSQLPump) getMCPRecords(data []interface{}) []*analytics.MCPRecord {
	var mcpRecords []*analytics.MCPRecord
	for _, r := range data {
		if r == nil {
			continue
		}
		rec, ok := r.(analytics.AnalyticsRecord)
		if !ok || !rec.IsMCPRecord() {
			continue
		}
		mr := rec.ToMCPRecord()
		mcpRecords = append(mcpRecords, &mr)
	}
	return mcpRecords
}

func (g *MCPSQLPump) GetEnvPrefix() string {
	return g.Conf.EnvPrefix
}

// writeMCPBatch writes a slice of MCP records to the DB in batches.
func (g *MCPSQLPump) writeMCPBatch(ctx context.Context, recs []*analytics.MCPRecord) {
	for ri := 0; ri < len(recs); ri += g.Conf.BatchSize {
		ends := ri + g.Conf.BatchSize
		if ends > len(recs) {
			ends = len(recs)
		}
		if tx := g.db.WithContext(ctx).Create(recs[ri:ends]); tx.Error != nil {
			g.log.Error(tx.Error)
		}
	}
}

// ensureMCPShardedTable switches the DB handle to the date-specific shard table,
// creating it via AutoMigrate if it does not yet exist.
func (g *MCPSQLPump) ensureMCPShardedTable(recDate string) {
	table := g.tableName + "_" + recDate
	g.db = g.db.Table(table)
	if !g.db.Migrator().HasTable(table) {
		if err := g.db.AutoMigrate(&analytics.MCPRecord{}); err != nil {
			g.log.WithError(err).Error("error creating sharded MCP table")
		}
	}
}

func (g *MCPSQLPump) WriteData(ctx context.Context, data []interface{}) error {
	g.log.Debug("Attempting to write ", len(data), " records...")

	mcpRecords := g.getMCPRecords(data)
	dataLen := len(mcpRecords)

	if dataLen == 0 {
		g.log.Debug("no MCP records")
		return nil
	}

	startIndex := 0
	endIndex := dataLen

	for i := 0; i <= dataLen; i++ {
		if g.Conf.TableSharding {
			recDate := mcpRecords[startIndex].AnalyticsRecord.TimeStamp.Format("20060102")
			var nextRecDate string
			if i == dataLen {
				nextRecDate = mcpRecords[dataLen-1].AnalyticsRecord.TimeStamp.Format("20060102")
				recDate = nextRecDate
			} else {
				nextRecDate = mcpRecords[i].AnalyticsRecord.TimeStamp.Format("20060102")
				if recDate == nextRecDate {
					continue
				}
			}
			endIndex = i
			g.ensureMCPShardedTable(recDate)
		} else {
			i = dataLen
		}

		g.writeMCPBatch(ctx, mcpRecords[startIndex:endIndex])
		startIndex = i
	}

	g.log.Info("Purged ", dataLen, " records...")
	return nil
}

func (g *MCPSQLPump) SetLogLevel(level logrus.Level) {
	g.log.Level = level
}
