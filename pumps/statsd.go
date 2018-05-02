package pumps

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/quipo/statsd"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type StatsdPump struct {
	dbConf *StatsdConf
}

var statsdPrefix = "statsd-pump"

type StatsdConf struct {
	Address string   `mapstructure:"address"`
	Fields  []string `mapstructure:"fields"`
	Tags    []string `mapstructure:"tags"`
}

func (s *StatsdPump) New() Pump {
	newPump := StatsdPump{}
	return &newPump
}

func (s *StatsdPump) GetName() string {
	return "Statsd Pump"
}

func (s *StatsdPump) Init(config interface{}) error {
	s.dbConf = &StatsdConf{}
	err := mapstructure.Decode(config, &s.dbConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": statsdPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	s.connect()

	log.WithFields(logrus.Fields{
		"prefix": statsdPrefix,
	}).Debug("StatsD CS: ", s.dbConf.Address)

	return nil
}

func (s *StatsdPump) connect() *statsd.StatsdClient {

	client := statsd.NewStatsdClient(s.dbConf.Address, "")

	for {
		log.WithField("prefix", statsdPrefix).Debug("connecting to statsD...")

		if err := client.CreateSocket(); err != nil {
			log.WithField("prefix", statsdPrefix).Error("statsD connection failed retrying in 5 seconds:", err)
			time.Sleep(5 * time.Second)

			continue
		}

		log.WithField("prefix", statsdPrefix).Debug("statsD connection successful...")

		return client
	}
}

func (s *StatsdPump) WriteData(data []interface{}) error {

	var client *statsd.StatsdClient

	if len(data) > 0 {
		client = s.connect()
		defer client.Close()
	}

	for _, v := range data {
		// Convert to AnalyticsRecord
		decoded := v.(analytics.AnalyticsRecord)

		// Format TimeStamp to Unix Time
		unixTime := time.Unix(decoded.TimeStamp.Unix(), 0)

		// Replace : to -
		sanitizedTime := strings.Replace(unixTime.String(), ":", "-", -1)

		// Remove the last splash after path
		decoded.Path = strings.TrimRight(decoded.Path, "/")

		mapping := map[string]interface{}{
			"path":          decoded.Method + decoded.Path,
			"response_code": decoded.ResponseCode,
			"api_key":       decoded.APIKey,
			"time_stamp":    sanitizedTime,
			"api_version":   decoded.APIVersion,
			"api_name":      decoded.APIName,
			"api_id":        decoded.APIID,
			"org_id":        decoded.OrgID,
			"oauth_id":      decoded.OauthID,
			"raw_request":   decoded.RawRequest,
			"request_time":  decoded.RequestTime,
			"raw_response":  decoded.RawResponse,
			"ip_address":    decoded.IPAddress,
		}

		// Combine tags
		var metricTags string
		for i, t := range s.dbConf.Tags {
			var tag string
			b, err := json.Marshal(mapping[t])
			if err != nil {
				tag = ""
			} else {
				tag = string(b)
				// Lowercase
				tag = strings.ToLower(tag)
			}

			if i != len(s.dbConf.Tags)-1 {
				metricTags += tag + "."
			} else {
				metricTags += tag
			}
		}

		// Sanitize quotes and empty string
		metricTags = strings.Replace(metricTags, "\"", "", -1)
		metricTags = strings.Replace(metricTags, " ", "", -1)

		// For each field, create metric caculation
		// Everybody has their own implementation here
		for _, f := range s.dbConf.Fields {
			if f == "request_time" {
				metric := f + "." + metricTags
				client.Timing(metric, mapping[f].(int64))
			}
		}
	}
	return nil
}
