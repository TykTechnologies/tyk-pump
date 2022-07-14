package pumps

import (
	"context"
	"errors"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps/mongo"
)

var log = logger.GetLogger()

type Pump interface {
	GetName() string
	Init(interface{}) error
	WriteData(context.Context, []interface{}) error
	SetFilters(analytics.AnalyticsFilters)
	GetFilters() analytics.AnalyticsFilters
	SetTimeout(timeout int)
	GetTimeout() int
	SetOmitDetailedRecording(bool)
	GetOmitDetailedRecording() bool
	GetEnvPrefix() string
	Shutdown() error
	SetMaxRecordSize(size int)
	GetMaxRecordSize() int
}

type UptimePump interface {
	GetName() string
	Init(interface{}) error
	WriteUptimeData(data []interface{})
}

func GetPumpByName(name string) (Pump, error) {

	switch name {
	case "dummy":
		return &DummyPump{}, nil
	case "mongo":
		return &mongo.MongoPump{}, nil
	case "mongo-pump-selective":
		return &MongoSelectivePump{}, nil
	case "mongo-pump-aggregate":
		return &MongoAggregatePump{}, nil
	case "csv":
		return &CSVPump{}, nil
	case "elasticsearch":
		return &ElasticsearchPump{}, nil
	case "influx":
		return &InfluxPump{}, nil
	case "influx2":
		return &Influx2Pump{}, nil
	case "moesif":
		return &MoesifPump{}, nil
	case "statsd":
		return &StatsdPump{}, nil
	case "segment":
		return &SegmentPump{}, nil
	case "graylog":
		return &GraylogPump{}, nil
	case "splunk":
		return &SplunkPump{}, nil
	case "hybrid":
		return &HybridPump{}, nil
	case "prometheus":
		return &PrometheusPump{}, nil
	case "logzio":
		return &LogzioPump{}, nil
	case "dogstatsd":
		return &DogStatsdPump{}, nil
	case "kafka":
		return &KafkaPump{}, nil
	case "syslog":
		return &SyslogPump{}, nil
	case "sql":
		return &SQLPump{}, nil
	case "sql_aggregate":
		return &SQLAggregatePump{}, nil
	case "stdout":
		return &StdOutPump{}, nil
	case "timestream":
		return &TimestreamPump{}, nil
	default:
	}

	return nil, errors.New(name + " Not found")
}
