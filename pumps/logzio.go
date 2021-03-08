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

	defaultLogzioCheckDiskSpace = true
	defaultLogzioDiskThreshold  = 98 // represent % of the disk
	defaultLogzioDrainDuration  = "3s"
	defaultLogzioURL            = "https://listener.logz.io:8071"

	minDiskThreshold = 0
	maxDiskThreshold = 100
)

type LogzioPumpConfig struct {
	CheckDiskSpace bool   `mapstructure:"check_disk_space"`
	DiskThreshold  int    `mapstructure:"disk_threshold"`
	DrainDuration  string `mapstructure:"drain_duration"`
	QueueDir       string `mapstructure:"queue_dir"`
	Token          string `mapstructure:"token"`
	URL            string `mapstructure:"url"`
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

func (p *LogzioPump) Init(config interface{}) error {
	p.config = NewLogzioPumpConfig()
	p.log = log.WithField("prefix", LogzioPumpPrefix)

	err := mapstructure.Decode(config, p.config)
	if err != nil {
		p.log.Fatalf("Failed to decode configuration: %s", err)
	}

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
