package pumps

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const mcpSQLAggregatePrefix = "sql-mcp-aggregate-pump"

var SQLMCPAggregateDefaultENV = PUMPS_ENV_PREFIX + "_SQLMCPAGGREGATE" + PUMPS_ENV_META_PREFIX

// MCPSQLAggregatePump writes aggregated MCP analytics to a dedicated SQL table.
type MCPSQLAggregatePump struct {
	SQLConf *SQLAggregatePumpConf
	db      *gorm.DB
	dbType  string

	// this channel is used to signal that the background index creation has finished - this is used for testing
	backgroundIndexCreated chan bool

	CommonPumpConfig
}

func (s *MCPSQLAggregatePump) GetName() string {
	return "SQL MCP Aggregate Pump"
}

func (s *MCPSQLAggregatePump) GetEnvPrefix() string {
	return s.SQLConf.EnvPrefix
}

func (s *MCPSQLAggregatePump) New() Pump {
	return &MCPSQLAggregatePump{}
}

func (s *MCPSQLAggregatePump) Init(conf interface{}) error {
	s.SQLConf = &SQLAggregatePumpConf{}
	s.log = log.WithField("prefix", mcpSQLAggregatePrefix)

	err := mapstructure.Decode(conf, s.SQLConf)
	if err != nil {
		s.log.Error("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(s, s.log, s.SQLConf, SQLMCPAggregateDefaultENV)

	db, err := OpenGormDB(&s.SQLConf.SQLConf, s.log)
	if err != nil {
		return err
	}
	s.db = db
	s.dbType = s.SQLConf.Type

	if !s.SQLConf.TableSharding {
		if err := s.ensureTable(analytics.AggregateMCPSQLTable); err != nil {
			return err
		}

		shouldRunOnBackground := false
		if s.dbType == "postgres" {
			shouldRunOnBackground = true
			s.backgroundIndexCreated = make(chan bool, 1)
		}

		if err := s.ensureIndex(analytics.AggregateMCPSQLTable, shouldRunOnBackground); err != nil {
			s.log.Error(err)
			return err
		}
	}

	if s.SQLConf.BatchSize == 0 {
		s.SQLConf.BatchSize = SQLDefaultQueryBatchSize
	}

	s.log.Debug("MCP SQL Aggregate Pump Initialized")
	return nil
}

// ensureTable creates the table if it doesn't exist.
func (s *MCPSQLAggregatePump) ensureTable(tableName string) error {
	if !s.db.Migrator().HasTable(tableName) {
		s.db = s.db.Table(tableName)
		if err := s.db.Migrator().CreateTable(&analytics.MCPSQLAnalyticsRecordAggregate{}); err != nil {
			s.log.Error("error creating table", err)
			return err
		}
	}
	return nil
}

// ensureIndex creates the composite index on (dimension, timestamp, org_id, dimension_value).
// For PostgreSQL it uses CONCURRENTLY to avoid locking the table.
func (s *MCPSQLAggregatePump) ensureIndex(tableName string, background bool) error {
	if s.SQLConf.OmitIndexCreation {
		s.log.Info("omit_index_creation set to true, omitting index creation..")
		return nil
	}
	indexName := fmt.Sprintf("%s_%s", tableName, newAggregatedIndexName)
	if !s.db.Migrator().HasIndex(tableName, indexName) {
		createIndexFn := func() error {
			option := ""
			if s.dbType == "postgres" {
				option = "CONCURRENTLY"
			}

			err := s.db.Table(tableName).Exec(fmt.Sprintf("CREATE INDEX %s IF NOT EXISTS %s ON %s (dimension, timestamp, org_id, dimension_value)", option, indexName, tableName)).Error
			if err != nil {
				s.log.Errorf("error creating index for table %s : %s", tableName, err.Error())
				return err
			}

			if background {
				s.backgroundIndexCreated <- true
			}

			s.log.Info("Index ", indexName, " for table ", tableName, " created successfully")
			return nil
		}

		if background {
			s.log.Info("Creating index for table ", tableName, " on background...")
			go func() {
				if err := createIndexFn(); err != nil {
					s.log.Error(err)
				}
			}()
			return nil
		}

		s.log.Info("Creating index for table ", tableName, "...")
		return createIndexFn()
	}

	return nil
}

// aggregationTimeMinutes returns the aggregation window in minutes based on config.
func (s *MCPSQLAggregatePump) aggregationTimeMinutes() int {
	if s.SQLConf.StoreAnalyticsPerMinute {
		return 1
	}
	return 60
}

// ensureMCPAggregateShardedTable switches to the date-specific shard and creates it if absent.
func (s *MCPSQLAggregatePump) ensureMCPAggregateShardedTable(recDate string) string {
	table := analytics.AggregateMCPSQLTable + "_" + recDate
	s.db = s.db.Table(table)
	if !s.db.Migrator().HasTable(table) {
		if err := s.db.Migrator().CreateTable(&analytics.MCPSQLAnalyticsRecordAggregate{}); err != nil {
			s.log.WithError(err).Warn("error creating sharded table")
		}
	}
	return table
}

// writeAggregatedSlice aggregates and writes a slice of records to the given table.
func (s *MCPSQLAggregatePump) writeAggregatedSlice(ctx context.Context, data []interface{}, table string) error {
	analyticsPerAPI := analytics.AggregateMCPData(data, "", s.aggregationTimeMinutes())
	for apiID := range analyticsPerAPI {
		ag := analyticsPerAPI[apiID]
		if err := s.DoAggregatedWriting(ctx, table, ag.OrgID, apiID, &ag); err != nil {
			s.log.WithError(err).Error("error writing record")
			return err
		}
	}
	return nil
}

func (s *MCPSQLAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	dataLen := len(data)
	s.log.Debug("Attempting to write ", dataLen, " records...")

	if dataLen == 0 {
		return nil
	}

	mcpRecordCount := 0
	for _, item := range data {
		if r, ok := item.(analytics.AnalyticsRecord); ok && r.IsMCPRecord() {
			mcpRecordCount++
		}
	}

	startIndex := 0
	endIndex := dataLen
	table := ""

	for i := 0; i <= dataLen; i++ {
		if s.SQLConf.TableSharding {
			recDate := data[startIndex].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
			var nextRecDate string
			if i == dataLen {
				nextRecDate = data[dataLen-1].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
				recDate = nextRecDate
			} else {
				nextRecDate = data[i].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
				if recDate == nextRecDate {
					continue
				}
			}
			endIndex = i
			table = s.ensureMCPAggregateShardedTable(recDate)
		} else {
			i = dataLen
			table = analytics.AggregateMCPSQLTable
		}

		if err := s.writeAggregatedSlice(ctx, data[startIndex:endIndex], table); err != nil {
			return err
		}
		startIndex = i
	}

	s.log.Info("Purged ", mcpRecordCount, " records...")
	return nil
}

func (s *MCPSQLAggregatePump) DoAggregatedWriting(ctx context.Context, table, orgID, apiID string, ag *analytics.MCPRecordAggregate) error {
	var recs []analytics.MCPSQLAnalyticsRecordAggregate

	dimensions := ag.Dimensions()
	for _, d := range dimensions {
		rec := analytics.MCPSQLAnalyticsRecordAggregate{
			ID:             hex.EncodeToString([]byte(fmt.Sprintf("%v", ag.TimeStamp.Unix()) + apiID + d.Name + d.Value)),
			OrgID:          orgID,
			APIID:          apiID,
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
