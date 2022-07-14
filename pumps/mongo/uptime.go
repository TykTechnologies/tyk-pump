package mongo

import (
	"strings"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"gopkg.in/vmihailenco/msgpack.v2"
)

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (p *Pump) WriteUptimeData(data []interface{}) {

	for p.dbSession == nil {
		p.Log.Debug("Connecting to mongoDB store")
		p.connect()
	}

	collectionName := "tyk_uptime_analytics"
	sess := p.dbSession.Copy()
	defer sess.Close()

	analyticsCollection := sess.DB("").C(collectionName)

	p.Log.Debug("Uptime Data: ", len(data))

	if len(data) == 0 {
		return
	}

	keys := make([]interface{}, len(data))

	for i, v := range data {
		decoded := analytics.UptimeReportData{}

		if err := msgpack.Unmarshal([]byte(v.(string)), &decoded); err != nil {
			// ToDo: should this work with serializer?
			p.Log.Error("Couldn't unmarshal analytics data:", err)
			continue
		}

		keys[i] = interface{}(decoded)

		p.Log.Debug("Decoded Record: ", decoded)
	}

	p.Log.Debug("Writing data to ", collectionName)

	if err := analyticsCollection.Insert(keys...); err != nil {

		p.Log.Error("Problem inserting to mongo collection: ", err)

		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			p.Log.Warning("--> Detected connection failure, reconnecting")

			p.connect()
		}
	}
}
