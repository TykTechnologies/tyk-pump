package pumps

import (
	"context"
	"encoding/json"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"time"
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

var SQSPrefix = "sqs-pump"
var SQSDefaultENV = PUMPS_ENV_PREFIX + "_SQS" + PUMPS_ENV_META_PREFIX

// @PumpConf SQS
type SQSConf struct {
	EnvPrefix         string `mapstructure:"meta_env_prefix"`
	QueueName         string `mapstructure:"aws_queue_name"`
	AWSRegion         string `mapstructure:"aws_region"`
	AWSSecret         string `mapstructure:"aws_secret"`
	AWSKey            string `mapstructure:"aws_key"`
	AWSEndpoint       string `mapstructure:"aws_endpoint"`
	AWSDelaySeconds   int32  `mapstructure:"aws_delay_seconds"`
	AWSMessageGroupID string `mapstructure:"aws_message_group_id"`
	AWSSQSBatchLimit  int    `mapstructure:"aws_sqs_batch_limit"`
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
		decoded := v.(analytics.AnalyticsRecord)
		decodedMessageByteArray, _ := json.Marshal(decoded)
		messages[i] = types.SendMessageBatchRequestEntry{
			MessageBody: aws.String(string(decodedMessageByteArray)),
			Id:          aws.String(decoded.GetObjectID().String()),
		}
		if s.SQSConf.AWSMessageGroupID != "" {
			messages[i].MessageGroupId = aws.String(s.SQSConf.AWSMessageGroupID)
		}
		if s.SQSConf.AWSDelaySeconds != 0 {
			messages[i].DelaySeconds = s.SQSConf.AWSDelaySeconds
		}
	}
	SQSError := s.write(ctx, messages)
	if SQSError != nil {
		s.log.WithError(SQSError).Error("unable to write message")

		return SQSError
	}
	s.log.Debug("ElapsedTime in seconds for ", len(data), " records:", time.Now().Sub(startTime))
	s.log.Info("Purged ", len(data), " records...")
	return nil
}

func (s *SQSPump) write(c context.Context, messages []types.SendMessageBatchRequestEntry) error {
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
