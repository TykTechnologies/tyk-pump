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

type DogStatsdPump struct {
	conf   *DogStatsdConf
	client *statsd.Client
	log    *logrus.Entry
}

type DogStatsdConf struct {
	Namespace string `mapstructure:"namespace"`
	Address   string `mapstructure:"address"`
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

	s.conf = &DogStatsdConf{}
	if err := mapstructure.Decode(conf, &s.conf); err != nil {
		return errors.Wrap(err, "unable to decode dogstatsd configuration")
	}

	if err := s.connect(); err != nil {
		return errors.Wrap(err, "unable to connect to dogstatsd client")
	}
	defer s.disconnect()

	return nil
}

func (s *DogStatsdPump) connect() error {
	c, err := statsd.New(s.conf.Address)
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

	if err := s.connect(); err != nil {
		s.log.WithError(err).Error("unable to connect to dogstatsd client")
		return err
	}
	defer s.disconnect()

	s.log.Info(fmt.Sprintf("purging %d records", len(data)))
	for _, v := range data {
		// Convert to AnalyticsRecord
		decoded := v.(analytics.AnalyticsRecord)

		decoded.Path = strings.TrimRight(decoded.Path, "/")

		//analyticsJsBytes, _ := json.MarshalIndent(decoded, "", "  ")
		//log.Infof("%s", analyticsJsBytes)

		tags := []string{
			"path:" + decoded.Path,
			"method:" + decoded.Method,
			fmt.Sprintf("response_code:%d", decoded.ResponseCode),
			"api_key:" + decoded.APIKey,
			fmt.Sprintf("time_stamp:%d", time.Unix(decoded.TimeStamp.Unix(), 0).Unix()),
			"api_version:" + decoded.APIVersion,
			"api_name:" + decoded.APIName,
			"api_id:" + decoded.APIID,
			"org_id:" + decoded.OrgID,
			"oauth_id:" + decoded.OauthID,
			fmt.Sprintf("request_time:%d", decoded.RequestTime),
			"ip_address:" + decoded.IPAddress,
		}

		for _, v := range decoded.Tags {
			tags = append(tags, strings.Replace(v, "-", ":", 1))
		}

		if err := s.client.TimeInMilliseconds("request", float64(decoded.RequestTime), tags, 1); err != nil {
			s.log.WithError(err).Error("unable to record request")
		}
	}

	return nil
}
