package pumps

import (
	"context"
	"encoding/json"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/mitchellh/mapstructure"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
)

type SQSSendMessageBatchAPI interface {
	GetQueueUrl(ctx context.Context,
		params *sqs.GetQueueUrlInput,
		optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)

	SendMessageBatch(ctx context.Context,
		params *sqs.SendMessageBatchInput,
		optFns ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error)
}

type SQSPump struct {
	SQSClient   SQSSendMessageBatchAPI
	SQSQueueURL *string
	SQSConf     *SQSConf
	log         *logrus.Entry
	CommonPumpConfig
}

var (
	SQSPrefix     = "sqs-pump"
	SQSDefaultENV = PUMPS_ENV_PREFIX + "_SQS" + PUMPS_ENV_META_PREFIX
)

// SQSConf represents the configuration structure for the Tyk Pump SQS (Simple Queue Service) pump.
type SQSConf struct {
	// EnvPrefix defines the prefix for environment variables related to this SQS configuration.
	EnvPrefix string `mapstructure:"meta_env_prefix"`

	// QueueName specifies the name of the AWS Simple Queue Service (SQS) queue for message delivery.
	QueueName string `mapstructure:"aws_queue_name"`

	// AWSRegion sets the AWS region where the SQS queue is located.
	AWSRegion string `mapstructure:"aws_region"`

	// AWSSecret is the AWS secret key used for authentication.
	AWSSecret string `mapstructure:"aws_secret"`

	// AWSKey is the AWS access key ID used for authentication.
	AWSKey string `mapstructure:"aws_key"`

	// AWSEndpoint is the custom endpoint URL for AWS SQS, if applicable.
	AWSEndpoint string `mapstructure:"aws_endpoint"`

	// AWSMessageGroupID specifies the message group ID for ordered processing within the SQS queue.
	AWSMessageGroupID string `mapstructure:"aws_message_group_id"`

	// AWSMessageIDDeduplicationEnabled enables/disables message deduplication based on unique IDs.
	AWSMessageIDDeduplicationEnabled bool `mapstructure:"aws_message_id_deduplication_enabled"`

	// AWSDelaySeconds configures the delay (in seconds) before messages become available for processing.
	AWSDelaySeconds int32 `mapstructure:"aws_delay_seconds"`

	// AWSSQSBatchLimit sets the maximum number of messages in a single batch when sending to the SQS queue.
	AWSSQSBatchLimit int `mapstructure:"aws_sqs_batch_limit"`
}

func (s *SQSPump) New() Pump {
	newPump := SQSPump{}
	return &newPump
}

func (s *SQSPump) GetName() string {
	return "SQS Pump"
}

func (s *SQSPump) GetEnvPrefix() string {
	return s.SQSConf.EnvPrefix
}

func (s *SQSPump) Init(config interface{}) error {
	s.SQSConf = &SQSConf{}
	s.log = log.WithField("prefix", SQSPrefix)

	err := mapstructure.Decode(config, &s.SQSConf)
	if err != nil {
		s.log.Fatal("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(s, s.log, s.SQSConf, SQSDefaultENV)

	s.SQSClient, err = s.NewSQSPublisher()
	if err != nil {
		s.log.Fatal("Failed to create sqs client: ", err)
		return err
	}
	// Get URL of queue
	gQInput := &sqs.GetQueueUrlInput{
		QueueName: aws.String(s.SQSConf.QueueName),
	}

	result, err := s.SQSClient.GetQueueUrl(context.TODO(), gQInput)
	if err != nil {
		return err
	}
	s.SQSQueueURL = result.QueueUrl

	s.log.Info(s.GetName() + " Initialized")

	return nil
}

func (s *SQSPump) WriteData(ctx context.Context, data []interface{}) error {
	s.log.Info("Attempting to write ", len(data), " records...")
	startTime := time.Now()

	messages := make([]types.SendMessageBatchRequestEntry, len(data))
	for i, v := range data {
		decoded, ok := v.(analytics.AnalyticsRecord)
		if !ok {
			s.log.Errorf("Unable to decode message: %v", v)
			continue
		}
		decodedMessageByteArray, err := json.Marshal(decoded)
		if err != nil {
			s.log.Errorf("Unable to marshal message: %v", err)
			continue
		}
		messages[i] = types.SendMessageBatchRequestEntry{
			MessageBody: aws.String(string(decodedMessageByteArray)),
			Id:          aws.String(ulid.Make().String()),
		}
		if s.SQSConf.AWSMessageGroupID != "" {
			messages[i].MessageGroupId = aws.String(s.SQSConf.AWSMessageGroupID)
		}
		if s.SQSConf.AWSDelaySeconds != 0 {
			messages[i].DelaySeconds = s.SQSConf.AWSDelaySeconds
		}

		// for FIFO SQS
		if s.SQSConf.AWSMessageGroupID != "" {
			messages[i].MessageGroupId = aws.String(s.SQSConf.AWSMessageGroupID)
		}
		if s.SQSConf.AWSMessageIDDeduplicationEnabled {
			messages[i].MessageDeduplicationId = messages[i].Id
		}
	}
	SQSError := s.write(ctx, messages)
	if SQSError != nil {
		s.log.WithError(SQSError).Error("unable to write message")

		return SQSError
	}
	s.log.Debug("ElapsedTime in seconds for ", len(data), " records:", time.Since(startTime))
	s.log.Info("Purged ", len(data), " records...")
	return nil
}

func (s *SQSPump) write(c context.Context, messages []types.SendMessageBatchRequestEntry) error {
	log.Debug(messages)
	for i := 0; i < len(messages); i += s.SQSConf.AWSSQSBatchLimit {
		end := i + s.SQSConf.AWSSQSBatchLimit

		if end > len(messages) {
			end = len(messages)
		}
		sMInput := &sqs.SendMessageBatchInput{
			Entries:  messages[i:end],
			QueueUrl: s.SQSQueueURL,
		}

		if _, err := s.SQSClient.SendMessageBatch(c, sMInput); err != nil {
			return err
		}
	}

	return nil
}

func (s *SQSPump) NewSQSPublisher() (c *sqs.Client, err error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(s.SQSConf.AWSRegion),
	)
	if err != nil {
		return nil, err
	}

	client := sqs.NewFromConfig(cfg, func(options *sqs.Options) {
		if s.SQSConf.AWSEndpoint != "" {
			options.BaseEndpoint = aws.String(s.SQSConf.AWSEndpoint)
		}
		if s.SQSConf.AWSKey != "" && s.SQSConf.AWSSecret != "" {
			options.Credentials = credentials.NewStaticCredentialsProvider(s.SQSConf.AWSKey, s.SQSConf.AWSSecret, "")
		}
	})

	return client, nil
}
