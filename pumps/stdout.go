package pumps

import (
	"context"
	"fmt"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps/common"
	"github.com/mitchellh/mapstructure"
)

var (
	stdOutPrefix     = "stdout-pump"
	stdOutDefaultENV = common.PUMPS_ENV_PREFIX + "_STDOUT" + common.PUMPS_ENV_META_PREFIX
)

type StdOutPump struct {
	common.Pump
	conf *StdOutConf
}

// @PumpConf StdOut
type StdOutConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// Format of the analytics logs. Default is `text` if `json` is not explicitly specified. When
	// JSON logging is used all pump logs to stdout will be JSON.
	Format string `json:"format" mapstructure:"format"`
	// Root name of the JSON object the analytics record is nested in.
	LogFieldName string `json:"log_field_name" mapstructure:"log_field_name"`
}

func (s *StdOutPump) GetName() string {
	return "Stdout Pump"
}

func (s *StdOutPump) GetEnvPrefix() string {
	return s.conf.EnvPrefix
}

func (s *StdOutPump) Init(config interface{}) error {

	s.Log = log.WithField("prefix", stdOutPrefix)

	s.conf = &StdOutConf{}
	err := mapstructure.Decode(config, &s.conf)

	if err != nil {
		s.Log.Fatal("Failed to decode configuration: ", err)
	}

	s.ProcessEnvVars(s.Log, s.conf, stdOutDefaultENV)

	if s.conf.LogFieldName == "" {
		s.conf.LogFieldName = "tyk-analytics-record"
	}

	s.Log.Info(s.GetName() + " Initialized")

	return nil

}

/**
** Write the actual Data to Stdout Here
 */
func (s *StdOutPump) WriteData(ctx context.Context, data []interface{}) error {
	s.Log.Debug("Attempting to write ", len(data), " records...")

	//Data is all the analytics being written
	for _, v := range data {

		select {
		case <-ctx.Done():
			return nil
		default:
			decoded := v.(analytics.AnalyticsRecord)

			if s.conf.Format == "json" {
				formatter := &logrus.JSONFormatter{}
				entry := log.WithField(s.conf.LogFieldName, decoded)
				entry.Level = logrus.InfoLevel
				data, _ := formatter.Format(entry)
				fmt.Print(string(data))
			} else {
				s.Log.WithField(s.conf.LogFieldName, decoded).Info()
			}

		}
	}
	s.Log.Info("Purged ", len(data), " records...")

	return nil
}
