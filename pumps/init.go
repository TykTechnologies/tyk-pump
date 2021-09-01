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
	AvailablePumps["mongo-pump-aggregate"] = &MongoAggregatePump{}
	AvailablePumps["csv"] = &CSVPump{}
	AvailablePumps["elasticsearch"] = &ElasticsearchPump{}
	AvailablePumps["influx"] = &InfluxPump{}
	AvailablePumps["influx2"] = &Influx2Pump{}
	AvailablePumps["moesif"] = &MoesifPump{}
	AvailablePumps["statsd"] = &StatsdPump{}
	AvailablePumps["segment"] = &SegmentPump{}
	AvailablePumps["graylog"] = &GraylogPump{}
	AvailablePumps["splunk"] = &SplunkPump{}
	AvailablePumps["hybrid"] = &HybridPump{}
	AvailablePumps["prometheus"] = &PrometheusPump{}
	AvailablePumps["logzio"] = &LogzioPump{}
	AvailablePumps["dogstatsd"] = &DogStatsdPump{}
	AvailablePumps["kafka"] = &KafkaPump{}
	AvailablePumps["syslog"] = &SyslogPump{}
	AvailablePumps["sql"] = &SQLPump{}
	AvailablePumps["sql_aggregate"] = &SQLAggregatePump{}
	AvailablePumps["stdout"] = &StdOutPump{}
	AvailablePumps["timestream"] = &TimestreamPump{}
	AvailablePumps["mongo-graph"] = &GraphMongoPump{}
	AvailablePumps["sql-graph"] = &GraphSQLPump{}
	AvailablePumps["sql-graph-aggregate"] = &GraphSQLAggregatePump{}
	AvailablePumps["resurfaceio"] = &ResurfacePump{}
}
