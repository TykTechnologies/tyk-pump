package pumps

import (
	"context"
	"os"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"

	"github.com/mitchellh/mapstructure"

	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
)

type SQLAggregatePumpConf struct {
	SQLConf `mapstructure:",squash"`

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

func (c *SQLAggregatePump) New() Pump {
	newPump := SQLAggregatePump{}
	return &newPump
}

func (c *SQLAggregatePump) GetName() string {
	return "SQL Pump"
}

func (c *SQLAggregatePump) Init(conf interface{}) error {
	c.SQLConf = &SQLAggregatePumpConf{}
	err := mapstructure.Decode(conf, &c.SQLConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": SQLAggregatePumpPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	logLevel := gorm_logger.Silent

	switch os.Getenv("TYK_LOGLEVEL") {
	case "debug":
		logLevel = gorm_logger.Info
	case "info":
		logLevel = gorm_logger.Warn
	case "warning":
		logLevel = gorm_logger.Error
	}

	db, err := gorm.Open(Dialect(&c.SQLConf.SQLConf), &gorm.Config{
		AutoEmbedd:             true,
		UseJSONTags:            true,
		SkipDefaultTransaction: true,
		Logger:                 gorm_logger.Default.LogMode(logLevel),
	})

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": SQLAggregatePumpPrefix,
		}).Error(err)
		return err
	}
	c.db = db
	c.db.AutoMigrate(&analytics.SQLAnalyticsRecordAggregate{})

	log.WithFields(logrus.Fields{
		"prefix": SQLAggregatePumpPrefix,
	}).Debug("SQL Initialized")
	return nil
}

func (c *SQLAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	analyticsPerOrg := analytics.AggregateData(data, c.SQLConf.TrackAllPaths, c.SQLConf.IgnoreTagPrefixList, c.SQLConf.StoreAnalyticsPerMinute)

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

			resp := c.db.Create(rec)
			if resp.Error != nil {
				panic(resp.Error)
			}
		}
	}

	return nil
}
