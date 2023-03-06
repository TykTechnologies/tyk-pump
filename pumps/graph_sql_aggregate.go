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

var SQLGraphAggregateDefaultENV = PUMPS_ENV_PREFIX + "_SQLGRAPHAGGREGATE" + PUMPS_ENV_META_PREFIX

type GraphSQLAggregatePump struct {
	SQLConf *SQLAggregatePumpConf
	db      *gorm.DB

	CommonPumpConfig
}

func (s *GraphSQLAggregatePump) GetName() string {
	return "Sql Graph Aggregate Pump"
}

func (s *GraphSQLAggregatePump) New() Pump {
	return &GraphSQLAggregatePump{}
}

func (s *GraphSQLAggregatePump) Init(conf interface{}) error {
	s.SQLConf = &SQLAggregatePumpConf{}
	s.log = log.WithField("prefix", SQLAggregatePumpPrefix)

	err := mapstructure.Decode(conf, &s.SQLConf)
	if err != nil {
		s.log.Error("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(s, s.log, s.SQLConf, SQLGraphAggregateDefaultENV)

	logLevel := gorm_logger.Silent

	switch s.SQLConf.LogLevel {
	case "debug":
		logLevel = gorm_logger.Info
	case "info":
		logLevel = gorm_logger.Warn
	case "warning":
		logLevel = gorm_logger.Error
	}

	dialect, errDialect := Dialect(&s.SQLConf.SQLConf)
	if errDialect != nil {
		s.log.Error(errDialect)
		return errDialect
	}
	db, err := gorm.Open(dialect, &gorm.Config{
		AutoEmbedd:  true,
		UseJSONTags: true,
		Logger:      gorm_logger.Default.LogMode(logLevel),
	})
	if err != nil {
		s.log.Error(err)
		return err
	}
	s.db = db
	if !s.SQLConf.TableSharding {
		if err := s.db.Table(analytics.AggregateGraphSQLTable).AutoMigrate(&analytics.SQLAnalyticsRecordAggregate{}); err != nil {
			s.log.WithError(err).Warn("error migrating table")
		}
	}

	if s.SQLConf.BatchSize == 0 {
		s.SQLConf.BatchSize = SQLDefaultQueryBatchSize
	}

	s.log.Debug("SQLAggregate Initialized")
	return nil
}

func (s *GraphSQLAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	dataLen := len(data)
	s.log.Debug("Attempting to write ", dataLen, " records...")

	if dataLen == 0 {
		return nil
	}

	startIndex := 0
	endIndex := dataLen
	table := ""

	for i := 0; i <= dataLen; i++ {
		if s.SQLConf.TableSharding {
			recDate := data[startIndex].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
			var nextRecDate string
			// if we're on i == dataLen iteration, it means that we're out of index range. We're going to use the last record date.
			if i == dataLen {
				nextRecDate = data[dataLen-1].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
				recDate = nextRecDate
			} else {
				nextRecDate = data[i].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")

				// if both dates are equal, we shouldn't write in the table yet.
				if recDate == nextRecDate {
					continue
				}
			}

			endIndex = i

			table = analytics.AggregateGraphSQLTable + "_" + recDate
			s.db = s.db.Table(table)
			if !s.db.Migrator().HasTable(table) {
				if err := s.db.AutoMigrate(&analytics.SQLAnalyticsRecordAggregate{}); err != nil {
					s.log.WithError(err).Warn("error running auto migration")
				}
			}
		} else {
			i = dataLen // write all records at once for non-sharded case, stop for loop after 1 iteration
			table = analytics.AggregateGraphSQLTable
		}

		// if StoreAnalyticsPerMinute is set to true, we will create new documents with records every 1 minute
		var aggregationTime int
		if s.SQLConf.StoreAnalyticsPerMinute {
			aggregationTime = 1
		} else {
			aggregationTime = 60
		}

		analyticsPerOrg := analytics.AggregateGraphData(data[startIndex:endIndex], "", aggregationTime)

		for orgID := range analyticsPerOrg {
			ag := analyticsPerOrg[orgID]
			err := s.DoAggregatedWriting(ctx, table, orgID, &ag)
			if err != nil {
				s.log.WithError(err).Error("error writing record")
				return err
			}
		}

		startIndex = i // next day start index, necessary for sharded case
	}
	s.log.Info("Purged ", dataLen, " records...")

	return nil
}

func (s *GraphSQLAggregatePump) DoAggregatedWriting(ctx context.Context, table, orgID string, ag *analytics.GraphRecordAggregate) error {
	recs := []analytics.SQLAnalyticsRecordAggregate{}

	dimensions := ag.Dimensions()
	for _, d := range dimensions {
		rec := analytics.SQLAnalyticsRecordAggregate{
			ID:             hex.EncodeToString([]byte(fmt.Sprintf("%v", ag.TimeStamp.Unix()) + orgID + d.Name + d.Value)),
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

	for i := 0; i < len(recs); i += s.SQLConf.BatchSize {
		ends := i + s.SQLConf.BatchSize
		if ends > len(recs) {
			ends = len(recs)
		}

		// we use excluded as temp  table since it's supported by our SQL storages https://www.postgresql.org/docs/9.5/sql-insert.html#SQL-ON-CONFLICT  https://www.sqlite.org/lang_UPSERT.html
		tx := s.db.WithContext(ctx).Table(table).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(analytics.OnConflictAssignments(table, "excluded")),
		}).Create(recs[i:ends])
		if tx.Error != nil {
			s.log.Error("error writing aggregated records into "+table+":", tx.Error)
			return tx.Error
		}
	}

	return nil
}
