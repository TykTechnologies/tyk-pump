package pumps

import (
	"encoding/base64"
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/robertkowalski/graylog-golang"
)

type GraylogPump struct {
	client *gelf.Gelf
	conf   *GraylogConf
}

type GraylogConf struct {
	GraylogHost string   `mapstructure:"host"`
	GraylogPort int      `mapstructure:"port"`
	Tags        []string `mapstructure: "tags"`
}

var graylogPrefix string = "graylog-pump"

func (p *GraylogPump) New() Pump {
	newPump := GraylogPump{}
	return &newPump
}

func (p *GraylogPump) GetName() string {
	return "Graylog Pump"
}

func (p *GraylogPump) Init(conf interface{}) error {
	p.conf = &GraylogConf{}
	err := mapstructure.Decode(conf, &p.conf)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": graylogPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	if p.conf.GraylogHost == "" {
		p.conf.GraylogHost = "localhost"
	}

	if p.conf.GraylogPort == 0 {
		p.conf.GraylogPort = 1000
	}
	log.WithFields(logrus.Fields{
		"prefix": graylogPrefix,
	}).Info("GraylogHost:", p.conf.GraylogHost)
	log.WithFields(logrus.Fields{
		"prefix": graylogPrefix,
	}).Info("GraylogPort:", p.conf.GraylogPort)

	p.connect()
	return nil
}

func (p *GraylogPump) connect() {
	p.client = gelf.New(gelf.Config{
		GraylogPort:     p.conf.GraylogPort,
		GraylogHostname: p.conf.GraylogHost,
	})
}

func (p *GraylogPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": graylogPrefix,
	}).Info("Writing ", len(data), " records")

	if p.client == nil {
		p.connect()
		p.WriteData(data)
	}

	for _, item := range data {
		record := item.(analytics.AnalyticsRecord)

		rReq, err := base64.StdEncoding.DecodeString(record.RawRequest)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": graylogPrefix,
			}).Fatal(err)
		}

		rResp, err := base64.StdEncoding.DecodeString(record.RawRequest)

		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": graylogPrefix,
			}).Fatal(err)
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
			log.WithFields(logrus.Fields{
				"prefix": graylogPrefix,
			}).Fatal(err)
		}

		gelfData := map[string]interface{}{
			//"version": "1.1",
			"host":      "tyk-pumps",
			"timestamp": record.TimeStamp.Unix(),
			"message":   string(message),
		}

		gelfString, err := json.Marshal(gelfData)

		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": graylogPrefix,
			}).Fatal(err)
		}

		log.WithFields(logrus.Fields{
			"prefix": graylogPrefix,
		}).Info("Writing ", string(message))

		p.client.Log(string(gelfString))
	}
	return nil
}
