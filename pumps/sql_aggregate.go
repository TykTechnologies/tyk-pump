package pumps

import (
	"context"

	"github.com/TykTechnologies/tyk-pump/analytics"

	"github.com/mitchellh/mapstructure"

	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

type SQLAggregatePumpConf struct {
	SQLConf `mapstructure:",squash"`

	EnvPrefix               string   `mapstructure:"meta_env_prefix"`
	TrackAllPaths           bool     `mapstructure:"track_all_paths"`
	IgnoreTagPrefixList     []string `mapstructure:"ignore_tag_prefix_list"`
	ThresholdLenTagList     int      `mapstructure:"threshold_len_tag_list"`
	StoreAnalyticsPerMinute bool     `mapstructure:"store_analytics_per_minute"`
	IgnoreAggregationsList  []string `mapstructure:"ignore_aggregations"`
}

type SQLAggregatePump struct {
	CommonPumpConfig

	SQLConf *SQLAggregatePumpConf

	db      *gorm.DB
	dbType  string
	dialect gorm.Dialector
}

var SQLAggregatePumpPrefix = "SQL-aggregate-pump"
var SQLAggregateDefaultENV = PUMPS_ENV_PREFIX + "_SQLAGGREGATE" + PUMPS_ENV_META_PREFIX

func (c *SQLAggregatePump) New() Pump {
	newPump := SQLAggregatePump{}
	return &newPump
}

func (c *SQLAggregatePump) GetName() string {
	return "SQL Aggregate Pump"
}

func (c *SQLAggregatePump) GetEnvPrefix() string {
	return c.SQLConf.EnvPrefix
}

func (c *SQLAggregatePump) Init(conf interface{}) error {
	c.SQLConf = &SQLAggregatePumpConf{}
	c.log = log.WithField("prefix", SQLAggregatePumpPrefix)

	err := mapstructure.Decode(conf, &c.SQLConf)
	if err != nil {
		c.log.Error("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(c, c.log, c.SQLConf, SQLAggregateDefaultENV)

	logLevel := gorm_logger.Silent

	switch c.SQLConf.LogLevel {
	case "debug":
		logLevel = gorm_logger.Info
	case "info":
		logLevel = gorm_logger.Warn
	case "warning":
		logLevel = gorm_logger.Error
	}

	dialect, errDialect := Dialect(&c.SQLConf.SQLConf)
	if errDialect != nil {
		c.log.Error(errDialect)
		return errDialect
	}
	db, err := gorm.Open(dialect, &gorm.Config{
		AutoEmbedd:             true,
		UseJSONTags:            true,
		SkipDefaultTransaction: true,
		Logger:                 gorm_logger.Default.LogMode(logLevel),
	})

	if err != nil {
		c.log.Error(err)
		return err
	}
	c.db = db
	c.db.AutoMigrate(&analytics.SQLAnalyticsRecordAggregate{})

	c.log.Debug("SQLAggregate Initialized")
	return nil
}

func (c *SQLAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
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

	for i := 0; i < len(typedData); i += batch {
		j := i + batch
		if j > len(typedData) {
			j = len(typedData)
		}

		resp := c.db

		if c.SQLConf.TableSharding {
			table := "tyk_aggregated_" + typedData[i].TimeStamp.Format("20060102")
			if !c.db.Migrator().HasTable(table) {
				c.db.Table(table).AutoMigrate(&analytics.SQLAnalyticsRecordAggregate{})
			}
			resp = resp.Table(table)
		}

		analyticsPerOrg := analytics.AggregateData(data[i:j], c.SQLConf.TrackAllPaths, c.SQLConf.IgnoreTagPrefixList, c.SQLConf.StoreAnalyticsPerMinute)

		for orgID, ag := range analyticsPerOrg {
			// ag.DiscardAggregations([]string{"keyendpoints", "oauthendpoints", "apiendpoints"})

			for _, d := range ag.Dimensions() {
				rec := analytics.SQLAnalyticsRecordAggregate{
					OrgID:          orgID,
					TimeStamp:      ag.TimeStamp.Unix(),
					Counter:        *d.Counter,
					Dimension:      d.Name,
					DimensionValue: d.Value,
				}
				rec.ProcessStatusCodes()

				resp = resp.WithContext(ctx).Create(rec)
				if resp.Error != nil {
					return resp.Error
				}
			}
		}
	}

	c.log.Info("Purged ", len(data), " records...")

	return nil
}
