package mongo

import (
	"strings"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps/internal/mgo"
	"gopkg.in/vmihailenco/msgpack.v2"
)

// WriteUptimeData will pull the data from the in-memory store and drop it into the specified MongoDB collection
func (p *Pump) WriteUptimeData(data []interface{}) {
	if len(data) == 0 {
		return
	}

	collectionName := "tyk_uptime_analytics"
	sess := p.dbSession.Copy()
	defer sess.Close()

	analyticsCollection := sess.DB("").C(collectionName)

	p.Log.Debug("Uptime Data: ", len(data))

	var keys []interface{}
	for _, v := range data {
		decoded := analytics.UptimeReportData{}

		stringValue, ok := v.(string)
		if !ok {
			continue
		}

		if err := msgpack.Unmarshal([]byte(stringValue), &decoded); err != nil {
			p.Log.Error("Couldn't unmarshal analytics data:", err)
			continue
		}
		keys = append(keys, interface{}(decoded))

		p.Log.Debug("Decoded Record: ", decoded)
	}

	if len(keys) == 0 {
		return
	}

	p.Log.Debug("Writing data to ", collectionName)

	if err := analyticsCollection.Insert(keys...); err != nil {

		p.Log.Error("Problem inserting to mongo collection: ", err)

		if strings.Contains(err.Error(), "Closed explicitly") || strings.Contains(err.Error(), "EOF") {
			p.Log.Warning("--> Detected connection failure, reconnecting")

			p.connect(mgo.NewDialer())
		}
	}
}
