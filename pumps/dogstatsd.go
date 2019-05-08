package pumps

import (
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

const (
	defaultDogstatsdSampleRate             = 1
	defaultDogstatsdBufferedMaxMessages    = 16
	defaultDogstatsdUDSWriteTimeoutSeconds = 1
)

type DogStatsdPump struct {
	conf   *DogStatsdConf
	client *statsd.Client
	log    *logrus.Entry
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

	if s.conf.SampleRate == 0 {
		s.conf.SampleRate = defaultDogstatsdSampleRate
	}

	if s.conf.BufferedMaxMessages == 0 {
		s.conf.BufferedMaxMessages = defaultDogstatsdBufferedMaxMessages
	}

	if s.conf.AsyncUDSWriteTimeout == 0 {
		s.conf.AsyncUDSWriteTimeout = defaultDogstatsdUDSWriteTimeoutSeconds
	}

	if err := s.connect(); err != nil {
		return errors.Wrap(err, "unable to connect to dogstatsd client")
	}

	return nil
}

func (s *DogStatsdPump) connect() error {

	var opts []statsd.Option
	if s.conf.Buffered {
		opts = append(opts, statsd.Buffered())
		opts = append(opts, statsd.WithMaxMessagesPerPayload(s.conf.BufferedMaxMessages))
	}

	if s.conf.AsyncUDS {
		opts = append(opts, statsd.WithAsyncUDS())
		opts = append(opts, statsd.WithWriteTimeoutUDS(time.Duration(s.conf.AsyncUDSWriteTimeout)*time.Second))
	}

	c, err := statsd.New(s.conf.Address, opts...)

	if err != nil {
		return errors.Wrap(err, "unable to create new dogstatsd client")
	}

	c.Namespace = s.conf.Namespace + "."

	// send tyk-pump tag with every metric
	c.Tags = append(c.Tags, "tyk-pump")

	s.client = c

	return nil
}

func (s *DogStatsdPump) disconnect() {
	_ = s.client.Close()
}

func (s *DogStatsdPump) WriteData(data []interface{}) error {
	if len(data) == 0 {
		return nil
	}

	s.log.Info(fmt.Sprintf("purging %d records", len(data)))
	for _, v := range data {
		// Convert to AnalyticsRecord
		decoded := v.(analytics.AnalyticsRecord)
		decoded.Path = strings.TrimRight(decoded.Path, "/")

		tags := []string{
			"path:" + decoded.Path,
			"method:" + decoded.Method,
			fmt.Sprintf("response_code:%d", decoded.ResponseCode),
			"api_key:" + decoded.APIKey,
			"api_version:" + decoded.APIVersion,
			"api_name:" + decoded.APIName,
			"api_id:" + decoded.APIID,
			"org_id:" + decoded.OrgID,
			"oauth_id:" + decoded.OauthID,
			"ip_address:" + decoded.IPAddress,
		}

		for _, v := range decoded.Tags {
			tags = append(tags, strings.Replace(v, "-", ":", 1))
		}

		if err := s.client.Histogram("request", float64(decoded.RequestTime), tags, s.conf.SampleRate); err != nil {
			s.log.WithError(err).Error("unable to record TimeInMilliseconds, dropping analytics record")
		}
	}

	return nil
}
