package pumps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

const (
	GraphSQLPrefix = "GraphSQL-Pump"
	GraphSQLTable  = "tyk_analytics_graph"
)

var GraphSQLDefaultENV = PUMPS_ENV_PREFIX + "_GRAPH_SQL" + PUMPS_ENV_META_PREFIX

type GraphSQLConf struct {
	// TableName is a configuration field unique to the sql-graph pump, this field specifies
	// the name of the sql table to be created/used for the pump in the cases of non-sharding
	// in the case of sharding, it specifies the table prefix
	TableName string `json:"table_name" mapstructure:"table_name"`

	SQLConf `mapstructure:",squash"`
}
type GraphSQLPump struct {
	db        *gorm.DB
	Conf      *GraphSQLConf
	tableName string
	CommonPumpConfig
}

func (g *GraphSQLPump) GetName() string {
	return "Graph SQL Pump"
}

func (g *GraphSQLPump) New() Pump {
	return &GraphSQLPump{}
}

func (g *GraphSQLPump) Init(conf interface{}) error {
	g.log = log.WithField("prefix", GraphSQLPrefix)

	if err := mapstructure.Decode(conf, &g.Conf); err != nil {
		g.log.WithError(err).Error("error decoding conf")
		return fmt.Errorf("error decoding conf: %w", err)
	}

	processPumpEnvVars(g, g.log, g.Conf, GraphSQLDefaultENV)

	logLevel := gorm_logger.Silent

	switch g.Conf.LogLevel {
	case "debug":
		logLevel = gorm_logger.Info
	case "info":
		logLevel = gorm_logger.Warn
	case "warning":
		logLevel = gorm_logger.Error
	}

	dialect, errDialect := Dialect(&g.Conf.SQLConf)
	if errDialect != nil {
		g.log.Error(errDialect)
		return errDialect
	}

	db, err := gorm.Open(dialect, &gorm.Config{
		AutoEmbedd:  true,
		UseJSONTags: true,
		Logger:      gorm_logger.Default.LogMode(logLevel),
	})
	if err != nil {
		g.log.WithError(err).Error("error opening gorm connection")
		return err
	}
	g.db = db

	if g.Conf.BatchSize == 0 {
		g.Conf.BatchSize = SQLDefaultQueryBatchSize
	}

	g.tableName = GraphSQLTable
	if name := g.Conf.TableName; name != "" {
		g.tableName = name
	}

	analytics.GraphSQLTableName = g.tableName
	if !g.Conf.TableSharding {
		if err := g.db.Table(g.tableName).AutoMigrate(&analytics.GraphRecord{}); err != nil {
			g.log.WithError(err).Error("error migrating graph analytics table")
			return err
		}
	} else if g.Conf.MigrateOldTables {
		// Migrate all existing sharded tables
		if err := g.migrateAllShardedTables(); err != nil {
			g.log.WithError(err).Warn("Failed to migrate existing sharded graph tables")
			// Don't fail initialization, just log the warning
		}
	} else {
		// Migrate current day's table to ensure it has latest schema
		currentDayTable := g.tableName + "_" + time.Now().Format("20060102")
		if err := g.db.Table(currentDayTable).AutoMigrate(&analytics.GraphRecord{}); err != nil {
			g.log.WithField("table", currentDayTable).WithError(err).Warn("Failed to migrate current day table")
			// Don't fail initialization, just log the warning
		} else {
			g.log.WithField("table", currentDayTable).Debug("Migrated current day table")
		}
	}
	g.db = g.db.Table(g.tableName)

	if g.db.Error != nil {
		g.log.WithError(err).Error("error initializing pump")
		return err
	}

	g.log.Debug("pump initialized and table set up")
	return nil
}

func (g *GraphSQLPump) getGraphRecords(data []interface{}) []*analytics.GraphRecord {
	var graphRecords []*analytics.GraphRecord
	for _, r := range data {
		if r != nil {
			var (
				rec analytics.AnalyticsRecord
				ok  bool
			)
			if rec, ok = r.(analytics.AnalyticsRecord); !ok || !rec.IsGraphRecord() {
				continue
			}
			gr := rec.ToGraphRecord()
			graphRecords = append(graphRecords, &gr)
		}
	}
	return graphRecords
}

func (g *GraphSQLPump) GetEnvPrefix() string {
	return g.Conf.EnvPrefix
}

func (g *GraphSQLPump) WriteData(ctx context.Context, data []interface{}) error {
	g.log.Debug("Attempting to write ", len(data), " records...")

	graphRecords := g.getGraphRecords(data)
	dataLen := len(graphRecords)

	startIndex := 0
	endIndex := dataLen
	// We iterate dataLen +1 times since we're writing the data after the date change on sharding_table:true
	if dataLen == 0 {
		g.log.Debug("no graphql records")
		return nil
	}
	for i := 0; i <= dataLen; i++ {
		if g.Conf.TableSharding {
			recDate := graphRecords[startIndex].AnalyticsRecord.TimeStamp.Format("20060102")
			var nextRecDate string
			// if we're on i == dataLen iteration, it means that we're out of index range. We're going to use the last record date.
			if i == dataLen {
				nextRecDate = graphRecords[dataLen-1].AnalyticsRecord.TimeStamp.Format("20060102")
				recDate = nextRecDate
			} else {
				nextRecDate = graphRecords[i].AnalyticsRecord.TimeStamp.Format("20060102")

				// if both dates are equal, we shouldn't write in the table yet.
				if recDate == nextRecDate {
					continue
				}
			}

			endIndex = i

			table := g.tableName + "_" + recDate
			g.db = g.db.Table(table)
			if !g.db.Migrator().HasTable(table) {
				if err := g.db.AutoMigrate(&analytics.GraphRecord{}); err != nil {
					g.log.Error("error creating table for record")
					g.log.WithError(err).Debug("error creating table for record")
				}
			}
		} else {
			i = dataLen // write all records at once for non-sharded case, stop for loop after 1 iteration
		}

		recs := graphRecords[startIndex:endIndex]

		for ri := 0; ri < len(recs); ri += g.Conf.BatchSize {
			ends := ri + g.Conf.BatchSize
			if ends > len(recs) {
				ends = len(recs)
			}
			tx := g.db.WithContext(ctx).Create(recs[ri:ends])
			if tx.Error != nil {
				g.log.Error(tx.Error)
			}
		}

		startIndex = i // next day start index, necessary for sharded case
	}

	g.log.Info("Purged ", dataLen, " records...")

	return nil
}

func (g *GraphSQLPump) SetLogLevel(level logrus.Level) {
	g.log.Level = level
}

// migrateAllShardedTables scans for all existing sharded tables and migrates them
func (g *GraphSQLPump) migrateAllShardedTables() error {
	if !g.Conf.TableSharding {
		// No sharding, nothing to migrate
		return nil
	}

	g.log.Info("Scanning for existing sharded graph tables to migrate...")

	// Get all tables in the database
	var tables []string
	err := g.db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tables).Error
	if err != nil {
		g.log.WithError(err).Warn("Failed to get list of tables, skipping migration scan")
		return nil
	}

	// Find tables matching our sharded pattern
	shardedTables := make([]string, 0)
	tablePrefix := g.tableName + "_"

	for _, table := range tables {
		if strings.HasPrefix(table, tablePrefix) {
			// Check if it matches the date pattern (YYYYMMDD)
			suffix := strings.TrimPrefix(table, tablePrefix)
			if len(suffix) == 8 {
				// Try to parse as date to validate format
				if _, err := time.Parse("20060102", suffix); err == nil {
					shardedTables = append(shardedTables, table)
				}
			}
		}
	}

	g.log.WithField("count", len(shardedTables)).Info("Found sharded graph tables to migrate")

	// Migrate each sharded table
	for _, tableName := range shardedTables {
		g.log.WithField("table", tableName).Debug("Migrating sharded graph table")

		g.db = g.db.Table(tableName)
		if err := g.db.AutoMigrate(&analytics.GraphRecord{}); err != nil {
			g.log.WithField("table", tableName).WithError(err).Warn("Failed to migrate sharded graph table")
			// Continue with other tables even if one fails
		} else {
			g.log.WithField("table", tableName).Debug("Successfully migrated sharded graph table")
		}
	}

	g.log.Info("Completed migration of sharded graph tables")
	return nil
}
