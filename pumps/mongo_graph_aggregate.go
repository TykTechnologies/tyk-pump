package pumps

import (
	"context"
	"errors"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type MongoGraphAggregatePump struct {
	MongoAggregatePump
	CommonPumpConfig
}

func (m *MongoGraphAggregatePump) New() Pump {
	return &MongoGraphAggregatePump{}
}

func (m *MongoGraphAggregatePump) GetEnvPrefix() string {
	return m.dbConf.EnvPrefix
}

func (m *MongoGraphAggregatePump) GetName() string {
	return "MongoDB Graph Aggregated Pump"
}

func (m *MongoGraphAggregatePump) Init(config interface{}) error {
	if err := m.MongoAggregatePump.Init(config); err != nil {
		return err
	}
	m.log = log.WithField("prefix", analytics.MongoGraphAggregatePrefix)
	return nil
}

func (m *MongoGraphAggregatePump) WriteData(ctx context.Context, data []interface{}) error {
	if m.dbSession == nil {
		m.log.Debug("Connecting to analytics store")
		m.connect()
		if err := m.WriteData(ctx, data); err != nil {
			return err
		}
	} else {
		// only calculate graph analytics
		filtered := make([]interface{}, 0)
		for _, t := range data {
			rec, ok := t.(analytics.AnalyticsRecord)
			if !ok {
				continue
			}
			if rec.IsGraphRecord() {
				filtered = append(filtered, t)
			}
		}
		m.log.Debugf("Attempting to write %d records", len(filtered))
		analyticsPerOrg := analytics.AggregateData(filtered, m.dbConf.TrackAllPaths, m.dbConf.IgnoreTagPrefixList, m.dbConf.StoreAnalyticsPerMinute, false)

		// put aggregated data into MongoDB
		for orgID, record := range analyticsPerOrg {
			err := m.DoAggregatedWriting(ctx, orgID, record)
			if err != nil {
				return err
			}

			m.log.Debug("Processed aggregated data for ", orgID)
		}
	}
	return nil
}

func (m *MongoGraphAggregatePump) DoAggregatedWriting(ctx context.Context, orgID string, filteredData analytics.AnalyticsRecordAggregate) error {
	collectionName, collErr := m.GetCollectionName(orgID)
	if collErr != nil {
		m.log.Info("No OrgID for AnalyticsRecord, skipping")
		return nil
	}

	thisSession := m.dbSession.Copy()
	defer thisSession.Close()

	analyticsCollection := thisSession.DB("").C(collectionName)
	indexCreateErr := m.ensureIndexes(analyticsCollection)

	if indexCreateErr != nil {
		m.log.Error(indexCreateErr)
	}

	query := bson.M{
		"orgid":     filteredData.OrgID,
		"timestamp": filteredData.TimeStamp,
	}

	if len(m.dbConf.IgnoreAggregationsList) > 0 {
		filteredData.DiscardAggregations(m.dbConf.IgnoreAggregationsList)
	}

	updateDoc := filteredData.AsChange()

	change := mgo.Change{
		Update:    updateDoc,
		ReturnNew: true,
		Upsert:    true,
	}

	doc := analytics.AnalyticsRecordAggregate{}
	_, err := analyticsCollection.Find(query).Apply(change, &doc)
	if err != nil {
		m.log.WithField("query", query).Error("UPSERT Failure: ", err)
		return m.HandleWriteErr(err)
	}

	// We have the new doc back, lets fix the averages
	avgUpdateDoc := doc.AsTimeUpdate()
	avgChange := mgo.Change{
		Update:    avgUpdateDoc,
		ReturnNew: true,
	}
	withTimeUpdate := analytics.AnalyticsRecordAggregate{}
	_, avgErr := analyticsCollection.Find(query).Apply(avgChange, &withTimeUpdate)

	if m.dbConf.ThresholdLenTagList != -1 && (len(withTimeUpdate.Tags) > m.dbConf.ThresholdLenTagList) {
		m.printAlert(withTimeUpdate, m.dbConf.ThresholdLenTagList)
	}

	if avgErr != nil {
		m.log.WithField("query", query).Error("AvgUpdate Failure: ", avgErr)
		return m.HandleWriteErr(avgErr)
	}

	if m.dbConf.UseMixedCollection {
		thisData := analytics.AnalyticsRecordAggregate{}
		err := analyticsCollection.Find(query).One(&thisData)
		if err != nil {
			m.log.WithField("query", query).Error("Couldn't find query doc:", err)
		} else {
			m.doMixedWrite(thisData, query)
		}
	}
	return nil
}

func (m *MongoGraphAggregatePump) GetCollectionName(orgid string) (string, error) {
	if orgid == "" {
		return "", errors.New("OrgID cannot be empty")
	}

	return "z_tyk_graph_analyticz_aggregate_" + orgid, nil
}
