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

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/retry"

	"github.com/mitchellh/mapstructure"
)

// https://docs.dynatrace.com/docs/analyze-explore-automate/logs/lma-log-ingestion/lma-log-ingestion-via-api
// https://docs.dynatrace.com/docs/discover-dynatrace/references/dynatrace-api/environment-api/log-monitoring-v2/post-ingest-logs

const (
	dynatraceDefaultPath      = "/api/v2/logs/ingest"
	dynatraceAuthHeaderName   = "Authorization"
	dynatraceAuthHeaderPrefix = "Api-Token "
	dynatracePumpPrefix       = "dynatrace-pump"
	dynatracePumpName         = "Dynatrace Pump"
	dynatraceDefaultEnv       = PUMPS_ENV_PREFIX + "_DYNATRACE" + PUMPS_ENV_META_PREFIX
	dynatraceMaxContentLength = 10485760 // 10 MB - https://docs.dynatrace.com/docs/analyze-explore-automate/logs/lma-limits
)

var (
	dynatraceErrInvalidSettings = errors.New("Empty settings")
)

// DynatraceClient contains Dynatrace client methods.
type DynatraceClient struct {
	Token         string
	EndpointUrl   string
	TLSSkipVerify bool
	httpClient    *http.Client
	retry         *retry.BackoffHTTPRetry
}

// DynatracePump is a Tyk Pump driver for Dynatrace.
type DynatracePump struct {
	client *DynatraceClient
	config *DynatracePumpConfig
	CommonPumpConfig
}

// DynatracePumpConfig contains the driver configuration parameters.
// @PumpConf Dynatrace
type DynatracePumpConfig struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_DYNATRACE_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// API Token - must have 'Ingest logs' scope.
	ApiToken string `json:"api_token" mapstructure:"api_token"`
	// Endpoint the Pump will send analytics too. Should look something like:
	// `https://{your-environment-id}.live.dynatrace.com` or `https://{your-activegate-domain}:9999/e/{your-environment-id}`.
	EndpointUrl string `json:"endpoint_url" mapstructure:"endpoint_url"`
	// Controls whether the pump client verifies the Dynatrace server's certificate chain and host name.
	SSLInsecureSkipVerify bool `json:"ssl_insecure_skip_verify" mapstructure:"ssl_insecure_skip_verify"`
	// SSL cert file location.
	SSLCertFile string `json:"ssl_cert_file" mapstructure:"ssl_cert_file"`
	// SSL cert key location.
	SSLKeyFile string `json:"ssl_key_file" mapstructure:"ssl_key_file"`
	// SSL Server name used in the TLS connection.
	SSLServerName string `json:"ssl_server_name" mapstructure:"ssl_server_name"`
	// Controls whether the pump client should hide the API key. In case you still need substring
	// of the value, check the next option. Default value is `false`.
	ObfuscateAPIKeys bool `json:"obfuscate_api_keys" mapstructure:"obfuscate_api_keys"`
	// Define the number of the characters from the end of the API key. The `obfuscate_api_keys`
	// should be set to `true`. Default value is `0`.
	ObfuscateAPIKeysLength int `json:"obfuscate_api_keys_length" mapstructure:"obfuscate_api_keys_length"`
	// Define which Analytics fields should participate in the Dynatrace event. Check the available
	// fields in the example below. Default value is `["http.method", "http.url", "http.status_code",
	// "http.client_ip", "api_key", "api_version", "api_name", "api_id", "org_id", "oauth_id",
	// "raw_request", "request_time", "raw_response"]`.
	Fields []string `json:"fields" mapstructure:"fields"`
	// Configures a list of additional key/value pairs to attach to events.
	// When configuring it via environment variable, the expected value
	// is a comma separated list of key-value pairs delimited with a colon.
	// Example: `TYK_PMP_PUMPS_DYNATRACE_META_PROPERTIES=key1:value1,key2:/value2`
	// Produces: `{"key1": "value1", "key2": "/value2"}`
	Properties map[string]string `json:"properties" mapstructure:"properties"`
	// Choose which tags to be ignored by the Dynatrace Pump. Keep in mind that the tag name and value
	// are hyphenated. Default value is `[]`.
	IgnoreTagPrefixList []string `json:"ignore_tag_prefix_list" mapstructure:"ignore_tag_prefix_list"`
	// If this is set to `true`, pump is going to send the analytics records in batch to Dynatrace.
	// Default value is `false`.
	EnableBatch bool `json:"enable_batch" mapstructure:"enable_batch"`
	// Max content length in bytes to be sent in batch requests. If the purged analytics records size don't reach
	// the amount of bytes, they're sent anyways during each purge loop. Default value is 10485760
	// (10 MB), the Dynatrace API limit.
	BatchMaxContentLength int `json:"batch_max_content_length" mapstructure:"batch_max_content_length"`
	// MaxRetries represents the maximum amount of retries to attempt if failed to send requests to Dynatrace API.
	// Default value is `0`
	MaxRetries uint64 `json:"max_retries" mapstructure:"max_retries"`
}

// New initializes a new pump.
func (p *DynatracePump) New() Pump {
	return &DynatracePump{}
}

// GetName returns the pump name.
func (p *DynatracePump) GetName() string {
	return dynatracePumpName
}

func (p *DynatracePump) GetEnvPrefix() string {
	return p.config.EnvPrefix
}

// Init performs the initialization of the DynatraceClient.
func (p *DynatracePump) Init(config interface{}) error {
	p.config = &DynatracePumpConfig{}
	p.log = log.WithField("prefix", dynatracePumpPrefix)

	err := mapstructure.Decode(config, p.config)
	if err != nil {
		return err
	}

	processPumpEnvVars(p, p.log, p.config, dynatraceDefaultEnv)

	p.log.Infof("%s Endpoint: %s", dynatracePumpName, p.config.EndpointUrl)

	p.client, err = NewDynatraceClient(p.config.ApiToken, p.config.EndpointUrl, p.config.SSLInsecureSkipVerify, p.config.SSLCertFile, p.config.SSLKeyFile, p.config.SSLServerName)
	if err != nil {
		return err
	}

	if p.config.EnableBatch && p.config.BatchMaxContentLength == 0 {
		p.config.BatchMaxContentLength = dynatraceMaxContentLength
	}

	if p.config.MaxRetries > 0 {
		p.log.Infof("%d max retries", p.config.MaxRetries)
	}

	p.client.retry = retry.NewBackoffRetry("Failed writing data to Dynatrace", p.config.MaxRetries, p.client.httpClient, p.log)
	p.log.Info(p.GetName() + " Initialized")

	return nil
}

// Filters the tags based on config rule
func (p *DynatracePump) FilterTags(filteredTags []string) []string {
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

// WriteData prepares an appropriate data structure and sends it to the API endpoint.
func (p *DynatracePump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Debug("Attempting to write ", len(data), " records...")

	var batchBuffer bytes.Buffer
	var currentBatchCount int

	// Start JSON array for batch mode
	if p.config.EnableBatch {
		batchBuffer.WriteByte('[')
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
			"http.method":      decoded.Method,
			"http.host":        decoded.Host,
			"http.url":         decoded.RawPath,
			"http.status_code": decoded.ResponseCode,
			"http.client_ip":   decoded.IPAddress,
			"api_key":          apiKey,
			"geo.city_name":    decoded.Geo.City.Names,
			"geo.country_name": decoded.Geo.Country.ISOCode,
			"geo.name":         decoded.Geo.Country.ISOCode,
			"geo.region_name":  decoded.Geo.City.GeoNameID,
			"content_length":   decoded.ContentLength,
			"user_agent":       decoded.UserAgent,
			"api_version":      decoded.APIVersion,
			"api_name":         decoded.APIName,
			"api_id":           decoded.APIID,
			"org_id":           decoded.OrgID,
			"oauth_id":         decoded.OauthID,
			"raw_request":      decoded.RawRequest,
			"request_time":     decoded.RequestTime,
			"raw_response":     decoded.RawResponse,
			"network":          decoded.Network,
			"latency":          decoded.Latency,
			"tags":             decoded.Tags,
			"alias":            decoded.Alias,
			"track_path":       decoded.TrackPath,
		}

		// Define an empty event
		event := make(map[string]interface{})

		// Populate the Dynatrace event with the fields set in the config
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
				"http.method":      decoded.Method,
				"http.url":         decoded.RawPath,
				"http.status_code": decoded.ResponseCode,
				"http.client_ip":   decoded.IPAddress,
				"api_key":          apiKey,
				"api_version":      decoded.APIVersion,
				"api_name":         decoded.APIName,
				"api_id":           decoded.APIID,
				"org_id":           decoded.OrgID,
				"oauth_id":         decoded.OauthID,
				"raw_request":      decoded.RawRequest,
				"request_time":     decoded.RequestTime,
				"raw_response":     decoded.RawResponse,
			}
		}

		// Populate custom properties
		if len(p.config.Properties) > 0 {
			// Loop through all fields set in the pump config
			for key, value := range p.config.Properties {
				event[key] = value
			}
		}

		event["timestamp"] = decoded.TimeStamp.UnixMilli()

		eventData, err := json.Marshal(event)
		if err != nil {
			return err
		}

		// Check if event will cause max content length to be exceeded
		maxContentLength := dynatraceMaxContentLength
		if p.config.EnableBatch {
			maxContentLength = p.config.BatchMaxContentLength
		}
		eventPayloadSize := len(eventData)
		if p.config.EnableBatch {
			eventPayloadSize += 2 // for JSON array brackets (if single event in batch) or comma separator and closing bracket (if multiple events in batch)
		}
		if eventPayloadSize > maxContentLength {
			p.log.Warnf("Event with timestamp '%s' too large (%d bytes), skipping", decoded.TimeStamp, len(eventData))
			continue
		}

		if p.config.EnableBatch {
			// If adding this event would exceed max content length, send current batch first
			if batchBuffer.Len()+eventPayloadSize > maxContentLength {
				// Close the current array and send
				batchBuffer.WriteByte(']')
				p.log.Debugf("Mid run - sending %d batch records...", currentBatchCount+1)
				if err := p.send(ctx, batchBuffer.Bytes()); err != nil {
					return err
				}
				// Reset for next batch
				batchBuffer.Reset()
				batchBuffer.WriteByte('[')
				currentBatchCount = 0
			}

			// Add comma separator if not the first event in this batch
			if currentBatchCount > 0 {
				batchBuffer.WriteByte(',')
			}
			batchBuffer.Write(eventData)
			currentBatchCount++
		} else {
			// For non-batch mode, API accepts single JSON object (does not need to be in array)
			if err := p.send(ctx, eventData); err != nil {
				return err
			}
		}
	}

	// Send remaining data in batch buffer
	if p.config.EnableBatch && currentBatchCount > 0 {
		batchBuffer.WriteByte(']')
		p.log.Debugf("End of run - sending %d batch records...", currentBatchCount)
		if err := p.send(ctx, batchBuffer.Bytes()); err != nil {
			return err
		}
	}

	p.log.Info("Purged ", len(data), " records...")

	return nil
}

// NewDynatraceClient initializes a new DynatraceClient.
func NewDynatraceClient(token string, endpointUrl string, skipVerify bool, certFile string, keyFile string, serverName string) (c *DynatraceClient, err error) {
	if token == "" || endpointUrl == "" {
		return c, dynatraceErrInvalidSettings
	}
	u, err := url.Parse(endpointUrl)
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
	http.DefaultClient.Transport = &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}
	u.Path = dynatraceDefaultPath // Append the default endpoint API path
	c = &DynatraceClient{
		Token:       token,
		EndpointUrl: u.String(),
		httpClient:  http.DefaultClient,
	}
	return c, nil
}

func (p *DynatracePump) send(ctx context.Context, data []byte) error {
	reader := bytes.NewReader(data)
	req, err := http.NewRequest(http.MethodPost, p.client.EndpointUrl, reader)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Add(dynatraceAuthHeaderName, dynatraceAuthHeaderPrefix+p.client.Token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	p.log.Debugf("Sending %d bytes to dynatrace", len(data))
	return p.client.retry.Send(req)
}
