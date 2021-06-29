package pumps

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

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
	IsUptime bool

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
			c.db.Table("tyk_uptime_analytics").AutoMigrate(&analytics.UptimeReportAggregateSQL{})
		} else {
			c.db.Table("tyk_analytics").AutoMigrate(&analytics.AnalyticsRecord{})
		}
	}

	c.log.Debug("SQL Initialized")
	return nil
}

func (c *SQLPump) WriteData(ctx context.Context, data []interface{}) error {
	c.log.Debug("Attempting to write ", len(data), " records...")

	var typedData []*analytics.AnalyticsRecord

	batch := 500
	newBatch := false
	for i, r := range data {
		rec := r.(analytics.AnalyticsRecord)
		typedData = append(typedData, &rec)

		if c.SQLConf.TableSharding && !newBatch && typedData[0].TimeStamp.Format("20060102") < rec.TimeStamp.Format("20060102") {
			batch = i
			newBatch = true
		}
	}

	if c.SQLConf.TableSharding && len(typedData) > 0 {
		// Check first and last record, to ensure that we will not hit issue when hour is changing
		table := "tyk_analytics_" + typedData[0].TimeStamp.Format("20060102")
		if !c.db.Migrator().HasTable(table) {
			c.db.Table(table).AutoMigrate(&analytics.AnalyticsRecord{})
		}

		table = "tyk_analytics_" + typedData[len(typedData)-1].TimeStamp.Format("20060102")
		if !c.db.Migrator().HasTable(table) {
			c.db.Table(table).AutoMigrate(&analytics.AnalyticsRecord{})
		}
	}

	for i := 0; i < len(typedData); i += batch {
		j := i + batch
		if j > len(typedData) {
			j = len(typedData)
		}

		resp := c.db

		if c.SQLConf.TableSharding {
			table := "tyk_analytics_" + typedData[i].TimeStamp.Format("20060102")
			resp = resp.Table(table)
		}

		resp = resp.WithContext(ctx).Create(typedData[i:j])
		if resp.Error != nil {
			c.log.Error(resp.Error)
			return resp.Error
		}
	}

	c.log.Info("Purged ", len(data), " records...")

	return nil
}

func (c *SQLPump) WriteUptimeData(data []interface{}) {
	c.log.Debug("Attempting to write ", len(data), " records...")

	var typedData []analytics.UptimeReportData

	batch := 500
	newBatch := false
	var firstDate time.Time
	for i, r := range data {

		decoded := analytics.UptimeReportData{}

		if err := msgpack.Unmarshal([]byte(r.(string)), &decoded); err != nil {
			c.log.Error("Couldn't unmarshal uptime analytics data:", err)

			continue
		}
		if i == 0 {
			firstDate = decoded.TimeStamp
		}

		typedData = append(typedData, decoded)

		if c.SQLConf.TableSharding && !newBatch && firstDate.Format("20060102") < decoded.TimeStamp.Format("20060102") {
			batch = i
			newBatch = true
		}
	}

	for i := 0; i < len(typedData); i += batch {
		j := i + batch
		if j > len(typedData) {
			j = len(typedData)
		}

		resp := c.db

		if c.SQLConf.TableSharding {
			table := "tyk_uptime_analytics_" + typedData[i].TimeStamp.Format("20060102")
			if !c.db.Migrator().HasTable(table) {
				c.db.Table(table).AutoMigrate(&analytics.UptimeReportAggregateSQL{})
			}
			resp = resp.Table(table)
		}

		analyticsPerOrg := analytics.AggregateUptimeData(typedData[i:j])
		for orgID, ag := range analyticsPerOrg {

			for _, d := range ag.Dimensions() {
				id := fmt.Sprint(ag.TimeStamp.Unix()) + d.Name + d.Value
				uID := hex.EncodeToString([]byte(id))

				rec := analytics.UptimeReportAggregateSQL{
					ID:             uID,
					OrgID:          orgID,
					TimeStamp:      ag.TimeStamp.Unix(),
					Counter:        *d.Counter,
					Dimension:      d.Name,
					DimensionValue: d.Value,
				}

				rec.ProcessStatusCodes()

				resp = resp.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "id"}},
					DoUpdates: clause.Assignments(rec.GetAssignments()),
				}).Create(rec)
				if resp.Error != nil {
					c.log.Error(resp.Error)
				}
			}
		}

	}

	c.log.Debug("Purged ", len(data), " records...")

}
