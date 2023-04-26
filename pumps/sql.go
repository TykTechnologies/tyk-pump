package pumps

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/vmihailenco/msgpack.v2"
	"gorm.io/gorm/clause"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
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
}

// @PumpConf SQL
type SQLConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The supported and tested types are `sqlite` and `postgres`.
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
}

func Dialect(cfg *SQLConf) (gorm.Dialector, error) {
	switch cfg.Type {
	case "sqlite":
		if cfg.ConnectionString == "" {
			log.Warning("`meta.connection_string` is empty. Falling back to in-memory storage. Warning: All data will be lost on process restart.")
			cfg.ConnectionString = "file::memory:?cache=shared"
		}

		return sqlite.Open(cfg.ConnectionString), nil
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

			table := analytics.SQLTable + "_" + recDate
			c.db = c.db.Table(table)
			if !c.db.Migrator().HasTable(table) {
				c.db.AutoMigrate(&analytics.AnalyticsRecord{})
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
