package pumps

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/quipo/statsd"
	"github.com/sirupsen/logrus"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type StatsdPump struct {
	dbConf *StatsdConf
	CommonPumpConfig
}

var statsdPrefix = "statsd-pump"
var statsdDefaultENV = PUMPS_ENV_PREFIX + "_STATSD" + PUMPS_ENV_META_PREFIX

// @PumpConf Statsd
type StatsdConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_STATSD_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// Address of statsd including host & port.
	Address string `json:"address" mapstructure:"address"`
	// Define which Analytics fields should have its own metric calculation.
	Fields []string `json:"fields" mapstructure:"fields"`
	// List of tags to be added to the metric.
	Tags []string `json:"tags" mapstructure:"tags"`
	// Allows to have a separated method field instead of having it embedded in the path field.
	SeparatedMethod bool `json:"separated_method" mapstructure:"separated_method"`
}

func (s *StatsdPump) New() Pump {
	newPump := StatsdPump{}
	return &newPump
}

func (s *StatsdPump) GetName() string {
	return "Statsd Pump"
}

func (s *StatsdPump) GetEnvPrefix() string {
	return s.dbConf.EnvPrefix
}

func (s *StatsdPump) Init(config interface{}) error {
	s.dbConf = &StatsdConf{}
	s.log = log.WithField("prefix", statsdPrefix)

	err := mapstructure.Decode(config, &s.dbConf)
	if err != nil {
		s.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(s, s.log, s.dbConf, statsdDefaultENV)

	s.connect()

	s.log.Debug("StatsD CS: ", s.dbConf.Address)
	s.log.Info(s.GetName() + " Initialized")

	return nil
}

func (s *StatsdPump) connect() *statsd.StatsdClient {

	client := statsd.NewStatsdClient(s.dbConf.Address, "")

	for {
		s.log.Debug("connecting to statsD...")

		if err := client.CreateSocket(); err != nil {
			s.log.Error("statsD connection failed retrying in 5 seconds:", err)
			time.Sleep(5 * time.Second)

			continue
		}

		s.log.Debug("statsD connection successful...")

		return client
	}
}

// isTimingField checks if a field should be sent as a timing metric
func (s *StatsdPump) isTimingField(field string) bool {
	timingFields := []string{"request_time", "latency_total", "latency_upstream", "latency_gateway"}
	for _, f := range timingFields {
		if field == f {
			return true
		}
	}
	return false
}

// sendTimingMetric sends a timing metric to StatsD with proper error handling
func (s *StatsdPump) sendTimingMetric(client *statsd.StatsdClient, field, metricTags string, value int64) {
	metric := field + "." + metricTags
	if err := client.Timing(metric, value); err != nil {
		s.log.WithFields(logrus.Fields{
			"field":  field,
			"metric": metric,
			"value":  value,
		}).Error("failed to send timing metric to StatsD:", err)
	}
}

func (s *StatsdPump) WriteData(ctx context.Context, data []interface{}) error {

	if len(data) == 0 {
		return nil
	}
	s.log.Debug("Attempting to write ", len(data), " records...")

	client := s.connect()
	defer client.Close()

	for _, v := range data {
		// Convert to AnalyticsRecord
		decoded := v.(analytics.AnalyticsRecord)

		mapping := s.getMappings(decoded)

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

		// Send timing metrics for each configured field
		for _, f := range s.dbConf.Fields {
			if s.isTimingField(f) {
				s.sendTimingMetric(client, f, metricTags, mapping[f].(int64))
			}
		}
	}
	s.log.Info("Purged ", len(data), " records...")

	return nil
}

func (s *StatsdPump) getMappings(decoded analytics.AnalyticsRecord) map[string]interface{} {
	// Format TimeStamp to Unix Time
	unixTime := time.Unix(decoded.TimeStamp.Unix(), 0)

	// Replace : to -
	sanitizedTime := strings.Replace(unixTime.String(), ":", "-", -1)

	// Remove the last splash after path
	decoded.Path = strings.TrimRight(decoded.Path, "/")

	mapping := map[string]interface{}{
		"path":             decoded.Method + decoded.Path,
		"response_code":    decoded.ResponseCode,
		"api_key":          decoded.APIKey,
		"time_stamp":       sanitizedTime,
		"api_version":      decoded.APIVersion,
		"api_name":         decoded.APIName,
		"api_id":           decoded.APIID,
		"org_id":           decoded.OrgID,
		"oauth_id":         decoded.OauthID,
		"raw_request":      decoded.RawRequest,
		"request_time":     decoded.RequestTime,
		"latency_total":    decoded.Latency.Total,
		"latency_upstream": decoded.Latency.Upstream,
		"latency_gateway":  decoded.Latency.Gateway,
		"raw_response":     decoded.RawResponse,
		"ip_address":       decoded.IPAddress,
	}
	if s.dbConf.SeparatedMethod {
		mapping["path"] = decoded.Path
		mapping["method"] = decoded.Method
	}

	return mapping
}
