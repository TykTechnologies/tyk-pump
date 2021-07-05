package pumps

import (
	"context"
	"errors"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

type PostgresConfig struct {
	// disables implicit prepared statement usage
	PreferSimpleProtocol bool `json:"prefer_simple_protocol" mapstructure:"prefer_simple_protocol"`
}

type MysqlConfig struct {
	// default size for string fields. By default set to: 256
	DefaultStringSize uint `json:"default_string_size" mapstructure:"default_string_size"`
	// disable datetime precision, which not supported before MySQL 5.6
	DisableDatetimePrecision bool `json:"disable_datetime_precision" mapstructure:"disable_datetime_precision"`
	// drop & create when rename index, rename index not supported before MySQL 5.7, MariaDB
	DontSupportRenameIndex bool `json:"dont_support_rename_index" mapstructure:"dont_support_rename_index"`
	// `change` when rename column, rename column not supported before MySQL 8, MariaDB
	DontSupportRenameColumn bool `json:"dont_support_rename_column" mapstructure:"dont_support_rename_column"`
	// auto configure based on currently MySQL version
	SkipInitializeWithVersion bool `json:"skip_initialize_with_version" mapstructure:"skip_initialize_with_version"`
}

type SQLPump struct {
	CommonPumpConfig

	SQLConf *SQLConf

	db      *gorm.DB
	dbType  string
	dialect gorm.Dialector
}

type SQLConf struct {
	EnvPrefix        string         `mapstructure:"meta_env_prefix"`
	Type             string         `json:"type" mapstructure:"type"`
	ConnectionString string         `json:"connection_string" mapstructure:"connection_string"`
	Postgres         PostgresConfig `json:"postgres" mapstructure:"postgres"`
	Mysql            MysqlConfig    `json:"mysql" mapstructure:"mysql"`
	TableSharding    bool           `json:"table_sharding" mapstructure:"table_sharding"`
	LogLevel         string         `json:"log_level" mapstructure:"log_level"`
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

var SQLPrefix = "SQL-pump"
var SQLDefaultENV = PUMPS_ENV_PREFIX + "_SQL" + PUMPS_ENV_META_PREFIX

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

func (c *SQLPump) Init(conf interface{}) error {
	c.SQLConf = &SQLConf{}
	c.log = log.WithField("prefix", SQLPrefix)

	err := mapstructure.Decode(conf, &c.SQLConf)
	if err != nil {
		c.log.Error("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(c, c.log, c.SQLConf, SQLDefaultENV)

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
		c.db.Table("tyk_analytics").AutoMigrate(&analytics.AnalyticsRecord{})
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
	//We iterate dataLen +1 times since we're writing the data after the date change on sharding_table:true
	for i := 0; i <= dataLen; i++ {
		if c.SQLConf.TableSharding {
			recDate := data[startIndex].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
			var nextRecDate string
			//if we're on i == dataLen iteration, it means that we're out of index range. We're going to use the last record date.
			if i == dataLen {
				nextRecDate = data[dataLen-1].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
			} else {
				nextRecDate = data[i].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")

				//if both dates are equal, we shouldn't write in the table yet.
				if recDate == nextRecDate {
					continue
				}
			}

			endIndex = i

			table := "tyk_analytics_" + recDate
			c.db = c.db.Table(table)
			if !c.db.Migrator().HasTable(table) {
				c.db.AutoMigrate(&analytics.AnalyticsRecord{})
			}
		} else {
			i = dataLen // write all records at once for non-sharded case, stop for loop after 1 iteration
		}

		c.db = c.db.WithContext(ctx).Create(typedData[startIndex:endIndex])
		if c.db.Error != nil {
			return c.db.Error
		}
		startIndex = i // next day start index, necessary for sharded case

	}

	c.log.Info("Purged ", len(data), " records...")

	return nil
}
