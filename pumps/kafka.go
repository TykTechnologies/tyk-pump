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
	writerConfig kafka.WriterConfig
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

	k.writerConfig.Brokers =k.kafkaConf.Broker
	k.writerConfig.Topic =k.kafkaConf.Topic
	k.writerConfig.Balancer =&kafka.LeastBytes{}
	k.writerConfig.Dialer =dialer
	k.writerConfig.WriteTimeout =k.kafkaConf.Timeout * time.Second
	k.writerConfig.ReadTimeout =k.kafkaConf.Timeout * time.Second
	if k.kafkaConf.Compressed {
		k.writerConfig.CompressionCodec = snappy.NewCompressionCodec()
	}

	k.log.Info("Kafka config: ",k.writerConfig)
	return nil
}

func (k *KafkaPump) WriteData(data []interface{}) error {
	startTime := time.Now()
	k.log.Info("Writing ", len(data), " records...")
	kafkaMessages := make([]kafka.Message, len(data))
	for i, v := range data {
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
		kafkaMessages[i] = kafka.Message{
			Time:  time.Now(),
			Value: json,
		}
	}
	//Send kafka message
	kafkaError := k.write(kafkaMessages)
	if kafkaError != nil {
		k.log.WithError(kafkaError).Error("unable to write message")
	}
	k.log.Debug("ElapsedTime in seconds for ", len(data), " records:", time.Now().Sub(startTime))
	return nil
}

func (k *KafkaPump) write(messages []kafka.Message) error{
	kafkaWriter := kafka.NewWriter(k.writerConfig)
	defer kafkaWriter.Close()
	return kafkaWriter.WriteMessages(context.Background(), messages...)
}