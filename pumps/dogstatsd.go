package pumps

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

const (
	// defaultDogstatsdObfuscateAPIKeys keeps the api_key tag masked unless an operator turns it
	// off deliberately, so a credential cannot reach the metrics pipeline through oversight.
	defaultDogstatsdObfuscateAPIKeys = true
	// defaultDogstatsdObfuscateAPIKeysLength reveals enough of the key to tell two keys apart in
	// a dashboard without disclosing a usable secret, and is the same number of characters the
	// Gateway keeps when it masks a key in its own output. Masking everything would collapse every
	// key onto one tag value and make the tag pointless, which only pushes operators to disable
	// obfuscation altogether.
	defaultDogstatsdObfuscateAPIKeysLength = 4

	defaultDogstatsdNamespace              = "default"
	defaultDogstatsdSampleRate             = 1
	defaultDogstatsdBufferedMaxMessages    = 16
	defaultDogstatsdUDSWriteTimeoutSeconds = 1
	defaultDogstatsdField                  = "request_time"
)

var dogstatPrefix = "dogstatsd"
var dogstatDefaultENV = PUMPS_ENV_PREFIX + "_DOGSTATSD" + PUMPS_ENV_META_PREFIX

// DogStatsD separates tags with "," and protocol fields with "|", and "#" opens the tag list.
// None of them are escaped by the client, so a value containing one corrupts the metric line.
var dogstatsdTagValueSanitiser = strings.NewReplacer(",", "_", "|", "_", "#", "_")

// dogstatsdSupportedFields are the field names a user may configure. It must stay in step with
// dogstatsdFieldValue; a test asserts that every name here resolves.
var dogstatsdSupportedFields = []string{"request_time", "latency_total", "latency_upstream", "latency_gateway"}

// isDogstatsdFieldSupported reports whether a configured field name is one the pump can emit.
func isDogstatsdFieldSupported(field string) bool {
	for _, supported := range dogstatsdSupportedFields {
		if field == supported {
			return true
		}
	}

	return false
}

// dogstatsdFieldValue resolves a supported field name to the value it emits. The boolean guards
// the caller against a name that never passed validation.
func dogstatsdFieldValue(field string, decoded *analytics.AnalyticsRecord) (int64, bool) {
	switch field {
	case "request_time":
		return decoded.RequestTime, true
	case "latency_total":
		return decoded.Latency.Total, true
	case "latency_upstream":
		return decoded.Latency.Upstream, true
	case "latency_gateway":
		return decoded.Latency.Gateway, true
	}

	return 0, false
}

type DogStatsdPump struct {
	conf   *DogStatsdConf
	client *statsd.Client
	CommonPumpConfig
}

// @PumpConf DogStatsd
type DogStatsdConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_DOGSTATSD_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// Prefix for your metrics to datadog.
	Namespace string `json:"namespace" mapstructure:"namespace"`
	// Address of the datadog agent including host & port.
	Address string `json:"address" mapstructure:"address"`
	// Defaults to `1` which equates to `100%` of requests. To sample at `50%`, set to `0.5`.
	SampleRate float64 `json:"sample_rate" mapstructure:"sample_rate"`
	// Enable async UDS over UDP https://github.com/Datadog/datadog-go#unix-domain-sockets-client.
	AsyncUDS bool `json:"async_uds" mapstructure:"async_uds"`
	// Integer write timeout in seconds if `async_uds: true`.
	AsyncUDSWriteTimeout int `json:"async_uds_write_timeout_seconds" mapstructure:"async_uds_write_timeout_seconds"`
	// Enable buffering of messages.
	Buffered bool `json:"buffered" mapstructure:"buffered"`
	// Max messages in single datagram if `buffered: true`. Default 16.
	BufferedMaxMessages int `json:"buffered_max_messages" mapstructure:"buffered_max_messages"`
	// List of tags to be added to the metric. The possible options are listed in the below example.
	//
	// If no tag is specified the fallback behavior is to use the below tags:
	// - `path`
	// - `method`
	// - `response_code`
	// - `api_version`
	// - `api_name`
	// - `api_id`
	// - `org_id`
	// - `tracked`
	// - `oauth_id`
	//
	// The `api_key` tag is also supported, but is never part of the fallback list above — it is
	// only sent when you add it to `tags` explicitly.
	//
	// Note that this configuration can generate significant charges due to the unbound nature of
	// the `path` tag. The `api_key` tag is similarly unbounded, and additionally puts
	// credential-derived data into your metrics pipeline, so enable `obfuscate_api_keys`
	// whenever you use it.
	//
	// `fields` replaces the default rather than adding to it: setting it without `request_time`
	// stops that metric being emitted, which will break dashboards referencing it by name.
	//
	// `latency_total` carries the same value as `request_time` — the gateway populates both from
	// one timing — so listing both emits two identical timeseries. To get the upstream breakdown
	// while keeping the existing metric name, use `["request_time", "latency_upstream"]`.
	//
	// An unsupported or duplicated field name is ignored with a warning rather than stopping the
	// pump, falling back to `request_time` if nothing supported is left.
	//
	// Each metric is sampled independently, so with a `sample_rate` below 1 a given request may
	// appear in one metric and not another.
	//
	// The `api_key` tag is obfuscated by default, emitting `****` and the last four characters.
	// Turning that off emits the raw authentication token as a tag value. Because the token can be
	// large, such as a JWT, an unobfuscated tag can also push the metric past the DogStatsD
	// datagram limit, and the metric is then dropped.
	//
	// Raw request and response bodies are not available as tags. DogStatsD tag values are neither
	// escaped nor length-bounded: a body large enough to exceed the datagram size causes the whole
	// metric to be dropped, and a body containing `,`, `|` or `#` corrupts the metric line. Use a
	// logging pump such as `splunk`, `elasticsearch` or `stdout` for payload capture instead.
	//
	// ```{.json}
	// "dogstatsd": {
	//   "type": "dogstatsd",
	//   "meta": {
	//     "address": "localhost:8125",
	//     "namespace": "pump",
	//     "async_uds": true,
	//     "async_uds_write_timeout_seconds": 2,
	//     "buffered": true,
	//     "buffered_max_messages": 32,
	//     "sample_rate": 0.5,
	//     "tags": [
	//       "method",
	//       "response_code",
	//       "api_version",
	//       "api_name",
	//       "api_id",
	//       "org_id",
	//       "tracked",
	//       "path",
	//       "oauth_id",
	//       "api_key"
	//     ],
	//     "fields": [
	//       "request_time",
	//       "latency_total",
	//       "latency_upstream"
	//     ],
	//     "obfuscate_api_keys": true,
	//     "obfuscate_api_keys_length": 4
	//   }
	// },
	// ```
	//
	// On startup, you should see the loaded configs when initializing the dogstatsd pump
	// ```
	// [May 10 15:23:44]  INFO dogstatsd: initializing pump
	// [May 10 15:23:44]  INFO dogstatsd: namespace: pump.
	// [May 10 15:23:44]  INFO dogstatsd: sample_rate: 50%
	// [May 10 15:23:44]  INFO dogstatsd: buffered: true, max_messages: 32
	// [May 10 15:23:44]  INFO dogstatsd: async_uds: true, write_timeout: 2s
	// [May 10 15:23:44]  INFO dogstatsd: fields: [request_time latency_total latency_upstream], obfuscate_api_keys: true
	// ```
	Tags []string `json:"tags" mapstructure:"tags"`
	// Define which Analytics fields should be sent as their own metric. The supported values are
	// `request_time`, `latency_total`, `latency_upstream` and `latency_gateway`.
	//
	// Defaults to `["request_time"]`, so leaving this unset emits exactly one metric per record.
	Fields []string `json:"fields" mapstructure:"fields"`
	// Controls whether the pump client should hide the API key used in the `api_key` tag.
	//
	// Defaults to `true`, matching how the Gateway masks keys in its own output: `****` plus the
	// last few characters unless key logging is deliberately enabled. The Splunk and Prometheus
	// pumps default the same option to `false`, so this differs from them — but the `api_key` tag
	// is new here, so no existing configuration changes behaviour, and a credential should not
	// reach a metrics pipeline because someone did not know to opt in.
	// Setting this to `false` emits the raw authentication token as a tag value.
	ObfuscateAPIKeys bool `json:"obfuscate_api_keys" mapstructure:"obfuscate_api_keys"`
	// Define the number of characters from the end of the API key to keep when
	// `obfuscate_api_keys` is enabled. Defaults to `4`.
	//
	// Setting this to `0` masks the key completely, which also collapses every key onto the same
	// tag value and makes the tag useless as a dimension.
	ObfuscateAPIKeysLength int `json:"obfuscate_api_keys_length" mapstructure:"obfuscate_api_keys_length"`
}

func (s *DogStatsdPump) New() Pump {
	newPump := DogStatsdPump{}
	return &newPump
}

func (s *DogStatsdPump) GetName() string {
	return "DogStatsd Pump"
}

func (s *DogStatsdPump) GetEnvPrefix() string {
	return s.conf.EnvPrefix
}

func (s *DogStatsdPump) Init(conf interface{}) error {

	s.log = log.WithField("prefix", dogstatPrefix)

	// Seed the defaults that are not the zero value before decoding. mapstructure and envconfig
	// both only write the keys they are given, so anything left unset keeps the value below.
	s.conf = &DogStatsdConf{
		ObfuscateAPIKeys:       defaultDogstatsdObfuscateAPIKeys,
		ObfuscateAPIKeysLength: defaultDogstatsdObfuscateAPIKeysLength,
	}

	if err := mapstructure.Decode(conf, &s.conf); err != nil {
		return errors.Wrap(err, "unable to decode dogstatsd configuration")
	}

	processPumpEnvVars(s, s.log, s.conf, dogstatDefaultENV)

	if s.conf.Namespace == "" {
		s.conf.Namespace = defaultDogstatsdNamespace
	}
	s.conf.Namespace += "."
	s.log.Infof("namespace: %s", s.conf.Namespace)

	if s.conf.SampleRate == 0 {
		s.conf.SampleRate = defaultDogstatsdSampleRate
	}
	s.log.Infof("sample_rate: %d%%", int(s.conf.SampleRate*100))

	if s.conf.Buffered && s.conf.BufferedMaxMessages == 0 {
		s.conf.BufferedMaxMessages = defaultDogstatsdBufferedMaxMessages
	}
	s.log.Infof("buffered: %t, max_messages: %d", s.conf.Buffered, s.conf.BufferedMaxMessages)

	if s.conf.AsyncUDSWriteTimeout == 0 {
		s.conf.AsyncUDSWriteTimeout = defaultDogstatsdUDSWriteTimeoutSeconds
	}
	s.log.Infof("async_uds: %t, write_timeout: %ds", s.conf.AsyncUDS, s.conf.AsyncUDSWriteTimeout)

	s.conf.Fields = s.resolveFields()

	if s.conf.ObfuscateAPIKeysLength < 0 {
		s.log.Warn("obfuscate_api_keys_length is negative, treating it as 0")
		s.conf.ObfuscateAPIKeysLength = 0
	}
	s.log.Infof("fields: %v, obfuscate_api_keys: %t", s.conf.Fields, s.conf.ObfuscateAPIKeys)

	var opts []statsd.Option
	if s.conf.Buffered {
		opts = append(opts, statsd.WithMaxMessagesPerPayload(s.conf.BufferedMaxMessages))
	} else {
		//this option is added to simulate an unbuffered behaviour. Specified in datadog 3.0.0 lib release https://github.com/DataDog/datadog-go/blob/master/CHANGELOG.md#breaking-changes-1
		opts = append(opts, statsd.WithMaxMessagesPerPayload(1))
	}

	if s.conf.AsyncUDS {
		opts = append(opts, statsd.WithWriteTimeoutUDS(time.Duration(s.conf.AsyncUDSWriteTimeout)*time.Second))
	}

	if err := s.connect(opts); err != nil {
		return errors.Wrap(err, "unable to connect to dogstatsd client")
	}

	s.log.Info(s.GetName() + " Initialized")

	return nil
}

func (s *DogStatsdPump) connect(options []statsd.Option) error {
	c, err := statsd.New(s.conf.Address, options...)
	if err != nil {
		return errors.Wrap(err, "unable to create new dogstatsd client")
	}

	c.Namespace = s.conf.Namespace
	c.Tags = append(c.Tags, "tyk-pump")

	s.client = c

	return nil
}

// resolveFields normalises the configured field names: trimming them, dropping duplicates and
// discarding any that are not supported. An unusable name is logged and skipped rather than
// failing the pump, so one typo costs a single metric instead of every metric this pump reports.
func (s *DogStatsdPump) resolveFields() []string {
	fields := make([]string, 0, len(s.conf.Fields))
	seen := make(map[string]bool, len(s.conf.Fields))

	for _, field := range s.conf.Fields {
		// Comma-separated environment variables commonly carry a space after the separator.
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}

		if !isDogstatsdFieldSupported(field) {
			s.log.Warnf("ignoring unsupported field '%s'; supported fields are %v",
				field, dogstatsdSupportedFields)

			continue
		}

		if seen[field] {
			s.log.Warnf("field '%s' is configured more than once, ignoring the duplicate", field)

			continue
		}

		seen[field] = true
		fields = append(fields, field)
	}

	if len(fields) == 0 {
		if len(s.conf.Fields) > 0 {
			s.log.Warnf("no supported field was configured, falling back to '%s'", defaultDogstatsdField)
		}

		return []string{defaultDogstatsdField}
	}

	return fields
}

// obfuscateAPIKey masks the API key when obfuscation is enabled, keeping only the configured
// number of trailing characters. Keys no longer than that length carry no safely revealable
// portion, so they are replaced wholesale rather than emitted in the clear.
func (s *DogStatsdPump) obfuscateAPIKey(apiKey string) string {
	if !s.conf.ObfuscateAPIKeys {
		return apiKey
	}

	keep := s.conf.ObfuscateAPIKeysLength
	if keep < 0 {
		keep = 0
	}

	// Walk back the requested number of characters from the end. Counting characters rather than
	// bytes stops a multi-byte key being split mid-character, and walking avoids allocating a
	// rune slice for a token that may be several kilobytes long.
	end := len(apiKey)
	for taken := 0; taken < keep && end > 0; taken++ {
		_, size := utf8.DecodeLastRuneInString(apiKey[:end])
		end -= size
	}

	// Anything left before the kept suffix means the key is longer than the part being revealed.
	if end > 0 {
		return "****" + apiKey[end:]
	}

	return "--"
}

func (s *DogStatsdPump) WriteData(ctx context.Context, data []interface{}) error {
	if len(data) == 0 {
		return nil
	}

	s.log.Debug("Attempting to write ", len(data), " records...")
	for _, v := range data {
		// Convert to AnalyticsRecord
		decoded := v.(analytics.AnalyticsRecord)

		/*
		 * From DataDog website:
		 * Tags shouldn’t originate from unbounded sources, such as EPOCH timestamps, user IDs, or request IDs. Doing
		 * so may infinitely increase the number of metrics for your organization and impact your billing.
		 *
		 * As such, we have significantly limited the available metrics which gets sent to datadog.
		 */
		var tags []string
		if len(s.conf.Tags) == 0 {
			tags = []string{
				"path:" + decoded.Path,                                // request path
				"method:" + decoded.Method,                            // request method
				fmt.Sprintf("response_code:%d", decoded.ResponseCode), // http response code
				"api_version:" + decoded.APIVersion,
				"api_name:" + decoded.APIName,
				"api_id:" + decoded.APIID,
				"org_id:" + decoded.OrgID,
				fmt.Sprintf("tracked:%t", decoded.TrackPath),
			}
			if decoded.OauthID != "" {
				tags = append(tags, "oauth_id:"+decoded.OauthID)
			}
		} else {
			tags = make([]string, 0, len(s.conf.Tags))
			for _, tag := range s.conf.Tags {
				var value string
				switch tag {
				case "method":
					value = "method:" + decoded.Method // request method
				case "response_code":
					value = fmt.Sprintf("response_code:%d", decoded.ResponseCode) // http response code
				case "api_version":
					value = "api_version:" + decoded.APIVersion
				case "api_name":
					value = "api_name:" + decoded.APIName
				case "api_id":
					value = "api_id:" + decoded.APIID
				case "org_id":
					value = "org_id:" + decoded.OrgID
				case "tracked":
					value = fmt.Sprintf("tracked:%t", decoded.TrackPath)
				case "path":
					decoded.Path = strings.TrimRight(decoded.Path, "/")
					value = "path:" + decoded.Path // request path
				case "oauth_id":
					if decoded.OauthID == "" {
						continue
					}
					value = "oauth_id:" + decoded.OauthID
				case "api_key":
					if decoded.APIKey == "" {
						continue
					}
					value = "api_key:" + dogstatsdTagValueSanitiser.Replace(s.obfuscateAPIKey(decoded.APIKey))
				default:
					return fmt.Errorf("undefined tag '%s'", tag)
				}
				tags = append(tags, value)
			}
		}

		// One metric per configured field. The client shards metrics across independent buffers
		// by name, so the order they reach the agent does not necessarily match the order below.
		for _, field := range s.conf.Fields {
			value, ok := dogstatsdFieldValue(field, &decoded)
			if !ok {
				// Init rejects unknown names, so this is unreachable in practice. Skipping rather
				// than returning keeps one bad name from costing the whole batch, which the caller
				// discards without retrying.
				continue
			}

			if err := s.client.Histogram(field, float64(value), tags, s.conf.SampleRate); err != nil {
				s.log.WithError(err).Error("unable to record Histogram, dropping analytics record")
			}
		}
	}
	s.log.Info("Purged ", len(data), " records...")

	return nil
}

func (s *DogStatsdPump) Shutdown() error {
	if s.conf.Buffered {
		return s.client.Flush()
	}
	return nil
}
