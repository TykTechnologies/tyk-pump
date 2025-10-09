package pumps

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
)

// KinesisPump is a Tyk Pump that sends analytics records to AWS Kinesis.
type KinesisPump struct {
	client      *kinesis.Client
	kinesisConf *KinesisConf
	log         *logrus.Entry
	CommonPumpConfig
}

// @PumpConf Kinesis
type KinesisConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_KINESIS_META`
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// A name to identify the stream. The stream name is scoped to the AWS account used by the application
	// that creates the stream. It is also scoped by AWS Region.
	// That is, two streams in two different AWS accounts can have the same name.
	// Two streams in the same AWS account but in two different Regions can also have the same name.
	StreamName string `mapstructure:"stream_name"`
	// AWS Region the Kinesis stream targets
	Region string `mapstructure:"region"`
	// Each PutRecords (the function used in this pump)request can support up to 500 records.
	// Each record in the request can be as large as 1 MiB, up to a limit of 5 MiB for the entire request, including partition keys.
	// Each shard can support writes up to 1,000 records per second, up to a maximum data write total of 1 MiB per second.
	BatchSize int `mapstructure:"batch_size"`
	// The KMS Key ID used for server-side encryption of the Kinesis stream.
	// Defaults to an empty string if not provided.
	KMSKeyID string `mapstructure:"kms_key_id" default:""`
}

var (
	kinesisPrefix     = "kinesis-pump"
	kinesisDefaultENV = PUMPS_ENV_PREFIX + "_KINESIS" + PUMPS_ENV_META_PREFIX
)

func (p *KinesisPump) New() Pump {
	newPump := KinesisPump{}
	return &newPump
}

// Init initializes the pump with configuration settings.
func (p *KinesisPump) Init(config interface{}) error {
	p.log = log.WithField("prefix", kinesisPrefix)

	// Read configuration file
	p.kinesisConf = &KinesisConf{}
	err := mapstructure.Decode(config, &p.kinesisConf)
	if err != nil {
		p.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(p, p.log, p.kinesisConf, kinesisDefaultENV)

	// Load AWS configuration
	// Credentials are loaded as specified in
	// https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithRegion(p.kinesisConf.Region))
	if err != nil {
		p.log.Fatalf("unable to load Kinesis SDK config, %v", err)
	}

	defaultBatchSize := 100
	if p.kinesisConf.BatchSize == 0 {
		p.kinesisConf.BatchSize = defaultBatchSize
	}

	if p.kinesisConf.StreamName == "" {
		p.log.Error("Stream name unset - may be unable to produce records")
	}

	// Create Kinesis client
	p.client = kinesis.NewFromConfig(cfg)

	// Check if KMSKeyID is provided and enable server-side encryption
	if p.kinesisConf.KMSKeyID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := p.client.StartStreamEncryption(ctx, &kinesis.StartStreamEncryptionInput{
			StreamName:     aws.String(p.kinesisConf.StreamName),
			EncryptionType: types.EncryptionTypeKms,
			KeyId:          aws.String(p.kinesisConf.KMSKeyID),
		})

		if err != nil {
			var resourceInUseErr *types.ResourceInUseException
			if errors.As(err, &resourceInUseErr) {
				p.log.Info("Server-side encryption is already enabled for the Kinesis stream.")
			} else {
				p.log.Fatalf("Failed to enable server-side encryption for Kinesis stream: %v", err)
			}
		} else {
			kmsKeyID := p.kinesisConf.KMSKeyID
			loggableKeyID := "***" // Default to a fully masked key
			if len(kmsKeyID) >= 8 {
				loggableKeyID = fmt.Sprintf("%s***%s", kmsKeyID[:4], kmsKeyID[len(kmsKeyID)-4:])
			}
			p.log.Info("Server-side encryption enabled for Kinesis stream with KMS Key ID: ", loggableKeyID)
		}
	}

	p.log.Info(p.GetName() + " Initialized")

	return nil
}

// WriteData writes the analytics records to AWS Kinesis in batches.
func (p *KinesisPump) WriteData(ctx context.Context, records []interface{}) error {
	batches := splitIntoBatches(records, p.kinesisConf.BatchSize)
	for _, batch := range batches {
		var entries []types.PutRecordsRequestEntry
		for _, record := range batch {
			// Build message format
			decoded, ok := record.(analytics.AnalyticsRecord)
			if !ok {
				p.log.WithField("record", record).Error("unable to decode record")
				continue
			}
			//nolint:dupl
			analyticsRecord := Json{
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
				"tags":            decoded.Tags,
			}

			// Transform object to json string
			json, jsonError := json.Marshal(analyticsRecord)
			if jsonError != nil {
				p.log.WithError(jsonError).Error("unable to marshal message")
			}

			n, err := rand.Int(rand.Reader, big.NewInt(1000000000))
			if err != nil {
				p.log.Error("failed to generate int for Partition key: ", err)
			}

			// Partition key uses a string representation of Int
			// Should even distribute across shards as AWS uses md5 of each message partition key
			entry := types.PutRecordsRequestEntry{
				Data:         json,
				PartitionKey: aws.String(fmt.Sprint(n)),
			}
			entries = append(entries, entry)
		}

		input := &kinesis.PutRecordsInput{
			StreamName: aws.String(p.kinesisConf.StreamName),
			Records:    entries,
		}

		output, err := p.client.PutRecords(ctx, input)
		if err != nil {
			p.log.Error("failed to put records to Kinesis: ", err)
		}

		// Check for failed records
		if output != nil {
			for _, record := range output.Records {
				if record.ErrorCode != nil {
					p.log.Debugf("Failed to put record: %s - %s", aws.ToString(record.ErrorCode), aws.ToString(record.ErrorMessage))
				}
				p.log.Debug(record)
			}
			p.log.Info("Purged ", len(output.Records), " records...")
		}
	}
	return nil
}

// splitIntoBatches splits the records into batches of the specified size.
func splitIntoBatches(records []interface{}, batchSize int) [][]interface{} {
	var batches [][]interface{}
	for batchSize < len(records) {
		records, batches = records[batchSize:], append(batches, records[0:batchSize:batchSize])
	}
	return append(batches, records)
}

// GetName returns the name of the pump.
func (p *KinesisPump) GetName() string {
	return "Kinesis Pump"
}

func (p *KinesisPump) GetEnvPrefix() string {
	return p.kinesisConf.EnvPrefix
}
