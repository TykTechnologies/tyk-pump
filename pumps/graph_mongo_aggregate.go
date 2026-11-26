package pumps

import (
	"context"
	"errors"
	"time"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

var (
	graphMongoAggregatePumpPrefix = "PMP_GRAPH_MONGOAGG"
	graphMongoAggregateDefaultEnv = PUMPS_ENV_PREFIX + "_GRAPH_MONGOAGGREGATE" + PUMPS_ENV_META_PREFIX
)

type GraphMongoAggregatePump struct {
	MongoAggregatePump
}

func (m *GraphMongoAggregatePump) New() Pump {
	newPump := GraphMongoAggregatePump{}
	return &newPump
}

func (m *GraphMongoAggregatePump) GetName() string {
	return "Graph MongoDB Aggregate Pump"
}

func (m *GraphMongoAggregatePump) Init(config interface{}) error {
	m.dbConf = &MongoAggregateConf{}
	m.log = log.WithField("prefix", "graph-mongo-pump-aggregate")

	err := mapstructure.Decode(config, &m.dbConf)
	if err == nil {
		err = mapstructure.Decode(config, &m.dbConf.BaseMongoConf)
	}

	if err != nil {
		m.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(m, m.log, m.dbConf, graphMongoAggregateDefaultEnv)

	if m.dbConf.ThresholdLenTagList == 0 {
		m.dbConf.ThresholdLenTagList = ThresholdLenTagList
	}
	m.SetAggregationTime()

	m.connect()

	m.log.Debug("MongoDB DB CS: ", m.dbConf.GetBlurredURL())
	m.log.Info(m.GetName() + " Initialized")

	lastTimestampAgggregateRecord, err := m.getLastDocumentTimestamp()

	if err != nil {
		m.log.Debug("Last document timestamp not found:", err)
	} else {
		analytics.SetlastTimestampAgggregateRecord(m.dbConf.MongoURL, lastTimestampAgggregateRecord)
	}

	return nil
}

func (m *GraphMongoAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	filtered := make([]interface{}, 0, len(data))
	for _, d := range data {
		if rec, ok := d.(analytics.AnalyticsRecord); ok && rec.IsGraphRecord() {
			filtered = append(filtered, d)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	m.log.Debug("Attempting to write ", len(filtered), " graph records")

	// calculate aggregates
	analyticsPerAPI := analytics.AggregateGraphData(filtered, m.dbConf.MongoURL, m.dbConf.AggregationTime)

	writingAttempts := []bool{false}
	if m.dbConf.UseMixedCollection {
		writingAttempts = append(writingAttempts, true)
	}

	for apiID := range analyticsPerAPI {
		filteredData := analyticsPerAPI[apiID]
		for _, isMixedCollection := range writingAttempts {
			err := m.DoAggregatedWriting(ctx, &filteredData, isMixedCollection)
			if err != nil {
				if shouldSelfHeal := m.ShouldSelfHeal(err); shouldSelfHeal {
					newErr := m.WriteData(ctx, data)
					if newErr == nil {
						m.log.Info("Self-healing successful")
					}
					return newErr
				}
				return err
			}
		}
		m.log.Debug("Processed aggregated graph data for ", apiID)
	}

	m.log.Info("Purged ", len(filtered), " graph records...")

	return nil
}

func (m *GraphMongoAggregatePump) DoAggregatedWriting(ctx context.Context, filteredData *analytics.GraphRecordAggregate, mixed bool) error {
	filteredData.Mixed = mixed
	indexCreateErr := m.ensureIndexes(filteredData.TableName())

	if indexCreateErr != nil {
		m.log.Error(indexCreateErr)
	}

	query := model.DBM{
		"orgid":     filteredData.OrgID,
		"timestamp": filteredData.TimeStamp,
	}

	if len(m.dbConf.IgnoreAggregationsList) > 0 {
		filteredData.DiscardAggregations(m.dbConf.IgnoreAggregationsList)
	}

	updateDoc := filteredData.AsChange()
	doc := &analytics.GraphRecordAggregate{
		AnalyticsRecordAggregate: analytics.AnalyticsRecordAggregate{
			OrgID: filteredData.OrgID,
			Mixed: mixed,
		},
	}

	m.log.WithFields(logrus.Fields{
		"collection": doc.TableName(),
	}).Debug("Attempt to upsert aggregated graph doc")

	err := m.store.Upsert(ctx, doc, query, updateDoc)
	if err != nil {
		m.log.WithField("query", query).Error("UPSERT Failure: ", err)
		return err
	}

	avgUpdateDoc := doc.AsTimeUpdate()

	withTimeUpdate := analytics.GraphRecordAggregate{
		AnalyticsRecordAggregate: analytics.AnalyticsRecordAggregate{
			OrgID: filteredData.OrgID,
			Mixed: mixed,
		},
	}

	err = m.store.Upsert(ctx, &withTimeUpdate, query, avgUpdateDoc)
	if err != nil {
		m.log.WithField("query", query).Error("AvgUpdate Failure: ", err)
		return err
	}
	if m.dbConf.ThresholdLenTagList != -1 && (len(withTimeUpdate.Tags) > m.dbConf.ThresholdLenTagList) {
		m.printAlert(withTimeUpdate.AnalyticsRecordAggregate, m.dbConf.ThresholdLenTagList)
	}

	return nil
}

func (m *GraphMongoAggregatePump) getLastDocumentTimestamp() (time.Time, error) {
	d := dbObject{
		tableName: analytics.GraphAggregateMixedCollectionName,
	}

	var result model.DBM
	err := m.store.Query(context.Background(), d, &result, model.DBM{"_sort": "-$natural", "_limit": 1})
	if err != nil {
		return time.Time{}, err
	}
	if ts, ok := result["timestamp"].(time.Time); ok {
		return ts, nil
	}
	return time.Time{}, errors.New("timestamp of type: time.Time not found in query result")
}
