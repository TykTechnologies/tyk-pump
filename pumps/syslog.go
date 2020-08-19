package pumps

import (
	"context"
	"fmt"
	"log/syslog"

	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/logrus"
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
	logPrefix = "syslog-pump"
)

type SyslogConf struct {
	Transport   string `mapstructure:"transport"`
	NetworkAddr string `mapstructure:"network_addr"`
	LogLevel    int    `mapstructure:"log_level"`
	Tag         string `mapstructure:"tag"`
}

func (s *SyslogPump) GetName() string {
	return "Syslog Pump"
}

func (s *SyslogPump) New() Pump {
	newPump := SyslogPump{}
	return &newPump
}

func (s *SyslogPump) Init(config interface{}) error {
	//Read configuration file
	s.syslogConf = &SyslogConf{}
	err := mapstructure.Decode(config, &s.syslogConf)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Fatal("Failed to decode configuration: ", err)
	}

	// Init the configs
	initConfigs(s)

	// Init the Syslog writer
	initWriter(s)

	log.Debug("Syslog Pump active")
	return nil
}

func initWriter(s *SyslogPump) {
	tag := logPrefix
	if s.syslogConf.Tag != "" {
		tag = s.syslogConf.Tag
	}
	syslogWriter, err := syslog.Dial(
		s.syslogConf.Transport,
		s.syslogConf.NetworkAddr,
		syslog.Priority(s.syslogConf.LogLevel),
		tag)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Fatal("failed to connect to Syslog Daemon: ", err)
	}

	s.writer = syslogWriter
}

// Set default values if they are not explicitly given
// And perform validation
func initConfigs(pump *SyslogPump) {
	if pump.syslogConf.Transport == "" {
		pump.syslogConf.Transport = "udp"
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Info("No Transport given, using 'udp'")
	}

	if pump.syslogConf.Transport != "udp" &&
		pump.syslogConf.Transport != "tcp" &&
		pump.syslogConf.Transport != "tls" {
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Fatal("Chosen invalid Transport type.  Please use a supported Transport type for Syslog")
	}

	if pump.syslogConf.NetworkAddr == "" {
		pump.syslogConf.NetworkAddr = "localhost:5140"
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Info("No host given, using 'localhost:5140'")
	}

	if pump.syslogConf.LogLevel == 0 {
		log.WithFields(logrus.Fields{
			"prefix": logPrefix,
		}).Warn("Using Log Level 0 (KERNEL) for Syslog pump")
	}
}

/**
** Write the actual Data to Syslog Here
 */
func (s *SyslogPump) WriteData(ctx context.Context, data []interface{}) error {

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

			// Print to Syslog
			_, _ = fmt.Fprintf(s.writer, "%s", message)
		}
	}

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
