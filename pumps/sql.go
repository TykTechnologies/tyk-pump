package pumps

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/vmihailenco/msgpack.v2"
	"gorm.io/gorm/clause"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

type PostgresConfig struct {
	// Disables implicit prepared statement usage.
	PreferSimpleProtocol bool `json:"prefer_simple_protocol" mapstructure:"prefer_simple_protocol"`
}

type MysqlConfig struct {
	// Default size for string fields. Defaults to `256`.
	DefaultStringSize uint `json:"default_string_size" mapstructure:"default_string_size"`
	// Disable datetime precision, which not supported before MySQL 5.6.
	DisableDatetimePrecision bool `json:"disable_datetime_precision" mapstructure:"disable_datetime_precision"`
	// Drop & create when rename index, rename index not supported before MySQL 5.7, MariaDB.
	DontSupportRenameIndex bool `json:"dont_support_rename_index" mapstructure:"dont_support_rename_index"`
	// `change` when rename column, rename column not supported before MySQL 8, MariaDB.
	DontSupportRenameColumn bool `json:"dont_support_rename_column" mapstructure:"dont_support_rename_column"`
	// Auto configure based on currently MySQL version.
	SkipInitializeWithVersion bool `json:"skip_initialize_with_version" mapstructure:"skip_initialize_with_version"`
}

type SQLPump struct {
	CommonPumpConfig
	IsUptime bool

	SQLConf *SQLConf

	db      *gorm.DB
	dbType  string
	dialect gorm.Dialector

	// this channel is used to signal that the background index creation has finished - this is used for testing
	backgroundIndexCreated chan bool
}

// @PumpConf SQL
type SQLConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_SQL_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The only supported and tested types are `postgres` and `mysql`.
	// From v1.12.0, we no longer support `sqlite` as a storage type.
	Type string `json:"type" mapstructure:"type"`
	// Specifies the connection string to the database.
	ConnectionString string `json:"connection_string" mapstructure:"connection_string"`
	// Postgres configurations.
	Postgres PostgresConfig `json:"postgres" mapstructure:"postgres"`
	// Mysql configurations.
	Mysql MysqlConfig `json:"mysql" mapstructure:"mysql"`
	// Specifies if all the analytics records are going to be stored in one table or in multiple
	// tables (one per day). By default, `false`. If `false`, all the records are going to be
	// stored in `tyk_aggregated` table. Instead, if it's `true`, all the records of the day are
	// going to be stored in `tyk_aggregated_YYYYMMDD` table, where `YYYYMMDD` is going to change
	// depending on the date.
	TableSharding bool `json:"table_sharding" mapstructure:"table_sharding"`
	// Specifies the SQL log verbosity. The possible values are: `info`,`error` and `warning`. By
	// default, the value is `silent`, which means that it won't log any SQL query.
	LogLevel string `json:"log_level" mapstructure:"log_level"`
	// Specifies the amount of records that are going to be written each batch. Type int. By
	// default, it writes 1000 records max per batch.
	BatchSize int `json:"batch_size" mapstructure:"batch_size"`
	// Specifies whether to migrate all existing sharded tables during initialization.
	// When true, scans for all sharded tables matching the pattern and migrates them on init.
	// When false, only migrates tables as they are accessed during WriteData.
	// Defaults to false for performance reasons.
	MigrateOldTables bool `json:"migrate_old_tables" mapstructure:"migrate_old_tables" default:"false"`
}

func Dialect(cfg *SQLConf) (gorm.Dialector, error) {
	switch cfg.Type {
	case "postgres":
		// Example connection_string: `"host=localhost user=gorm password=gorm DB.name=gorm port=9920 sslmode=disable TimeZone=Asia/Shanghai"`
		return postgres.New(postgres.Config{
			DSN:                  cfg.ConnectionString,
			PreferSimpleProtocol: cfg.Postgres.PreferSimpleProtocol,
		}), nil
	case "mysql":
		return mysql.New(mysql.Config{
			DSN:                       cfg.ConnectionString,
			DefaultStringSize:         cfg.Mysql.DefaultStringSize,
			DisableDatetimePrecision:  cfg.Mysql.DisableDatetimePrecision,
			DontSupportRenameIndex:    cfg.Mysql.DontSupportRenameIndex,
			DontSupportRenameColumn:   cfg.Mysql.DontSupportRenameColumn,
			SkipInitializeWithVersion: cfg.Mysql.SkipInitializeWithVersion,
		}), nil
	default:
		return nil, errors.New("Unsupported `config_storage.type` value:" + cfg.Type)
	}
}

var (
	SQLPrefix                = "SQL-pump"
	SQLDefaultENV            = PUMPS_ENV_PREFIX + "_SQL" + PUMPS_ENV_META_PREFIX
	SQLDefaultQueryBatchSize = 1000

	indexes = []struct {
		baseName string
		column   string
	}{
		{"idx_responsecode", "responsecode"},
		{"idx_apikey", "apikey"},
		{"idx_timestamp", "timestamp"},
		{"idx_apiid", "apiid"},
		{"idx_orgid", "orgid"},
		{"idx_oauthid", "oauthid"},
	}
)

func (c *SQLPump) New() Pump {
	newPump := SQLPump{}
	return &newPump
}

func (c *SQLPump) GetName() string {
	return "SQL Pump"
}

func (c *SQLPump) GetEnvPrefix() string {
	return c.SQLConf.EnvPrefix
}

func (c *SQLPump) SetDecodingRequest(decoding bool) {
	if decoding {
		log.WithField("pump", c.GetName()).Warn("Decoding request is not supported for SQL pump")
	}
}

func (c *SQLPump) SetDecodingResponse(decoding bool) {
	if decoding {
		log.WithField("pump", c.GetName()).Warn("Decoding response is not supported for SQL pump")
	}
}

func (c *SQLPump) Init(conf interface{}) error {
	c.SQLConf = &SQLConf{}
	if c.IsUptime {
		c.log = log.WithField("prefix", SQLPrefix+"-uptime")
	} else {
		c.log = log.WithField("prefix", SQLPrefix)
	}

	err := mapstructure.Decode(conf, &c.SQLConf)
	if err != nil {
		c.log.Error("Failed to decode configuration: ", err)
		return err
	}

	if !c.IsUptime {
		processPumpEnvVars(c, c.log, c.SQLConf, SQLDefaultENV)
	}

	logLevel := gorm_logger.Silent

	switch c.SQLConf.LogLevel {
	case "debug":
		logLevel = gorm_logger.Info
	case "info":
		logLevel = gorm_logger.Warn
	case "warning":
		logLevel = gorm_logger.Error
	}

	dialect, errDialect := Dialect(c.SQLConf)
	if errDialect != nil {
		c.log.Error(errDialect)
		return errDialect
	}

	db, err := gorm.Open(dialect, &gorm.Config{
		AutoEmbedd:  true,
		UseJSONTags: true,
		Logger:      gorm_logger.Default.LogMode(logLevel),
	})
	if err != nil {
		c.log.Error(err)
		return err
	}
	c.db = db

	if !c.SQLConf.TableSharding {
		if c.IsUptime {
			c.db.Table(analytics.UptimeSQLTable).AutoMigrate(&analytics.UptimeReportAggregateSQL{})
		} else {
			c.db.Table(analytics.SQLTable).AutoMigrate(&analytics.AnalyticsRecord{})
		}
	} else if c.SQLConf.MigrateOldTables {
		// Migrate all existing sharded tables on init
		if err := c.migrateAllShardedTables(); err != nil {
			c.log.WithError(err).Warn("Failed to migrate existing sharded tables")
			// Don't fail initialization, just log the warning
		}
	} else {
		// Migrate current day's table to ensure it has latest schema
		currentDayTable := analytics.SQLTable + "_" + time.Now().Format("20060102")
		if c.IsUptime {
			currentDayTable = analytics.UptimeSQLTable + "_" + time.Now().Format("20060102")
			if err := c.db.Table(currentDayTable).AutoMigrate(&analytics.UptimeReportAggregateSQL{}); err != nil {
				c.log.WithField("table", currentDayTable).WithError(err).Warn("Failed to migrate current day uptime table")
			} else {
				c.log.WithField("table", currentDayTable).Debug("Migrated current day uptime table")
			}
		} else {
			if err := c.db.Table(currentDayTable).AutoMigrate(&analytics.AnalyticsRecord{}); err != nil {
				c.log.WithField("table", currentDayTable).WithError(err).Warn("Failed to migrate current day table")
			} else {
				c.log.WithField("table", currentDayTable).Debug("Migrated current day table")
			}
		}
	}

	if c.SQLConf.BatchSize == 0 {
		c.SQLConf.BatchSize = SQLDefaultQueryBatchSize
	}

	c.log.Debug("SQL Initialized")
	return nil
}

func (c *SQLPump) WriteData(ctx context.Context, data []interface{}) error {
	c.log.Debug("Attempting to write ", len(data), " records...")

	var typedData []*analytics.AnalyticsRecord
	for _, r := range data {
		if r != nil {
			rec := r.(analytics.AnalyticsRecord)
			typedData = append(typedData, &rec)
		}
	}
	dataLen := len(typedData)

	startIndex := 0
	endIndex := dataLen
	// We iterate dataLen +1 times since we're writing the data after the date change on sharding_table:true
	for i := 0; i <= dataLen; i++ {
		if c.SQLConf.TableSharding && startIndex < len(typedData) {
			recDate := typedData[startIndex].TimeStamp.Format("20060102")
			var nextRecDate string
			// if we're on i == dataLen iteration, it means that we're out of index range. We're going to use the last record date.
			if i == dataLen {
				nextRecDate = typedData[dataLen-1].TimeStamp.Format("20060102")
			} else {
				nextRecDate = typedData[i].TimeStamp.Format("20060102")

				// if both dates are equal, we shouldn't write in the table yet.
				if recDate == nextRecDate {
					continue
				}
			}

			endIndex = i

			table := analytics.SQLTable + "_" + recDate
			c.db = c.db.Table(table)
			if errTable := c.ensureTable(table); errTable != nil {
				return errTable
			}
		} else {
			i = dataLen // write all records at once for non-sharded case, stop for loop after 1 iteration
		}

		recs := typedData[startIndex:endIndex]

		for i := 0; i < len(recs); i += c.SQLConf.BatchSize {
			ends := i + c.SQLConf.BatchSize
			if ends > len(recs) {
				ends = len(recs)
			}
			tx := c.db.WithContext(ctx).Create(recs[i:ends])
			if tx.Error != nil {
				c.log.Error(tx.Error)
			}
		}

		startIndex = i // next day start index, necessary for sharded case

	}

	c.log.Info("Purged ", len(data), " records...")

	return nil
}

func (c *SQLPump) WriteUptimeData(data []interface{}) {
	dataLen := len(data)
	c.log.Debug("Attempting to write ", dataLen, " records...")

	typedData := make([]analytics.UptimeReportData, len(data))

	for i, v := range data {
		decoded := analytics.UptimeReportData{}
		if err := msgpack.Unmarshal([]byte(v.(string)), &decoded); err != nil {
			// ToDo: should this work with serializer?
			c.log.Error("Couldn't unmarshal analytics data:", err)
			continue
		}
		typedData[i] = decoded
	}

	if len(typedData) == 0 {
		return
	}

	startIndex := 0
	endIndex := dataLen
	table = ""

	for i := 0; i <= dataLen; i++ {
		if c.SQLConf.TableSharding {
			recDate := typedData[startIndex].TimeStamp.Format("20060102")
			var nextRecDate string
			// if we're on i == dataLen iteration, it means that we're out of index range. We're going to use the last record date.
			if i == dataLen {
				nextRecDate = typedData[dataLen-1].TimeStamp.Format("20060102")
			} else {
				nextRecDate = typedData[i].TimeStamp.Format("20060102")

				// if both dates are equal, we shouldn't write in the table yet.
				if recDate == nextRecDate {
					continue
				}
			}

			endIndex = i

			table = analytics.UptimeSQLTable + "_" + recDate
			c.db = c.db.Table(table)
			if !c.db.Migrator().HasTable(table) {
				c.db.AutoMigrate(&analytics.UptimeReportAggregateSQL{})
			}
		} else {
			i = dataLen // write all records at once for non-sharded case, stop for loop after 1 iteration
			table = analytics.UptimeSQLTable
		}

		analyticsPerOrg := analytics.AggregateUptimeData(typedData[startIndex:endIndex])
		for orgID, ag := range analyticsPerOrg {
			recs := []analytics.UptimeReportAggregateSQL{}
			for _, d := range ag.Dimensions() {
				id := fmt.Sprintf("%v", ag.TimeStamp.Unix()) + orgID + d.Name + d.Value
				uID := hex.EncodeToString([]byte(id))

				rec := analytics.UptimeReportAggregateSQL{
					ID:             uID,
					OrgID:          orgID,
					TimeStamp:      ag.TimeStamp.Unix(),
					Counter:        *d.Counter,
					Dimension:      d.Name,
					DimensionValue: d.Value,
				}
				rec.ProcessStatusCodes(rec.Counter.ErrorMap)
				rec.Counter.ErrorList = nil
				rec.Counter.ErrorMap = nil

				recs = append(recs, rec)
			}

			for i := 0; i < len(recs); i += c.SQLConf.BatchSize {
				ends := i + c.SQLConf.BatchSize
				if ends > len(recs) {
					ends = len(recs)
				}
				tx := c.db.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "id"}},
					DoUpdates: clause.Assignments(analytics.OnConflictUptimeAssignments(table, "excluded")),
				}).Create(recs[i:ends])
				if tx.Error != nil {
					c.log.Error(tx.Error)
				}
			}

		}
		startIndex = i // next day start index, necessary for sharded case
	}

	c.log.Debug("Purged ", len(data), " records...")
}

func (c *SQLPump) buildIndexName(indexBaseName, tableName string) string {
	return fmt.Sprintf("%s_%s", tableName, indexBaseName)
}

func (c *SQLPump) createIndex(indexBaseName, tableName, column string) error {
	indexName := c.buildIndexName(indexBaseName, tableName)
	option := ""
	if c.dbType == "postgres" {
		option = "CONCURRENTLY"
	}

	columnExist := c.db.Migrator().HasColumn(&analytics.AnalyticsRecord{}, column)
	if !columnExist {
		return errors.New("cannot create index for non existent column " + column)
	}

	query := fmt.Sprintf("CREATE INDEX %s IF NOT EXISTS %s ON %s (%s)", option, indexName, tableName, column)

	err := c.db.Exec(query).Error
	if err != nil {
		c.log.WithFields(logrus.Fields{
			"index": indexName,
			"table": tableName,
		}).WithError(err).Error("Error creating index")
		return err
	}

	c.log.Infof("Index %s created for table %s", indexName, tableName)
	c.log.WithFields(logrus.Fields{
		"index": indexName,
		"table": tableName,
	}).Info("Index created")
	return nil
}

// ensureIndex check that all indexes for the analytics SQL table are in place
func (c *SQLPump) ensureIndex(tableName string, background bool) error {
	if !c.db.Migrator().HasTable(tableName) {
		return errors.New("cannot create indexes as table doesn't exist: " + tableName)
	}

	// waitgroup to facilitate testing and track when all indexes are created
	var wg sync.WaitGroup
	if background {
		wg.Add(len(indexes))
	}

	for _, idx := range indexes {
		indexName := tableName + idx.baseName

		if c.db.Migrator().HasIndex(tableName, indexName) {
			c.log.WithFields(logrus.Fields{
				"index": indexName,
				"table": tableName,
			}).Info("Index already exists")
			continue
		}

		if background {
			go func(baseName, cols string) {
				defer wg.Done()
				if err := c.createIndex(baseName, tableName, cols); err != nil {
					c.log.Error(err)
				}
			}(idx.baseName, idx.column)
		} else {
			if err := c.createIndex(idx.baseName, tableName, idx.column); err != nil {
				return err
			}
		}
	}

	if background {
		wg.Wait()
		c.backgroundIndexCreated <- true
	}
	return nil
}

// ensureTable creates the table if it doesn't exist
func (c *SQLPump) ensureTable(tableName string) error {
	if !c.db.Migrator().HasTable(tableName) {
		c.db = c.db.Table(tableName)
		if err := c.db.Migrator().CreateTable(&analytics.AnalyticsRecord{}); err != nil {
			c.log.Error("error creating table", err)
			return err
		}
		if err := c.ensureIndex(tableName, false); err != nil {
			return err
		}
	}
	return nil
}

// migrateAllShardedTables scans for all existing sharded tables and migrates them
func (c *SQLPump) migrateAllShardedTables() error {
	if !c.SQLConf.TableSharding {
		// No sharding, nothing to migrate
		return nil
	}

	c.log.Info("Scanning for existing sharded tables to migrate...")

	// Get all tables in the database
	var tables []string
	err := c.db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tables).Error
	if err != nil {
		c.log.WithError(err).Warn("Failed to get list of tables, skipping migration scan")
		return nil
	}

	// Find tables matching our sharded pattern
	shardedTables := make([]string, 0)
	tablePrefix := analytics.SQLTable + "_"

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

	c.log.WithField("count", len(shardedTables)).Info("Found sharded tables to migrate")

	// Migrate each sharded table
	for _, tableName := range shardedTables {
		c.log.WithField("table", tableName).Debug("Migrating sharded table")

		c.db = c.db.Table(tableName)
		if err := c.db.AutoMigrate(&analytics.AnalyticsRecord{}); err != nil {
			c.log.WithField("table", tableName).WithError(err).Warn("Failed to migrate sharded table")
			// Continue with other tables even if one fails
		} else {
			c.log.WithField("table", tableName).Debug("Successfully migrated sharded table")
		}
	}

	c.log.Info("Completed migration of sharded tables")
	return nil
}
