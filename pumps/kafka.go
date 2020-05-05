package pumps

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/segmentio/kafka-go/snappy"
)

type KafkaPump struct {
	kafkaConf    *KafkaConf
	writerConfig kafka.WriterConfig
	log          *logrus.Entry
	timeout      int
}

type Json map[string]interface{}

var kafkaPrefix = "kafka-pump"

type KafkaConf struct {
	Broker                []string          `mapstructure:"broker"`
	ClientId              string            `mapstructure:"client_id"`
	Topic                 string            `mapstructure:"topic"`
	Timeout               time.Duration     `mapstructure:"timeout"`
	Compressed            bool              `mapstructure:"compressed"`
	MetaData              map[string]string `mapstructure:"meta_data"`
	UseSSL                bool              `mapstructure:"use_ssl"`
	SSLInsecureSkipVerify bool              `mapstructure:"ssl_insecure_skip_verify"`
	SASLMechanism         string            `mapstructure:"sasl_mechanism"`
	Username              string            `mapstructure:"sasl_username"`
	Password              string            `mapstructure:"sasl_password"`
	Algorithm             string            `mapstructure:"sasl_algorithm"`
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

	var tlsConfig *tls.Config
	if k.kafkaConf.UseSSL {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: k.kafkaConf.SSLInsecureSkipVerify,
		}
	} else if k.kafkaConf.SASLMechanism != "" {
		k.log.WithField("SASL-Mechanism", k.kafkaConf.SASLMechanism).Warn("SASL-Mechanism is setted but use_ssl is false.")
	}

	var mechanism sasl.Mechanism

	switch k.kafkaConf.SASLMechanism {
	case "":
		break
	case "PLAIN", "plain":
		mechanism = plain.Mechanism{Username: k.kafkaConf.Username, Password: k.kafkaConf.Password}
	case "SCRAM", "scram":
		algorithm := scram.SHA256
		if k.kafkaConf.Algorithm == "sha-512" || k.kafkaConf.Algorithm == "SHA-512" {
			algorithm = scram.SHA512
		}
		var mechErr error
		mechanism, mechErr = scram.Mechanism(algorithm, k.kafkaConf.Username, k.kafkaConf.Password)
		if mechErr != nil {
			k.log.Fatal("Failed initialize kafka mechanism  : ", mechErr)
		}
	default:
		k.log.WithField("SASL-Mechanism", k.kafkaConf.SASLMechanism).Warn("Tyk pump doesn't support this SASL mechanism.")
	}

	//Kafka writer connection config
	dialer := &kafka.Dialer{
		Timeout:       k.kafkaConf.Timeout * time.Second,
		ClientID:      k.kafkaConf.ClientId,
		TLS:           tlsConfig,
		SASLMechanism: mechanism,
	}

	k.writerConfig.Brokers = k.kafkaConf.Broker
	k.writerConfig.Topic = k.kafkaConf.Topic
	k.writerConfig.Balancer = &kafka.LeastBytes{}
	k.writerConfig.Dialer = dialer
	k.writerConfig.WriteTimeout = k.kafkaConf.Timeout * time.Second
	k.writerConfig.ReadTimeout = k.kafkaConf.Timeout * time.Second
	if k.kafkaConf.Compressed {
		k.writerConfig.CompressionCodec = snappy.NewCompressionCodec()
	}

	k.log.Info("Kafka config: ", k.writerConfig)
	return nil
}

func (k *KafkaPump) WriteData(ctx context.Context, data []interface{}) error {
	startTime := time.Now()
	k.log.Info("Writing ", len(data), " records...")
	kafkaMessages := make([]kafka.Message, len(data))
	for i, v := range data {
		//Build message format
		decoded := v.(analytics.AnalyticsRecord)
		message := Json{
			"timestamp":       decoded.TimeStamp,
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
		//Add static metadata to json
		for key, value := range k.kafkaConf.MetaData {
			message[key] = value
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
	kafkaError := k.write(ctx, kafkaMessages)
	if kafkaError != nil {
		k.log.WithError(kafkaError).Error("unable to write message")
	}
	k.log.Debug("ElapsedTime in seconds for ", len(data), " records:", time.Now().Sub(startTime))
	return nil
}

func (k *KafkaPump) write(ctx context.Context, messages []kafka.Message) error {
	kafkaWriter := kafka.NewWriter(k.writerConfig)
	defer kafkaWriter.Close()
	return kafkaWriter.WriteMessages(ctx, messages...)
}

func (k *KafkaPump) SetTimeout(timeout int) {
	k.timeout = timeout
}

func (k *KafkaPump) GetTimeout() int {
	return k.timeout
}
