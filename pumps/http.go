package pumps

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/retry"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/net/http/httpproxy"
)

const (
	httpPumpPrefix = "http-pump"
	httpPumpName   = "HTTP Pump"
	httpDefaultENV = PUMPS_ENV_PREFIX + "_HTTP" + PUMPS_ENV_META_PREFIX

	httpDefaultBatchSize    = 500
	httpDefaultMaxBodyBytes = 10 * 1024 * 1024
	// a hung endpoint must not block the purge of every other pump when the base timeout isn't set
	httpDefaultRequestTimeout = 30
	httpDefaultMaxRetries     = 3
)

// HTTPPump sends analytics records to an HTTP endpoint as a JSON array of full records.
type HTTPPump struct {
	conf       *HTTPPumpConf
	endpoint   string
	httpClient *http.Client
	retry      *retry.BackoffHTTPRetry
	CommonPumpConfig
}

// HTTPPumpConf contains the HTTP pump configuration.
// @PumpConf HTTP
type HTTPPumpConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_HTTP_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// Endpoint the records are POSTed to, for example `https://collector.example.com/ingest`.
	URL string `json:"url" mapstructure:"url"`
	// Extra headers added to every request, for example `{"Authorization": "Bearer <token>"}`.
	// When set through the environment the format is `Name1:value1,Name2:value2`, so a value
	// set that way cannot contain `,` or `:`.
	Headers map[string]string `json:"headers" mapstructure:"headers"`
	// Path to the PEM file with trusted CA certificates used to verify the server's certificate.
	SSLCAFile string `json:"ssl_ca_file" mapstructure:"ssl_ca_file"`
	// Path to the PEM file with the client certificate, for mutual TLS.
	SSLCertFile string `json:"ssl_cert_file" mapstructure:"ssl_cert_file"`
	// Path to the PEM file with the client private key, for mutual TLS.
	SSLKeyFile string `json:"ssl_key_file" mapstructure:"ssl_key_file"`
	// SSL server name used in the TLS connection.
	SSLServerName string `json:"ssl_server_name" mapstructure:"ssl_server_name"`
	// Maximum number of records sent in one request. Default value is `500`.
	BatchSize int `json:"batch_size" mapstructure:"batch_size"`
	// Maximum size in bytes of the uncompressed JSON body of one request. A batch is split
	// before it grows past this size, and a single record larger than it is dropped.
	// Default value is `10485760` (10 MiB).
	MaxBodyBytes int `json:"max_body_bytes" mapstructure:"max_body_bytes"`
	// Maximum number of retries for a request that fails with a network error, a 5xx or a 429.
	// Any other 4xx is permanent and the batch is dropped. Set it to `0` to disable retries.
	// Default value is `3`.
	MaxRetries *uint64 `json:"max_retries" mapstructure:"max_retries"`
	// Timeout in seconds of a single request attempt, including reading the response.
	// It applies even when the base `timeout` option is not set. Default value is `30`.
	RequestTimeout int `json:"request_timeout" mapstructure:"request_timeout"`
	// Compress request bodies with gzip and send `Content-Encoding: gzip`. Default value is `false`.
	Gzip bool `json:"gzip" mapstructure:"gzip"`
	// Only send records of MCP traffic (records whose `mcp_stats.is_mcp` is true). Default value is `false`.
	MCPOnly bool `json:"mcp_only" mapstructure:"mcp_only"`
	// Controls whether the pump client verifies the server's certificate chain and host name.
	SSLInsecureSkipVerify bool `json:"ssl_insecure_skip_verify" mapstructure:"ssl_insecure_skip_verify"`
}

func (p *HTTPPump) New() Pump {
	return &HTTPPump{}
}

func (p *HTTPPump) GetName() string {
	return httpPumpName
}

func (p *HTTPPump) GetEnvPrefix() string {
	return p.conf.EnvPrefix
}

func (p *HTTPPump) Init(config interface{}) error {
	p.conf = &HTTPPumpConf{}
	p.log = log.WithField("prefix", httpPumpPrefix)

	if err := mapstructure.Decode(config, p.conf); err != nil {
		return err
	}

	processPumpEnvVars(p, p.log, p.conf, httpDefaultENV)

	if p.conf.URL == "" {
		return errors.New("http pump: url is required")
	}
	u, err := url.Parse(p.conf.URL)
	if err != nil {
		return fmt.Errorf("http pump: invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("http pump: url scheme must be http or https, got %q", u.Scheme)
	}
	p.endpoint = u.String()
	// credentials belong in headers; a query string or password in the url is not logged
	logEndpoint := retry.RedactURL(u)

	if p.conf.BatchSize <= 0 {
		p.conf.BatchSize = httpDefaultBatchSize
	}
	if p.conf.MaxBodyBytes <= 0 {
		p.conf.MaxBodyBytes = httpDefaultMaxBodyBytes
	}
	if p.conf.MaxRetries == nil {
		maxRetries := uint64(httpDefaultMaxRetries)
		p.conf.MaxRetries = &maxRetries
	}
	if p.conf.RequestTimeout <= 0 {
		p.conf.RequestTimeout = httpDefaultRequestTimeout
	}

	tlsConfig, err := NewTLSConfig(TLSConfig{
		CertFile:           p.conf.SSLCertFile,
		KeyFile:            p.conf.SSLKeyFile,
		CAFile:             p.conf.SSLCAFile,
		InsecureSkipVerify: p.conf.SSLInsecureSkipVerify,
		ServerName:         p.conf.SSLServerName,
	}, p.log)
	if err != nil {
		return fmt.Errorf("http pump: failed to configure TLS: %w", err)
	}

	// Each instance gets its own transport so several http pumps with different TLS
	// settings can run side by side. http.DefaultClient is never touched.
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return errors.New("http pump: unexpected default transport type")
	}
	transport = transport.Clone()
	transport.TLSClientConfig = tlsConfig
	// HTTP_PROXY, HTTPS_PROXY and NO_PROXY are read when the pump starts.
	// http.ProxyFromEnvironment would read them once per process and cache
	// them for every other client in it.
	proxy := httpproxy.FromEnvironment().ProxyFunc()
	transport.Proxy = func(r *http.Request) (*url.URL, error) { return proxy(r.URL) }
	p.httpClient = &http.Client{
		Transport: transport,
		Timeout:   time.Duration(p.conf.RequestTimeout) * time.Second,
		// Redirects are not followed: a 307/308 would replay the records and every
		// configured header to another location, possibly over plain http.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	p.retry = retry.NewBackoffRetry("Failed writing data to "+httpPumpName, *p.conf.MaxRetries, p.httpClient, p.log).AcceptAny2xx()

	p.log.Infof("%s Endpoint: %s", httpPumpName, logEndpoint)
	p.log.Info(p.GetName() + " Initialized")

	return nil
}

// WriteData sends the records in batches of at most batch_size records and max_body_bytes bytes.
// A failed batch is dropped and the remaining batches are still sent. The returned error
// summarises the failures, so it is logged once by the caller and not once per batch.
func (p *HTTPPump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Debug("Attempting to write ", len(data), " records...")

	var (
		body          bytes.Buffer
		count         int
		sent          int
		batches       int
		failedBatches int
		failedRecords int
		firstErr      error
		dropped       int
		unsent        int
		errs          []error
	)

	flush := func() {
		if count == 0 {
			return
		}
		body.WriteByte(']')
		batches++
		if err := p.send(ctx, body.Bytes()); err != nil {
			p.log.WithError(err).Debugf("Failed to send %d records, dropping them", count)
			failedBatches++
			failedRecords += count
			if firstErr == nil {
				firstErr = err
			}
		} else {
			sent += count
		}
		body.Reset()
		count = 0
	}

	for i, v := range data {
		if ctx.Err() != nil {
			// the pending batch is not sent on a context that is already done
			unsent = count + len(data) - i
			count = 0
			errs = append(errs, fmt.Errorf("%d records not sent: %w", unsent, ctx.Err()))
			break
		}

		record, ok := v.(analytics.AnalyticsRecord)
		if !ok {
			p.log.Errorf("Unable to decode record: %v", v)
			continue
		}
		if p.conf.MCPOnly && !record.IsMCPRecord() {
			continue
		}

		// the full record is sent by design, like the sqs pump; api_key is hashed when the gateway has hash_keys on
		encoded, err := json.Marshal(record) // #nosec G117
		if err != nil {
			p.log.WithError(err).Error("Unable to marshal record")
			continue
		}
		// two bytes for the enclosing brackets
		if len(encoded)+2 > p.conf.MaxBodyBytes {
			dropped++
			p.log.Errorf("Record of %d bytes exceeds max_body_bytes (%d), dropping it", len(encoded), p.conf.MaxBodyBytes)
			continue
		}
		// one byte for the separator and one for the closing bracket
		if count > 0 && (count >= p.conf.BatchSize || body.Len()+len(encoded)+2 > p.conf.MaxBodyBytes) {
			flush()
		}

		if count == 0 {
			body.WriteByte('[')
		} else {
			body.WriteByte(',')
		}
		body.Write(encoded)
		count++
	}
	flush()

	if failedBatches > 0 {
		errs = append(errs, fmt.Errorf("failed to send %d of %d batches (%d records), first error: %w",
			failedBatches, batches, failedRecords, firstErr))
	}
	if dropped > 0 {
		errs = append(errs, fmt.Errorf("dropped %d records larger than max_body_bytes", dropped))
	}

	p.log.Info("Purged ", sent, " records...")

	return errors.Join(errs...)
}

func (p *HTTPPump) send(ctx context.Context, payload []byte) error {
	if p.conf.Gzip {
		var compressed bytes.Buffer
		zw := gzip.NewWriter(&compressed)
		if _, err := zw.Write(payload); err != nil {
			return err
		}
		if err := zw.Close(); err != nil {
			return err
		}
		payload = compressed.Bytes()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for name, value := range p.conf.Headers {
		if strings.EqualFold(name, "Host") {
			req.Host = value
			continue
		}
		req.Header.Set(name, value)
	}
	if p.conf.Gzip {
		req.Header.Set("Content-Encoding", "gzip")
	} else {
		req.Header.Del("Content-Encoding")
	}

	p.log.Debugf("Sending %d bytes to %s", len(payload), httpPumpName)
	return p.retry.Send(req)
}
