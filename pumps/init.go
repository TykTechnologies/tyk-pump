package pumps

import "github.com/TykTechnologies/tyk-pump/logger"

var log = logger.GetLogger()
var AvailablePumps map[string]Pump

func init() {
	AvailablePumps = make(map[string]Pump)

	// Register all the storage handlers here
	AvailablePumps["dummy"] = &DummyPump{}
	AvailablePumps["mongo"] = &MongoPump{}
	AvailablePumps["mongo-pump-selective"] = &MongoSelectivePump{}
	AvailablePumps["csv"] = &CSVPump{}
	AvailablePumps["elasticsearch"] = &ElasticsearchPump{}
	AvailablePumps["influx"] = &InfluxPump{}
	AvailablePumps["statsd"] = &StatsdPump{}
	AvailablePumps["segment"] = &SegmentPump{}
	AvailablePumps["graylog"] = &GraylogPump{}
}
