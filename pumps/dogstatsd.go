package pumps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

const (
	defaultDogstatsdNamespace              = "default"
	defaultDogstatsdSampleRate             = 1
	defaultDogstatsdBufferedMaxMessages    = 16
	defaultDogstatsdUDSWriteTimeoutSeconds = 1
)

var dogstatPrefix = "dogstatsd"
var dogstatDefaultENV = PUMPS_ENV_PREFIX + "_DOGSTATSD" + PUMPS_ENV_META_PREFIX

type DogStatsdPump struct {
	conf   *DogStatsdConf
	client *statsd.Client
	CommonPumpConfig
}

type DogStatsdConf struct {
	EnvPrefix            string   `mapstructure:"meta_env_prefix"`
	Namespace            string   `mapstructure:"namespace"`
	Address              string   `mapstructure:"address"`
	SampleRate           float64  `mapstructure:"sample_rate"`
	AsyncUDS             bool     `mapstructure:"async_uds"`
	AsyncUDSWriteTimeout int      `mapstructure:"async_uds_write_timeout_seconds"`
	Buffered             bool     `mapstructure:"buffered"`
	BufferedMaxMessages  int      `mapstructure:"buffered_max_messages"`
	Tags                 []string `mapstructure:"tags"`
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
				default:
					return fmt.Errorf("undefined tag '%s'", tag)
				}
				tags = append(tags, value)
			}
		}

		if err := s.client.Histogram("request_time", float64(decoded.RequestTime), tags, s.conf.SampleRate); err != nil {
			s.log.WithError(err).Error("unable to record Histogram, dropping analytics record")
		}
	}
	s.log.Info("Purged ", len(data), " records...")

	return nil
}
