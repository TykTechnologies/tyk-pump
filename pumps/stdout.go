package pumps

import (
	"context"
	"fmt"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
)

var (
	stdOutPrefix     = "stdout-pump"
	stdOutDefaultENV = PUMPS_ENV_PREFIX + "_STDOUT" + PUMPS_ENV_META_PREFIX
)

type StdOutPump struct {
	CommonPumpConfig
	conf *StdOutConf
}

// @PumpConf StdOut
type StdOutConf struct {
	EnvPrefix    string `json:"meta_env_prefix" mapstructure:"meta_env_prefix"`
	Format       string `json:"format" mapstructure:"format"`
	LogFieldName string `json:"log_field_name" mapstructure:"log_field_name"`
}

func (s *StdOutPump) GetName() string {
	return "Stdout Pump"
}

func (s *StdOutPump) GetEnvPrefix() string {
	return s.conf.EnvPrefix
}

func (s *StdOutPump) New() Pump {
	newPump := StdOutPump{}
	return &newPump
}

func (s *StdOutPump) Init(config interface{}) error {

	s.log = log.WithField("prefix", stdOutPrefix)

	s.conf = &StdOutConf{}
	err := mapstructure.Decode(config, &s.conf)

	if err != nil {
		s.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(s, s.log, s.conf, stdOutDefaultENV)

	if s.conf.LogFieldName == "" {
		s.conf.LogFieldName = "tyk-analytics-record"
	}

	s.log.Info(s.GetName() + " Initialized")

	return nil

}

/**
** Write the actual Data to Stdout Here
 */
func (s *StdOutPump) WriteData(ctx context.Context, data []interface{}) error {
	s.log.Debug("Attempting to write ", len(data), " records...")

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
				s.log.WithField(s.conf.LogFieldName, decoded).Info()
			}

		}
	}
	s.log.Info("Purged ", len(data), " records...")

	return nil
}
