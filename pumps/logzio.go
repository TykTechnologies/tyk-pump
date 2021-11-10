package pumps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	lg "github.com/logzio/logzio-go"
	"github.com/mitchellh/mapstructure"
)

const (
	LogzioPumpPrefix = "logzio-pump"
	LogzioPumpName   = "Logzio Pump"
	logzioDefaultENV = PUMPS_ENV_PREFIX + "_LOGZIO" + PUMPS_ENV_META_PREFIX

	defaultLogzioCheckDiskSpace = true
	defaultLogzioDiskThreshold  = 98 // represent % of the disk
	defaultLogzioDrainDuration  = "3s"
	defaultLogzioURL            = "https://listener.logz.io:8071"

	minDiskThreshold = 0
	maxDiskThreshold = 100
)

// @PumpConf Logzio
type LogzioPumpConfig struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// Set the sender to check if it crosses the maximum allowed disk usage. Default value is
	// `true`.
	CheckDiskSpace bool `json:"check_disk_space" mapstructure:"check_disk_space"`
	// Set disk queue threshold, once the threshold is crossed the sender will not enqueue the
	// received logs. Default value is `98` (percentage of disk).
	DiskThreshold int `json:"disk_threshold" mapstructure:"disk_threshold"`
	// Set drain duration (flush logs on disk). Default value is `3s`.
	DrainDuration string `json:"drain_duration" mapstructure:"drain_duration"`
	// The directory for the queue.
	QueueDir string `json:"queue_dir" mapstructure:"queue_dir"`
	// Token for sending data to your logzio account.
	Token string `json:"token" mapstructure:"token"`
	// If you do not want to use the default Logzio url i.e. when using a proxy. Default is
	// `https://listener.logz.io:8071`.
	URL string `json:"url" mapstructure:"url"`
}

func NewLogzioPumpConfig() *LogzioPumpConfig {
	return &LogzioPumpConfig{
		CheckDiskSpace: defaultLogzioCheckDiskSpace,
		DiskThreshold:  defaultLogzioDiskThreshold,
		DrainDuration:  defaultLogzioDrainDuration,
		QueueDir: fmt.Sprintf("%s%s%s%s%d", os.TempDir(), string(os.PathSeparator),
			"logzio-buffer", string(os.PathSeparator), time.Now().UnixNano()),
		URL: defaultLogzioURL,
	}
}

type LogzioPump struct {
	sender *lg.LogzioSender
	config *LogzioPumpConfig
	CommonPumpConfig
}

func NewLogzioClient(conf *LogzioPumpConfig) (*lg.LogzioSender, error) {
	if conf.Token == "" {
		return nil, fmt.Errorf("token is required")
	}

	drainDuration, err := time.ParseDuration(conf.DrainDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse drain_duration: %s", err)
	}

	diskThreshold := conf.DiskThreshold
	if diskThreshold < minDiskThreshold || diskThreshold > maxDiskThreshold {
		return nil, fmt.Errorf("threshold has to be between %d and %d", minDiskThreshold, maxDiskThreshold)
	}

	l, err := lg.New(
		conf.Token,
		lg.SetCheckDiskSpace(conf.CheckDiskSpace),
		lg.SetDrainDiskThreshold(conf.DiskThreshold),
		lg.SetDrainDuration(drainDuration),
		lg.SetDebug(os.Stderr),
		lg.SetTempDirectory(conf.QueueDir),
		lg.SetUrl(conf.URL),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create new logzio sender: %s", err)
	}

	return l, nil
}

func (p *LogzioPump) New() Pump {
	return &LogzioPump{}
}

func (p *LogzioPump) GetName() string {
	return LogzioPumpName
}

func (p *LogzioPump) GetEnvPrefix() string {
	return p.config.EnvPrefix
}

func (p *LogzioPump) Init(config interface{}) error {
	p.config = NewLogzioPumpConfig()
	p.log = log.WithField("prefix", LogzioPumpPrefix)

	err := mapstructure.Decode(config, p.config)
	if err != nil {
		p.log.Fatalf("Failed to decode configuration: %s", err)
	}

	processPumpEnvVars(p, p.log, p.config, logzioDefaultENV)

	p.log.Debugf("Initializing %s with the following configuration: %+v", LogzioPumpName, p.config)

	p.sender, err = NewLogzioClient(p.config)
	if err != nil {
		return err
	}
	p.log.Info(p.GetName() + " Initialized")

	return nil
}

func (p *LogzioPump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Debug("Attempting to write ", len(data), " records...")

	for _, v := range data {
		decoded := v.(analytics.AnalyticsRecord)
		mapping := map[string]interface{}{
			"@timestamp":      decoded.TimeStamp,
			"http_method":     decoded.Method,
			"request_uri":     decoded.Path,
			"response_code":   decoded.ResponseCode,
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
		}

		event, err := json.Marshal(mapping)
		if err != nil {
			return fmt.Errorf("failed to marshal decoded data: %s", err)
		}

		p.sender.Send(event)
	}
	p.log.Info("Purged ", len(data), " records...")

	return nil
}
