package pumps

import (
	"context"
	"encoding/json"
	"fmt"
	"log/syslog"

	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type SyslogPump struct {
	syslogConf *SyslogConf
	writer     *syslog.Writer
	filters    analytics.AnalyticsFilters
	timeout    int
	CommonPumpConfig
}

var (
	syslogPrefix     = "syslog-pump"
	syslogDefaultENV = PUMPS_ENV_PREFIX + "_SYSLOG" + PUMPS_ENV_META_PREFIX
)

// @PumpConf Syslog
type SyslogConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_SYSLOG_META`
	EnvPrefix string `json:"meta_env_prefix" mapstructure:"meta_env_prefix"`
	// Possible values are `udp, tcp, tls` in string form.
	Transport string `json:"transport" mapstructure:"transport"`
	// Host & Port combination of your syslog daemon ie: `"localhost:5140"`.
	NetworkAddr string `json:"network_addr" mapstructure:"network_addr"`
	// The severity level, an integer from 0-7, based off the Standard:
	// [Syslog Severity Levels](https://en.wikipedia.org/wiki/Syslog#Severity_level).
	LogLevel int `json:"log_level" mapstructure:"log_level"`
	// Prefix tag
	//
	// When working with FluentD, you should provide a
	// [FluentD Parser](https://docs.fluentd.org/input/syslog) based on the OS you are using so
	// that FluentD can correctly read the logs.
	//
	// ```{.json}
	// "syslog": {
	//   "name": "syslog",
	//   "meta": {
	//     "transport": "udp",
	//     "network_addr": "localhost:5140",
	//     "log_level": 6,
	//     "tag": "syslog-pump"
	//   }
	// ```
	Tag string `json:"tag" mapstructure:"tag"`
	// When set to true, prevents log fragmentation by serializing messages to JSON.
	// When false, uses the legacy format. Defaults to false for backward compatibility.
	SyslogFragmentation bool `json:"syslog_fragmentation" mapstructure:"syslog_fragmentation"`
}

func (s *SyslogPump) GetName() string {
	return "Syslog Pump"
}

func (s *SyslogPump) New() Pump {
	newPump := SyslogPump{}
	return &newPump
}

func (s *SyslogPump) GetEnvPrefix() string {
	return s.syslogConf.EnvPrefix
}

func (s *SyslogPump) Init(config interface{}) error {
	//Read configuration file
	s.syslogConf = &SyslogConf{}
	s.log = log.WithField("prefix", syslogPrefix)

	err := mapstructure.Decode(config, &s.syslogConf)
	if err != nil {
		s.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(s, s.log, s.syslogConf, syslogDefaultENV)
	// Init the configs
	s.initConfigs()

	// Init the Syslog writer
	s.initWriter()

	s.log.Info(s.GetName() + " Initialized")

	return nil
}

func (s *SyslogPump) initWriter() {
	tag := syslogPrefix
	if s.syslogConf.Tag != "" {
		tag = s.syslogConf.Tag
	}
	syslogWriter, err := syslog.Dial(
		s.syslogConf.Transport,
		s.syslogConf.NetworkAddr,
		syslog.Priority(s.syslogConf.LogLevel),
		tag)

	if err != nil {
		s.log.Fatal("failed to connect to Syslog Daemon: ", err)
	}

	s.writer = syslogWriter
}

// Set default values if they are not explicitly given
// And perform validation
func (s *SyslogPump) initConfigs() {
	if s.syslogConf.Transport == "" {
		s.syslogConf.Transport = "udp"
		s.log.Info("No Transport given, using 'udp'")
	}

	if s.syslogConf.Transport != "udp" &&
		s.syslogConf.Transport != "tcp" &&
		s.syslogConf.Transport != "tls" {
		s.log.Fatal("Chosen invalid Transport type.  Please use a supported Transport type for Syslog")
	}

	if s.syslogConf.NetworkAddr == "" {
		s.syslogConf.NetworkAddr = "localhost:5140"
		s.log.Info("No host given, using 'localhost:5140'")
	}

	if s.syslogConf.LogLevel == 0 {
		s.log.Warn("Using Log Level 0 (KERNEL) for Syslog pump")
	}
}

/**
** Write the actual Data to Syslog Here
 */
func (s *SyslogPump) WriteData(ctx context.Context, data []interface{}) error {
	s.log.Debug("Attempting to write ", len(data), " records...")

	//Data is all the analytics being written
	for _, v := range data {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Decode the raw analytics into Form
			decoded := v.(analytics.AnalyticsRecord)

			if s.syslogConf.SyslogFragmentation {
				// New behavior: Serialize to JSON to prevent log fragmentation
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

				// Serialize to JSON to prevent log fragmentation
				jsonData, err := json.Marshal(message)
				if err != nil {
					s.log.Error("Failed to marshal message to JSON: ", err)
					continue
				}

				// Print to Syslog as single-line JSON
				_, _ = fmt.Fprintf(s.writer, "%s", string(jsonData))
			} else {
				// Legacy behavior: Use the old format for backward compatibility
				_, _ = fmt.Fprintf(s.writer, "timestamp=%s method=%s path=%s raw_path=%s response_code=%d alias=%s api_key=%s api_version=%s api_name=%s api_id=%s org_id=%s oauth_id=%s raw_request=%s request_time_ms=%d raw_response=%s ip_address=%s host=%s content_length=%d user_agent=%s",
					decoded.TimeStamp,
					decoded.Method,
					decoded.Path,
					decoded.RawPath,
					decoded.ResponseCode,
					decoded.Alias,
					decoded.APIKey,
					decoded.APIVersion,
					decoded.APIName,
					decoded.APIID,
					decoded.OrgID,
					decoded.OauthID,
					decoded.RawRequest,
					decoded.RequestTime,
					decoded.RawResponse,
					decoded.IPAddress,
					decoded.Host,
					decoded.ContentLength,
					decoded.UserAgent)
			}
		}
	}
	s.log.Info("Purged ", len(data), " records...")

	return nil
}

func (s *SyslogPump) SetTimeout(timeout int) {
	s.timeout = timeout
}

func (s *SyslogPump) GetTimeout() int {
	return s.timeout
}

func (s *SyslogPump) SetFilters(filters analytics.AnalyticsFilters) {
	s.filters = filters
}
func (s *SyslogPump) GetFilters() analytics.AnalyticsFilters {
	return s.filters
}
