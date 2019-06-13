package pumps

import (
	"context"
	"encoding/json"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/snappy"
	"time"
)

type KafkaPump struct {
	kafkaConf   *KafkaConf
	kafkaWriter *kafka.Writer
	log         *logrus.Entry
}

type Json map[string]interface{}

var kafkaPrefix = "kafka-pump"

type KafkaConf struct {
	Broker     []string      `mapstructure:"broker"`
	ClientId   string        `mapstructure:"client_id"`
	Topic      string        `mapstructure:"topic"`
	Timeout    time.Duration `mapstructure:"timeout"`
	Compressed bool          `mapstructure:"compressed"`
}

func (k *KafkaPump) New() Pump {
	newPump := KafkaPump{}
	return &newPump
}

func (k *KafkaPump) GetName() string {
	return "Kafka Pump"
}

func (k *KafkaPump) Init(config interface{}) error {
	k.log = log.WithField("prefix", kafkaPrefix)

	//Read configuration file
	k.kafkaConf = &KafkaConf{}
	err := mapstructure.Decode(config, &k.kafkaConf)

	if err != nil {
		k.log.Fatal("Failed to decode configuration: ", err)
	}

	//Kafka writer connection config
	dialer := &kafka.Dialer{
		Timeout:  k.kafkaConf.Timeout * time.Second,
		ClientID: k.kafkaConf.ClientId,
	}

	kafkaConfig := kafka.WriterConfig{
		Brokers:      k.kafkaConf.Broker,
		Topic:        k.kafkaConf.Topic,
		Balancer:     &kafka.LeastBytes{},
		Dialer:       dialer,
		WriteTimeout: k.kafkaConf.Timeout * time.Second,
		ReadTimeout:  k.kafkaConf.Timeout * time.Second,
		Async:        true, //Non blocking write operations
	}
	if k.kafkaConf.Compressed {
		kafkaConfig.CompressionCodec = snappy.NewCompressionCodec()
	}

	//Create a new writer
	k.kafkaWriter = kafka.NewWriter(kafkaConfig)

	k.log.Debug("Broker: ", k.kafkaConf.Broker)
	k.log.Debug("ClientId: ", k.kafkaConf.ClientId)
	k.log.Debug("Topic: ", k.kafkaConf.Topic)
	k.log.Debug("Timeout: ", k.kafkaConf.Timeout)
	k.log.Debug("Compressed: ", k.kafkaConf.Compressed)
	return nil
}

func (k *KafkaPump) WriteData(data []interface{}) error {
	startTime := time.Now()
	k.log.Info("Writing ", len(data), " records...")
	for _, v := range data {
		//Build message format
		decoded := v.(analytics.AnalyticsRecord)
		message := Json{
			"@timestamp":      decoded.TimeStamp,
			"method":          decoded.Method,
			"path":            decoded.Path,
			"raw_path":        decoded.RawPath,
			"response_code":   decoded.ResponseCode,
			"alias":           decoded.Alias,
			"api_key":         decoded.APIKey,
			"api_version":     decoded.APIVersion,
			"api_name":        decoded.APIName,
			"api_id":          decoded.APIID,
			"org_id":          decoded.OrgID,
			"oauth_id":        decoded.OauthID,
			"raw_request":     decoded.RawRequest,
			"request_time_ms": decoded.RequestTime,
			"raw_response":    decoded.RawResponse,
			"ip_address":      decoded.IPAddress,
			"host":            decoded.Host,
			"content_length":  decoded.ContentLength,
			"user_agent":      decoded.UserAgent,
		}

		//Transform object to json string
		json, jsonError := json.Marshal(message)
		if jsonError != nil {
			k.log.WithError(jsonError).Error("unable to marshal message")
		}

		//Kafka message structure
		kafkaMessage := kafka.Message{
			Time:  time.Now(),
			Value: json,
		}

		//Send kafka message
		kafkaError := k.kafkaWriter.WriteMessages(context.Background(), kafkaMessage)
		if kafkaError != nil {
			k.log.WithError(kafkaError).Error("unable to write message")
		}
	}
	k.log.Debug("ElapsedTime in seconds for ", len(data), " records:", time.Now().Sub(startTime))
	return nil
}
