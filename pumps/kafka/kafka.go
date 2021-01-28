package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/mitchellh/mapstructure"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/segmentio/kafka-go/snappy"
)



type KafkaPump struct {
	kafkaConf    *KafkaConf

	kafkaClient KafkaClient
	log          *logrus.Entry
	pumps.CommonPumpConfig
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
	SSLCertFile           string            `mapstructure:"ssl_cert_file"`
	SSLKeyFile            string            `mapstructure:"ssl_key_file"`
	SASLMechanism         string            `mapstructure:"sasl_mechanism"`
	Username              string            `mapstructure:"sasl_username"`
	Password              string            `mapstructure:"sasl_password"`
	Algorithm             string            `mapstructure:"sasl_algorithm"`
}

func (k *KafkaPump) New() pumps.Pump {
	newPump := KafkaPump{}
	return &newPump
}

func (k *KafkaPump) GetName() string {
	return "Kafka Pump"
}

func (k *KafkaPump) Init(config interface{}) error {
	k.log = logger.GetLogger().WithField("prefix", kafkaPrefix)

	//Read configuration file
	k.kafkaConf = &KafkaConf{}
	err := mapstructure.Decode(config, &k.kafkaConf)

	if err != nil {
		k.log.Fatal("Failed to decode configuration: ", err)
	}

	var tlsConfig *tls.Config
	var cert tls.Certificate
	if k.kafkaConf.UseSSL {
		if k.kafkaConf.SSLCertFile != "" && k.kafkaConf.SSLKeyFile != "" {
			var cert tls.Certificate
			k.log.Debug("Loading certificates for mTLS.")
			cert, err = tls.LoadX509KeyPair(k.kafkaConf.SSLCertFile, k.kafkaConf.SSLKeyFile)
			if err != nil {
				k.log.Debug("Error loading mTLS certificates:", err)
				return err
			}
			tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: k.kafkaConf.SSLInsecureSkipVerify,
			}
		} else if k.kafkaConf.SSLCertFile != "" || k.kafkaConf.SSLKeyFile != "" {
			k.log.Error("Only one of ssl_cert_file and ssl_cert_key configuration option is setted, you should set both to enable mTLS.")
		} else {
			tlsConfig = &tls.Config{
				InsecureSkipVerify: k.kafkaConf.SSLInsecureSkipVerify,
			}
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
	tlsConfig = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: k.kafkaConf.SSLInsecureSkipVerify,
	}

	//Kafka writer connection config
	dialer := &kafka.Dialer{
		Timeout:       k.kafkaConf.Timeout * time.Second,
		ClientID:      k.kafkaConf.ClientId,
		TLS:           tlsConfig,
		SASLMechanism: mechanism,
	}

	writerConfig := kafka.WriterConfig{}
	writerConfig.Brokers = k.kafkaConf.Broker
	writerConfig.Topic = k.kafkaConf.Topic
	writerConfig.Balancer = &kafka.LeastBytes{}
	writerConfig.Dialer = dialer
	writerConfig.WriteTimeout = k.kafkaConf.Timeout * time.Second
	writerConfig.ReadTimeout = k.kafkaConf.Timeout * time.Second
	if k.kafkaConf.Compressed {
		writerConfig.CompressionCodec = snappy.NewCompressionCodec()
	}

	k.log.Info("Kafka config: ", writerConfig)

	k.kafkaClient = kafka.NewWriter(writerConfig)
	return nil
}

var timeNow = time.Now

func (k *KafkaPump) WriteData(ctx context.Context, data []interface{}) error {
	startTime := time.Now()
	k.log.Info("Writing ", len(data), " records...")
	messages := make([]kafka.Message, len(data))
	for i, v := range data {
		//Build message format
		decoded := v.(analytics.AnalyticsRecord)
		message := fromRecordToJson(decoded)
		//Add static metadata to json
		for key, value := range k.kafkaConf.MetaData {
			message[key] = value
		}

		//Transform object to json string
		json, jsonError := json.Marshal(message)
		if jsonError != nil {
			k.log.WithError(jsonError).Error("unable to marshal message")
			return errors.New("unable to marshal kafka message:"+jsonError.Error())
		}

		//Kafka message structure
		messages[i] = kafka.Message{
			Time:  timeNow(),
			Value: json,
		}
	}
	//Send kafka message
	kafkaError :=  k.kafkaClient.WriteMessages(ctx, messages...)
	if kafkaError != nil {
		k.log.WithError(kafkaError).Error("unable to write message")
		return errors.New("error writing message to kafka:"+kafkaError.Error())
	}
	k.log.Debug("ElapsedTime in seconds for ", len(data), " records:", time.Now().Sub(startTime))
	return nil
}

func fromRecordToJson(record analytics.AnalyticsRecord) Json{
	return Json{
		"timestamp":       record.TimeStamp,
		"method":          record.Method,
		"path":            record.Path,
		"raw_path":        record.RawPath,
		"response_code":   record.ResponseCode,
		"alias":           record.Alias,
		"api_key":         record.APIKey,
		"api_version":     record.APIVersion,
		"api_name":        record.APIName,
		"api_id":          record.APIID,
		"org_id":          record.OrgID,
		"oauth_id":        record.OauthID,
		"raw_request":     record.RawRequest,
		"request_time_ms": record.RequestTime,
		"raw_response":    record.RawResponse,
		"ip_address":      record.IPAddress,
		"host":            record.Host,
		"content_length":  record.ContentLength,
		"user_agent":      record.UserAgent,
	}
}
