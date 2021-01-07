package pumps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/TykTechnologies/tyk-pump/analyticspb"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/TykTechnologies/logrus"
)

const (
	defaultDogstatsdNamespace              = "default"
	defaultDogstatsdSampleRate             = 1
	defaultDogstatsdBufferedMaxMessages    = 16
	defaultDogstatsdUDSWriteTimeoutSeconds = 1
)

type DogStatsdPump struct {
	conf   *DogStatsdConf
	client *statsd.Client
	log    *logrus.Entry
	CommonPumpConfig
}

type DogStatsdConf struct {
	Namespace            string  `mapstructure:"namespace"`
	Address              string  `mapstructure:"address"`
	SampleRate           float64 `mapstructure:"sample_rate"`
	AsyncUDS             bool    `mapstructure:"async_uds"`
	AsyncUDSWriteTimeout int     `mapstructure:"async_uds_write_timeout_seconds"`
	Buffered             bool    `mapstructure:"buffered"`
	BufferedMaxMessages  int     `mapstructure:"buffered_max_messages"`
}

func (s *DogStatsdPump) New() Pump {
	newPump := DogStatsdPump{}
	return &newPump
}

func (s *DogStatsdPump) GetName() string {
	return "DogStatsd Pump"
}

func (s *DogStatsdPump) Init(conf interface{}) error {
	s.log = log.WithField("prefix", "dogstatsd")

	s.log.Info("initializing pump")
	if err := mapstructure.Decode(conf, &s.conf); err != nil {
		return errors.Wrap(err, "unable to decode dogstatsd configuration")
	}

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
		opts = append(opts, statsd.Buffered())
		opts = append(opts, statsd.WithMaxMessagesPerPayload(s.conf.BufferedMaxMessages))
	}

	if s.conf.AsyncUDS {
		opts = append(opts, statsd.WithAsyncUDS())
		opts = append(opts, statsd.WithWriteTimeoutUDS(time.Duration(s.conf.AsyncUDSWriteTimeout)*time.Second))
	}

	if err := s.connect(opts); err != nil {
		return errors.Wrap(err, "unable to connect to dogstatsd client")
	}

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

	s.log.Info(fmt.Sprintf("purging %d records", len(data)))
	for _, v := range data {
		// Convert to AnalyticsRecord
		decoded := v.(analyticspb.AnalyticsRecord)
		decoded.Path = strings.TrimRight(decoded.Path, "/")

		/*
		 * From DataDog website:
		 * Tags shouldn’t originate from unbounded sources, such as EPOCH timestamps, user IDs, or request IDs. Doing
		 * so may infinitely increase the number of metrics for your organization and impact your billing.
		 *
		 * As such, we have significantly limited the available metrics which gets sent to datadog.
		 */
		tags := []string{
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

		if err := s.client.Histogram("request_time", float64(decoded.RequestTime), tags, s.conf.SampleRate); err != nil {
			s.log.WithError(err).Error("unable to record Histogram, dropping analytics record")
		}
	}

	return nil
}
