package pumps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
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
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_STDOUT_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// Format of the analytics logs. Default is `text` if `json` is not explicitly specified. When
	// JSON logging is used all pump logs to stdout will be JSON.
	Format string `json:"format" mapstructure:"format"`
	// Root name of the JSON object the analytics record is nested in.
	LogFieldName string `json:"log_field_name" mapstructure:"log_field_name"`
	// CleanJSON formats raw_request and raw_response as valid JSON objects instead of escaped strings.
	CleanJSON bool `json:"clean_json" mapstructure:"clean_json"`
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

				if s.conf.CleanJSON {
					decoded.RawRequest = transformHTTPPayload(decoded.RawRequest)
					decoded.RawResponse = transformHTTPPayload(decoded.RawResponse)
				}

				entry := log.WithField(s.conf.LogFieldName, decoded)
				entry.Level = logrus.InfoLevel
				entry.Time = time.Now().UTC()
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

// transformHTTPPayload separates HTTP headers from the body using the standard
// HTTP separator (\r\n\r). It removes unnecessary whitespaces from the headers
// and compacts the JSON body if it is valid.
func transformHTTPPayload(raw string) string {
	if raw == "" {
		return raw
	}

	sep := "\r\n\r\n"
	parts := strings.SplitN(raw, sep, 2)

	if len(parts) == 2 {
		headers := parts[0]
		bodyBytes := []byte(parts[1])

		if json.Valid(bodyBytes) {
			var compacted bytes.Buffer
			if err := json.Compact(&compacted, bodyBytes); err == nil {
				return fmt.Sprintf("%s %s", removeWhitespaces(headers), compacted.String())
			}
		}
	}

	return removeWhitespaces(raw)
}

// removeWhitespaces removes carriage returns ('\r') and tabs ('\t'),
// and replaces newlines with a single space to create a single-line output.
func removeWhitespaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			b.WriteString(" ")
		case '\r', '\t':
			// skip
		default:
			b.WriteByte(s[i])
		}
	}

	return b.String()
}
