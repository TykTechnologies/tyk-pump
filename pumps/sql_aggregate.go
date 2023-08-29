package pumps

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/TykTechnologies/tyk-pump/analytics"

	"github.com/mitchellh/mapstructure"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gorm_logger "gorm.io/gorm/logger"
)

// @PumpConf SQLAggregate
type SQLAggregatePumpConf struct {
	// TYKCONFIGEXPAND
	SQLConf `mapstructure:",squash"`

	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// Specifies if it should store aggregated data for all the endpoints. By default, `false`
	// which means that only store aggregated data for `tracked endpoints`.
	TrackAllPaths bool `json:"track_all_paths" mapstructure:"track_all_paths"`
	// Specifies prefixes of tags that should be ignored.
	IgnoreTagPrefixList []string `json:"ignore_tag_prefix_list" mapstructure:"ignore_tag_prefix_list"`
	ThresholdLenTagList int      `json:"threshold_len_tag_list" mapstructure:"threshold_len_tag_list"`
	// Determines if the aggregations should be made per minute instead of per hour.
	StoreAnalyticsPerMinute bool     `json:"store_analytics_per_minute" mapstructure:"store_analytics_per_minute"`
	IgnoreAggregationsList  []string `json:"ignore_aggregations" mapstructure:"ignore_aggregations"`
	// Set to true to disable the default tyk index creation.
	OmitIndexCreation bool `json:"omit_index_creation" mapstructure:"omit_index_creation"`
}

type SQLAggregatePump struct {
	CommonPumpConfig
	SQLConf *SQLAggregatePumpConf
	db      *gorm.DB
	dbType  string
	dialect gorm.Dialector

	// this channel is used to signal that the background index creation has finished - this is used for testing
	backgroundIndexCreated chan bool
}

var (
	SQLAggregatePumpPrefix = "SQL-aggregate-pump"
	SQLAggregateDefaultENV = PUMPS_ENV_PREFIX + "_SQLAGGREGATE" + PUMPS_ENV_META_PREFIX
)

const (
	oldAggregatedIndexName = "dimension"
	newAggregatedIndexName = "idx_dimension"
)

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

func (c *SQLAggregatePump) SetDecodingRequest(decoding bool) {
	if decoding {
		log.WithField("pump", c.GetName()).Warn("Decoding request is not supported for SQL Aggregate pump")
	}
}

func (c *SQLAggregatePump) SetDecodingResponse(decoding bool) {
	if decoding {
		log.WithField("pump", c.GetName()).Warn("Decoding response is not supported for SQL Aggregate pump")
	}
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
	c.dbType = c.SQLConf.Type

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
		// if table doesn't exist, create it
		if err := c.ensureTable(analytics.AggregateSQLTable); err != nil {
			return err
		}

		// we can run the index creation in background only for postgres since it supports CONCURRENTLY
		shouldRunOnBackground := false
		if c.dbType == "postgres" {
			shouldRunOnBackground = true
			c.backgroundIndexCreated = make(chan bool, 1)
		}

		// if index doesn't exist, create it
		if err := c.ensureIndex(analytics.AggregateSQLTable, shouldRunOnBackground); err != nil {
			c.log.Error(err)
			return err
		}
	}

	if c.SQLConf.BatchSize == 0 {
		c.SQLConf.BatchSize = SQLDefaultQueryBatchSize
	}

	c.log.Debug("SQLAggregate Initialized")
	return nil
}

// ensureIndex creates the new optimized index for tyk_aggregated.
// it uses CONCURRENTLY to avoid locking the table for a long time - postgresql.org/docs/current/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY
// if background is true, it will run the index creation in a goroutine
// if not, it will block until it finishes
func (c *SQLAggregatePump) ensureIndex(tableName string, background bool) error {
	if c.SQLConf.OmitIndexCreation {
		c.log.Info("omit_index_creation set to true, omitting index creation..")
		return nil
	}

	if !c.db.Migrator().HasIndex(tableName, newAggregatedIndexName) {
		createIndexFn := func(c *SQLAggregatePump) error {
			option := ""
			if c.dbType == "postgres" {
				option = "CONCURRENTLY"
			}

			err := c.db.Table(tableName).Exec(fmt.Sprintf("CREATE INDEX %s IF NOT EXISTS %s ON %s (dimension, timestamp, org_id, dimension_value)", option, newAggregatedIndexName, tableName)).Error
			if err != nil {
				c.log.Errorf("error creating index for table %s : %s", tableName, err.Error())
				return err
			}

			if background {
				c.backgroundIndexCreated <- true
			}

			c.log.Info("Index ", newAggregatedIndexName, " for table ", tableName, " created successfully")

			return nil
		}

		if background {
			c.log.Info("Creating index for table ", tableName, " on background...")

			go func(c *SQLAggregatePump) {
				if err := createIndexFn(c); err != nil {
					c.log.Error(err)
				}
			}(c)

			return nil
		}

		c.log.Info("Creating index for table ", tableName, "...")
		return createIndexFn(c)
	}
	c.log.Info(newAggregatedIndexName, " already exists.")

	return nil
}

// ensureTable creates the table if it doesn't exist
func (c *SQLAggregatePump) ensureTable(tableName string) error {
	if !c.db.Migrator().HasTable(tableName) {
		c.db = c.db.Table(tableName)

		if err := c.db.Migrator().CreateTable(&analytics.SQLAnalyticsRecordAggregate{}); err != nil {
			c.log.Error("error creating table", err)
			return err
		}
	}
	return nil
}

// WriteData aggregates and writes the passed data to SQL database. When table sharding is enabled, startIndex and endIndex
// are found by checking timestamp of the records. The main for loop iterates and finds the index where a new day starts.
// Then, the data is passed to AggregateData function and written to database day by day on different tables. However,
// if table sharding is not enabled, the for loop iterates one time and all data is passed at once to the AggregateData
// function and written to database on single table.
func (c *SQLAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	dataLen := len(data)
	c.log.Debug("Attempting to write ", dataLen, " records...")

	if dataLen == 0 {
		return nil
	}

	startIndex := 0
	endIndex := dataLen
	table := ""
	for i := 0; i <= dataLen; i++ {
		if c.SQLConf.TableSharding {
			recDate := data[startIndex].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
			var nextRecDate string
			// if we're on i == dataLen iteration, it means that we're out of index range. We're going to use the last record date.
			if i == dataLen {
				nextRecDate = data[dataLen-1].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
			} else {
				nextRecDate = data[i].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")

				// if both dates are equal, we shouldn't write in the table yet.
				if recDate == nextRecDate {
					continue
				}
			}

			endIndex = i

			table = analytics.AggregateSQLTable + "_" + recDate
			c.db = c.db.Table(table)
			if errTable := c.ensureTable(table); errTable != nil {
				return errTable
			}
			if err := c.ensureIndex(table, false); err != nil {
				return err
			}
		} else {
			i = dataLen // write all records at once for non-sharded case, stop for loop after 1 iteration
			table = analytics.AggregateSQLTable
		}

		// if StoreAnalyticsPerMinute is set to true, we will create new documents with records every 1 minute
		var aggregationTime int
		if c.SQLConf.StoreAnalyticsPerMinute {
			aggregationTime = 1
		} else {
			aggregationTime = 60
		}

		analyticsPerOrg := analytics.AggregateData(data[startIndex:endIndex], c.SQLConf.TrackAllPaths, c.SQLConf.IgnoreTagPrefixList, "", aggregationTime)

		for orgID, ag := range analyticsPerOrg {

			err := c.DoAggregatedWriting(ctx, table, orgID, ag)
			if err != nil {
				return err
			}
		}

		startIndex = i // next day start index, necessary for sharded case
	}

	c.log.Info("Purged ", dataLen, " records...")

	return nil
}

func (c *SQLAggregatePump) DoAggregatedWriting(ctx context.Context, table, orgID string, ag analytics.AnalyticsRecordAggregate) error {
	recs := []analytics.SQLAnalyticsRecordAggregate{}

	dimensions := ag.Dimensions()
	for _, d := range dimensions {
		uID := hex.EncodeToString([]byte(fmt.Sprintf("%v", ag.TimeStamp.Unix()) + orgID + d.Name + d.Value))
		rec := analytics.SQLAnalyticsRecordAggregate{
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

		// we use excluded as temp  table since it's supported by our SQL storages https://www.postgresql.org/docs/9.5/sql-insert.html#SQL-ON-CONFLICT  https://www.sqlite.org/lang_UPSERT.html
		tx := c.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(analytics.OnConflictAssignments(table, "excluded")),
		}).Create(recs[i:ends])
		if tx.Error != nil {
			c.log.Error("error writing aggregated records into "+table+":", tx.Error)
			return tx.Error
		}
	}

	return nil
}
