package pumps

import (
	"context"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/mitchellh/mapstructure"
)

var stdOutPrefix = "stdout-pump"

type StdOutPump struct {
	CommonPumpConfig
	conf *StdOutConf
}

type StdOutConf struct {
	Format       string `mapstructure:"format"`
	LogFieldName string `mapstructure:"log_field_name"`
}

func (s *StdOutPump) GetName() string {
	return "Stdout Pump"
}

func (s *StdOutPump) New() Pump {
	newPump := StdOutPump{}
	return &newPump
}

func (s *StdOutPump) Init(config interface{}) error {

	s.conf = &StdOutConf{}
	err := mapstructure.Decode(config, &s.conf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": statsdPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	log = logger.GetLogger()

	if s.conf.Format == "json" {
		log.Formatter = &logrus.JSONFormatter{}
	}

	if s.conf.LogFieldName == "" {
		s.conf.LogFieldName = "tyk-analytics-record"
	}

	return nil

}

/**
** Write the actual Data to Stdout Here
 */
func (s *StdOutPump) WriteData(ctx context.Context, data []interface{}) error {

	//Data is all the analytics being written
	for _, v := range data {

		select {
		case <-ctx.Done():
			return nil
		default:
			// Decode the raw analytics into Form
			decoded := v.(analytics.AnalyticsRecord)
			message := Json{
				"timestamp":       decoded.TimeStamp,
				"method":          decoded.Method,
				"path":            decoded.Path,
				"raw_path":        decoded.RawPath,
				"response_code":   decoded.ResponseCode,
				"alias":           decoded.Alias,
				"api_key":         decoded.APIKey,
				"api_version":     decoded.APIVersion,
				"api_name":        decoded.APIName,
				"api_id":          decoded.APIID,
				"org_id":          decoded.OrgID,
				"oauth_id":        decoded.OauthID,
				"raw_request":     decoded.RawRequest,
				"request_time_ms": decoded.RequestTime,
				"raw_response":    decoded.RawResponse,
				"ip_address":      decoded.IPAddress,
				"host":            decoded.Host,
				"content_length":  decoded.ContentLength,
				"user_agent":      decoded.UserAgent,
			}

			log.WithFields(logrus.Fields{
				s.conf.LogFieldName: message,
			}).Info()
		}
	}

	return nil
}
