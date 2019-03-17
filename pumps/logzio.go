package pumps

import (
	"encoding/json"
	"time"

	lg "github.com/logzio/logzio-go"

	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"fmt"
	"os"
)

const (
	LogzioPumpPrefix       = "logzio-pump"
	LogzioPumpName         = "Logzio Pump"

	defaultCheckDiskSpace = true
	defaultDiskThreshold  = 98.0 // represent % of the disk
	defaultDrainDuration  = "3s"
	defaultURL            = "https://listener.logz.io:8071"
)

type LogzioPumpConfig struct {
	CheckDiskSpace bool   			`mapstructure:"check_disk_space"`
	DiskThreshold  int    			`mapstructure:"disk_threshold"`
	DrainDuration  string			`mapstructure:"darin_duration"`
	QueueDir	   string 			`mapstructure:"queue_dir"`
	Token          string 			`mapstructure:"token"`
	URL            string 			`mapstructure:"url"`
}

func NewLogzioPumpConfig() *LogzioPumpConfig {
	return &LogzioPumpConfig{
		CheckDiskSpace: defaultCheckDiskSpace,
		DiskThreshold: defaultDiskThreshold,
		DrainDuration: defaultDrainDuration,
		QueueDir: fmt.Sprintf("%s%s%s%s%d", os.TempDir(), string(os.PathSeparator),
		"logzio-buffer", string(os.PathSeparator), time.Now().UnixNano()),
		URL: defaultURL,
	}
}

type LogzioPump struct {
	sender *lg.LogzioSender
	config *LogzioPumpConfig
}

func NewLogzioClient(conf *LogzioPumpConfig) (*lg.LogzioSender, error) {
	if conf.Token == "" {
		return nil, fmt.Errorf("token is required")
	}

	drainDuration, err := time.ParseDuration(conf.DrainDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse drain_duration: %s", err)
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
	err := mapstructure.Decode(config, p.config)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": LogzioPumpPrefix,
		}).Fatalf("Failed to decode configuration: ", err)
	}

	log.WithFields(logrus.Fields{
		"prefix": pumpPrefix,
	}).Infof("Initializing %s with the following configuration: %+v", pumpName, p.config)

	p.sender, err = NewLogzioClient(p.config)
	if err != nil {
		return err
	}

	return nil
}

func (p *LogzioPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": pumpPrefix,
	}).Info("Writing ", len(data), " records")

	for _, v := range data {
		decoded := v.(analytics.AnalyticsRecord)
		mapping := map[string]interface{}{
			"@timestamp":	 	decoded.TimeStamp,
			"http_method":   	decoded.Method,
			"request_uri":   	decoded.Path,
			"response_code": 	decoded.ResponseCode,
			"api_key":       	decoded.APIKey,
			"api_version":   	decoded.APIVersion,
			"api_name":      	decoded.APIName,
			"api_id":        	decoded.APIID,
			"org_id":        	decoded.OrgID,
			"oauth_id":      	decoded.OauthID,
			"raw_request":   	decoded.RawRequest,
			"request_time_ms":  decoded.RequestTime,
			"raw_response":  	decoded.RawResponse,
			"ip_address":    	decoded.IPAddress,
		}

		msg, err := json.Marshal(mapping)
		if err != nil {
			return fmt.Errorf("failed to marshal decoded data: %s", err)
		}

		p.sender.Send(msg)
	}
	return nil
}