package pumps

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

const (
	defaultPath      = "/services/collector/event/1.0"
	authHeaderName   = "authorization"
	authHeaderPrefix = "Splunk "
	splunkPumpPrefix = "splunk-pump"
	splunkPumpName   = "Splunk Pump"
	splunkDefaultENV = PUMPS_ENV_PREFIX + "_SPLUNK" + PUMPS_ENV_META_PREFIX
)

var (
	errInvalidSettings = errors.New("Empty settings")
	//By default in spunk ~ 800 MB. https://docs.splunk.com/Documentation/Splunk/latest/Admin/Limitsconf#.5Bhttp_input.5D
	maxContentLength = 838860800
)

// SplunkClient contains Splunk client methods.
type SplunkClient struct {
	Token         string
	CollectorURL  string
	TLSSkipVerify bool

	httpClient *http.Client
}

// SplunkPump is a Tyk Pump driver for Splunk.
type SplunkPump struct {
	client *SplunkClient
	config *SplunkPumpConfig
	CommonPumpConfig
}

// SplunkPumpConfig contains the driver configuration parameters.
type SplunkPumpConfig struct {
	EnvPrefix              string   `mapstructure:"meta_env_prefix"`
	CollectorToken         string   `mapstructure:"collector_token"`
	CollectorURL           string   `mapstructure:"collector_url"`
	SSLInsecureSkipVerify  bool     `mapstructure:"ssl_insecure_skip_verify"`
	SSLCertFile            string   `mapstructure:"ssl_cert_file"`
	SSLKeyFile             string   `mapstructure:"ssl_key_file"`
	SSLServerName          string   `mapstructure:"ssl_server_name"`
	ObfuscateAPIKeys       bool     `mapstructure:"obfuscate_api_keys"`
	ObfuscateAPIKeysLength int      `mapstructure:"obfuscate_api_keys_length"`
	Fields                 []string `mapstructure:"fields"`
	IgnoreTagPrefixList    []string `mapstructure:"ignore_tag_prefix_list"`
	EnableBatch            bool     `mapstructure:"enable_batch"`
	BatchMaxContentLength  int      `mapstructure:"batch_max_content_length"`
}

// New initializes a new pump.
func (p *SplunkPump) New() Pump {
	return &SplunkPump{}
}

// GetName returns the pump name.
func (p *SplunkPump) GetName() string {
	return splunkPumpName
}

func (p *SplunkPump) GetEnvPrefix() string {
	return p.config.EnvPrefix
}

// Init performs the initialization of the SplunkClient.
func (p *SplunkPump) Init(config interface{}) error {
	p.config = &SplunkPumpConfig{}
	p.log = log.WithField("prefix", splunkPumpPrefix)

	err := mapstructure.Decode(config, p.config)
	if err != nil {
		return err
	}

	processPumpEnvVars(p, p.log, p.config, splunkDefaultENV)

	p.log.Infof("%s Endpoint: %s", splunkPumpName, p.config.CollectorURL)

	p.client, err = NewSplunkClient(p.config.CollectorToken, p.config.CollectorURL, p.config.SSLInsecureSkipVerify, p.config.SSLCertFile, p.config.SSLKeyFile, p.config.SSLServerName)
	if err != nil {
		return err
	}

	if p.config.EnableBatch && p.config.BatchMaxContentLength == 0 {
		p.config.BatchMaxContentLength = maxContentLength
	}

	p.log.Info(p.GetName() + " Initialized")

	return nil
}

// Filters the tags based on config rule
func (p *SplunkPump) FilterTags(filteredTags []string) []string {
	// Loop all explicitly ignored tags
	for _, excludeTag := range p.config.IgnoreTagPrefixList {
		// Loop the current analytics item tags
		for key, currentTag := range filteredTags {
			// If the current tag's value includes an ignored word, remove it from the list
			if strings.HasPrefix(currentTag, excludeTag) {
				copy(filteredTags[key:], filteredTags[key+1:])
				filteredTags[len(filteredTags)-1] = ""
				filteredTags = filteredTags[:len(filteredTags)-1]
			}
		}
	}

	return filteredTags
}

// WriteData prepares an appropriate data structure and sends it to the HTTP Event Collector.
func (p *SplunkPump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Debug("Attempting to write ", len(data), " records...")

	var batchBuffer bytes.Buffer

	fnSendBytes := func(data []byte) error {
		_, errSend := p.client.Send(ctx, data)
		if errSend != nil {
			p.log.Error("Error writing data to Spunk ", errSend)
			return errSend
		}
		return nil
	}

	for _, v := range data {
		decoded := v.(analytics.AnalyticsRecord)
		apiKey := decoded.APIKey

		// Check if the APIKey obfuscation is configured and its doable
		if p.config.ObfuscateAPIKeys && len(apiKey) > p.config.ObfuscateAPIKeysLength {
			// Obfuscate the APIKey, starting with 4 asterics and followed by last N chars (configured separately) of the APIKey
			// The default value of the length is 0 so unless another number is configured, the APIKey will be fully hidden
			apiKey = "****" + apiKey[len(apiKey)-p.config.ObfuscateAPIKeysLength:]
		}

		mapping := map[string]interface{}{
			"method":         decoded.Method,
			"host":           decoded.Host,
			"path":           decoded.Path,
			"raw_path":       decoded.RawPath,
			"content_length": decoded.ContentLength,
			"user_agent":     decoded.UserAgent,
			"response_code":  decoded.ResponseCode,
			"api_key":        apiKey,
			"time_stamp":     decoded.TimeStamp,
			"api_version":    decoded.APIVersion,
			"api_name":       decoded.APIName,
			"api_id":         decoded.APIID,
			"org_id":         decoded.OrgID,
			"oauth_id":       decoded.OauthID,
			"raw_request":    decoded.RawRequest,
			"request_time":   decoded.RequestTime,
			"raw_response":   decoded.RawResponse,
			"ip_address":     decoded.IPAddress,
			"geo":            decoded.Geo,
			"network":        decoded.Network,
			"latency":        decoded.Latency,
			"tags":           decoded.Tags,
			"alias":          decoded.Alias,
			"track_path":     decoded.TrackPath,
		}

		// Define an empty event
		event := make(map[string]interface{})

		// Populate the Splunk event with the fields set in the config
		if len(p.config.Fields) > 0 {
			// Loop through all fields set in the pump config
			for _, field := range p.config.Fields {
				// Skip the next actions in case the configured field doesn't exist
				if _, ok := mapping[field]; !ok {
					continue
				}

				// Check if the current analytics field is "tags" and see if some tags are explicitly excluded
				if field == "tags" && len(p.config.IgnoreTagPrefixList) > 0 {
					// Reassign the tags after successful filtration
					mapping["tags"] = p.FilterTags(mapping["tags"].([]string))
				}

				// Adding field value
				event[field] = mapping[field]
			}
		} else {
			// Set the default event fields
			event = map[string]interface{}{
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
		}
		eventWrap := struct {
			Time  int64                  `json:"time"`
			Event map[string]interface{} `json:"event"`
		}{Time: decoded.TimeStamp.Unix(), Event: event}

		data, err := json.Marshal(eventWrap)
		if err != nil {
			return err
		}

		if p.config.EnableBatch {
			//if we're batching and the len of our data is already bigger than max_content_length, we send the data and reset the buffer
			if batchBuffer.Len()+len(data) > p.config.BatchMaxContentLength {
				if err := fnSendBytes(batchBuffer.Bytes()); err != nil {
					return err
				}
				batchBuffer.Reset()
			}
			batchBuffer.Write(data)
		} else {
			if err := fnSendBytes(data); err != nil {
				return err
			}
		}
	}

	//this if is for data remaining in the buffer when len(buffer) is lower than max_content_length
	if p.config.EnableBatch && batchBuffer.Len() > 0 {
		if err := fnSendBytes(batchBuffer.Bytes()); err != nil {
			return err
		}
		batchBuffer.Reset()
	}

	p.log.Info("Purged ", len(data), " records...")

	return nil
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
		if certFile == "" && keyFile == "" {
			return c, errors.New("ssl_insecure_skip_verify set to false but no ssl_cert_file or ssl_key_file specified")
		}
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
func (c *SplunkClient) Send(ctx context.Context, data []byte) (*http.Response, error) {

	reader := bytes.NewReader(data)
	req, err := http.NewRequest("POST", c.CollectorURL, reader)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add(authHeaderName, authHeaderPrefix+c.Token)
	return c.httpClient.Do(req)
}
