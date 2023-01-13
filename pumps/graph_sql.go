package pumps

import (
	"context"
	"fmt"
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

type GraphSQLPump struct {
	CommonPumpConfig

	SQLConf SQLConf

	db      *gorm.DB
	dialect gorm.Dialector

	tableName string
}

func (g *GraphSQLPump) GetName() string {
	return "Graph SQL Pump"
}

func (g *GraphSQLPump) New() Pump {
	return &GraphSQLPump{}
}

func (g *GraphSQLPump) Init(conf interface{}) error {
	g.log = log.WithField("prefix", GraphSQLPrefix)

	if err := mapstructure.Decode(conf, &g.SQLConf); err != nil {
		g.log.WithError(err).Error("error decoding conf")
		return fmt.Errorf("error decoding conf: %w", err)
	}

	logLevel := gorm_logger.Silent

	switch g.SQLConf.LogLevel {
	case "debug":
		logLevel = gorm_logger.Info
	case "info":
		logLevel = gorm_logger.Warn
	case "warning":
		logLevel = gorm_logger.Error
	}

	dialect, errDialect := Dialect(&g.SQLConf)
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

	if g.SQLConf.BatchSize == 0 {
		g.SQLConf.BatchSize = SQLDefaultQueryBatchSize
	}

	g.tableName = GraphSQLTable
	if name := g.SQLConf.TableName; name != "" {
		g.tableName = name
	}
	if !g.SQLConf.TableSharding {
		if err := g.db.Table(g.tableName).AutoMigrate(&analytics.GraphRecord{}); err != nil {
			g.log.WithError(err).Error("error migrating graph analytics table")
			return err
		}
	}
	g.db = g.db.Table(g.tableName)

	g.log.Debug("pump initialized and table set up")
	return nil
}

func (g *GraphSQLPump) WriteData(ctx context.Context, data []interface{}) error {
	g.log.Debug("Attempting to write ", len(data), " records...")

	var graphRecords []*analytics.GraphRecord
	for _, r := range data {
		if r != nil {
			rec := r.(analytics.AnalyticsRecord)
			if !rec.IsGraphRecord() {
				continue
			}
			gr, err := rec.ToGraphRecord()
			if err != nil {
				g.log.Warnf("error converting 1 record")
				g.log.WithError(err).Debug("error converting record")
				continue
			}
			graphRecords = append(graphRecords, &gr)
		}
	}
	dataLen := len(graphRecords)

	startIndex := 0
	endIndex := dataLen
	//We iterate dataLen +1 times since we're writing the data after the date change on sharding_table:true
	for i := 0; i <= dataLen; i++ {
		if g.SQLConf.TableSharding {
			recDate := graphRecords[startIndex].AnalyticsRecord.TimeStamp.Format("20060102")
			var nextRecDate string
			//if we're on i == dataLen iteration, it means that we're out of index range. We're going to use the last record date.
			if i == dataLen {
				nextRecDate = graphRecords[dataLen-1].AnalyticsRecord.TimeStamp.Format("20060102")
			} else {
				nextRecDate = graphRecords[i].AnalyticsRecord.TimeStamp.Format("20060102")

				//if both dates are equal, we shouldn't write in the table yet.
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
					continue
				}
			}
		} else {
			i = dataLen // write all records at once for non-sharded case, stop for loop after 1 iteration
		}

		recs := graphRecords[startIndex:endIndex]

		for i := 0; i < len(recs); i += g.SQLConf.BatchSize {
			ends := i + g.SQLConf.BatchSize
			if ends > len(recs) {
				ends = len(recs)
			}
			tx := g.db.WithContext(ctx).Create(recs[i:ends])
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
