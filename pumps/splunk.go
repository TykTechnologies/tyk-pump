package pumps

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

const (
	defaultPath      = "/services/collector/event/1.0"
	authHeaderName   = "authorization"
	authHeaderPrefix = "Splunk "
	pumpPrefix       = "splunk-pump"
	pumpName         = "Splunk Pump"
)

var (
	errInvalidSettings = errors.New("Empty settings")
)

// SplunkClient contains Splunk client methods.
type SplunkClient struct {
	Token         string
	CollectorURL  string
	TLSSkipVerify bool

	httpClient *http.Client
}

// NewSplunkClient initializes a new SplunkClient.
func NewSplunkClient(token string, collectorURL string, skipVerify bool, certFile string, keyFile string, serverName string) (c *SplunkClient, err error) {
	if token == "" || collectorURL == "" {
		return c, errInvalidSettings
	}
	u, err := url.Parse(collectorURL)
	if err != nil {
		return c, err
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: skipVerify}
	if !skipVerify {
		// Load certificates:
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return c, err
		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}, ServerName: serverName}
	}
	http.DefaultClient.Transport = &http.Transport{TLSClientConfig: tlsConfig}
	// Append the default collector API path:
	u.Path = defaultPath
	c = &SplunkClient{
		Token:        token,
		CollectorURL: u.String(),
		httpClient:   http.DefaultClient,
	}
	return c, nil
}

// Send sends an event to the Splunk HTTP Event Collector interface.
func (c *SplunkClient) Send(event map[string]interface{}, ts time.Time) (*http.Response, error) {
	eventWrap := struct {
		Time  int64                  `json:"time"`
		Event map[string]interface{} `json:"event"`
	}{Event: event}
	eventWrap.Time = ts.Unix()
	eventJSON, err := json.Marshal(eventWrap)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(eventJSON)
	req, err := http.NewRequest("POST", c.CollectorURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Add(authHeaderName, authHeaderPrefix+c.Token)
	return c.httpClient.Do(req)
}

// SplunkPump is a Tyk Pump driver for Splunk.
type SplunkPump struct {
	client *SplunkClient
	config *SplunkPumpConfig
}

// SplunkPumpConfig contains the driver configuration parameters.
type SplunkPumpConfig struct {
	CollectorToken        string `mapstructure:"collector_token"`
	CollectorURL          string `mapstructure:"collector_url"`
	SSLInsecureSkipVerify bool   `mapstructure:"ssl_insecure_skip_verify"`
	SSLCertFile           string `mapstructure:"ssl_cert_file"`
	SSLKeyFile            string `mapstructure:"ssl_key_file"`
	SSLServerName         string `mapstructure:"ssl_server_name"`
	ObfuscateAPIKeys      bool   `mapstructure:"obfuscate_api_keys"`
}

// New initializes a new pump.
func (p *SplunkPump) New() Pump {
	return &SplunkPump{}
}

// GetName returns the pump name.
func (p *SplunkPump) GetName() string {
	return pumpName
}

// Init performs the initialization of the SplunkClient.
func (p *SplunkPump) Init(config interface{}) error {
	p.config = &SplunkPumpConfig{}
	err := mapstructure.Decode(config, p.config)
	if err != nil {
		return err
	}
	log.WithFields(logrus.Fields{
		"prefix": pumpPrefix,
	}).Infof("%s Endpoint: %s", pumpName, p.config.CollectorURL)

	p.client, err = NewSplunkClient(p.config.CollectorToken, p.config.CollectorURL, p.config.SSLInsecureSkipVerify, p.config.SSLCertFile, p.config.SSLKeyFile, p.config.SSLServerName)
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{
		"prefix": pumpPrefix,
	}).Debugf("%s Initialized", pumpName)
	return nil
}

// WriteData prepares an appropriate data structure and sends it to the HTTP Event Collector.
func (p *SplunkPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": pumpPrefix,
	}).Info("Writing ", len(data), " records")
	for _, v := range data {
		decoded := v.(analytics.AnalyticsRecord)
		apiKey := decoded.APIKey
		if p.config.ObfuscateAPIKeys {
			if len(apiKey) > 4 {
				apiKey = "****" + apiKey[len(apiKey)-4:]
			}
		}

		event := map[string]interface{}{
			"method":        decoded.Method,
			"path":          decoded.Path,
			"response_code": decoded.ResponseCode,
			"api_key":       apiKey,
			"time_stamp":    decoded.TimeStamp,
			"api_version":   decoded.APIVersion,
			"api_name":      decoded.APIName,
			"api_id":        decoded.APIID,
			"org_id":        decoded.OrgID,
			"oauth_id":      decoded.OauthID,
			"raw_request":   decoded.RawRequest,
			"request_time":  decoded.RequestTime,
			"raw_response":  decoded.RawResponse,
			"ip_address":    decoded.IPAddress,
		}
		p.client.Send(event, decoded.TimeStamp)
	}
	return nil
}
