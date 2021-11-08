package pumps

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/mitchellh/mapstructure"
	gelf "github.com/robertkowalski/graylog-golang"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type GraylogPump struct {
	client *gelf.Gelf
	conf   *GraylogConf
	CommonPumpConfig
}

// @PumpConf Graylog
type GraylogConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// Graylog host.
	GraylogHost string `json:"host" mapstructure:"host"`
	// Graylog port.
	GraylogPort int `json:"port" mapstructure:"port"`
	// List of tags to be added to the metric. The possible options are listed in the below example.
	//
	// If no tag is specified the fallback behaviour is to don't send anything.
	// The possible values are:
	// - `path`
	// - `method`
	// - `response_code`
	// - `api_version`
	// - `api_name`
	// - `api_id`
	// - `org_id`
	// - `tracked`
	// - `oauth_id`
	// - `raw_request`
	// - `raw_response`
	// - `request_time`
	// - `ip_address`
	Tags []string `json:"tags" mapstructure:"tags"`
}

var graylogPrefix = "graylog-pump"
var graylogDefaultENV = PUMPS_ENV_PREFIX + "_GRAYLOG" + PUMPS_ENV_META_PREFIX

func (p *GraylogPump) New() Pump {
	newPump := GraylogPump{}
	return &newPump
}

func (p *GraylogPump) GetName() string {
	return "Graylog Pump"
}

func (p *GraylogPump) GetEnvPrefix() string {
	return p.conf.EnvPrefix
}

func (p *GraylogPump) Init(conf interface{}) error {
	p.conf = &GraylogConf{}

	p.log = log.WithField("prefix", graylogPrefix)

	err := mapstructure.Decode(conf, &p.conf)
	if err != nil {
		p.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(p, p.log, p.conf, graylogDefaultENV)

	if p.conf.GraylogHost == "" {
		p.conf.GraylogHost = "localhost"
	}

	if p.conf.GraylogPort == 0 {
		p.conf.GraylogPort = 1000
	}
	p.log.Info("GraylogHost:", p.conf.GraylogHost)
	p.log.Info("GraylogPort:", p.conf.GraylogPort)

	p.connect()

	p.log.Info(p.GetName() + " Initialized")

	return nil
}

func (p *GraylogPump) connect() {
	p.client = gelf.New(gelf.Config{
		GraylogPort:     p.conf.GraylogPort,
		GraylogHostname: p.conf.GraylogHost,
	})
}

func (p *GraylogPump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Debug("Attempting to write ", len(data), " records...")

	if p.client == nil {
		p.connect()
		p.WriteData(ctx, data)
	}

	for _, item := range data {
		record := item.(analytics.AnalyticsRecord)

		rReq, err := base64.StdEncoding.DecodeString(record.RawRequest)
		if err != nil {
			p.log.Fatal(err)
		}

		rResp, err := base64.StdEncoding.DecodeString(record.RawResponse)

		if err != nil {
			p.log.Fatal(err)
		}

		mapping := map[string]interface{}{
			"method":        record.Method,
			"path":          record.Path,
			"response_code": record.ResponseCode,
			"api_key":       record.APIKey,
			"api_version":   record.APIVersion,
			"api_name":      record.APIName,
			"api_id":        record.APIID,
			"org_id":        record.OrgID,
			"oauth_id":      record.OauthID,
			"raw_request":   string(rReq),
			"request_time":  record.RequestTime,
			"ip_address":    record.IPAddress,
			"raw_response":  string(rResp),
		}

		messageMap := map[string]interface{}{}

		for _, key := range p.conf.Tags {
			if value, ok := mapping[key]; ok {
				messageMap[key] = value
			}
		}

		message, err := json.Marshal(messageMap)
		if err != nil {
			p.log.Fatal(err)
		}

		gelfData := map[string]interface{}{
			//"version": "1.1",
			"host":      "tyk-pumps",
			"timestamp": record.TimeStamp.Unix(),
			"message":   string(message),
		}

		gelfString, err := json.Marshal(gelfData)

		if err != nil {
			p.log.Fatal(err)
		}

		p.log.Debug("Writing ", string(message))

		p.client.Log(string(gelfString))
	}
	p.log.Info("Purged ", len(data), " records...")

	return nil
}
