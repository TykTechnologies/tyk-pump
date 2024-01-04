package pumps

import (
	"context"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/aws/aws-sdk-go-v2/aws"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
)

// MockSQSSendMessageBatchAPI is a mock implementation of SQSSendMessageBatchAPI for testing purposes.
type MockSQSSendMessageBatchAPI struct {
	GetQueueUrlFunc      func(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)
	SendMessageBatchFunc func(ctx context.Context, params *sqs.SendMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error)
}

func (m *MockSQSSendMessageBatchAPI) GetQueueUrl(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
	return m.GetQueueUrlFunc(ctx, params, optFns...)
}

func (m *MockSQSSendMessageBatchAPI) SendMessageBatch(ctx context.Context, params *sqs.SendMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error) {
	return m.SendMessageBatchFunc(ctx, params, optFns...)
}

func TestSQSPump_WriteData(t *testing.T) {
	// Mock SQS client
	mockSQS := &MockSQSSendMessageBatchAPI{
		// Implement the required functions for testing
		GetQueueUrlFunc: func(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
			// Implement the mock behavior for GetQueueUrl
			return &sqs.GetQueueUrlOutput{QueueUrl: aws.String("mockQueueUrl")}, nil
		},
		SendMessageBatchFunc: func(ctx context.Context, params *sqs.SendMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error) {
			// Implement the mock behavior for SendMessageBatch
			return &sqs.SendMessageBatchOutput{}, nil
		},
	}

	// Create an instance of SQSPump with the mock SQS client
	sqsPump := &SQSPump{
		SQSClient:   mockSQS,
		SQSQueueURL: aws.String("mockQueueUrl"),
		SQSConf: &SQSConf{
			QueueName:        "test-queue",
			AWSSQSBatchLimit: 10,
		},
		log:              log.WithField("prefix", SQSPrefix), // You might want to provide a mock logger for testing
		CommonPumpConfig: CommonPumpConfig{},                 // You might want to set CommonPumpConfig fields for testing
	}

	// Create a context for testing
	ctx := context.TODO()

	// Create mock data for testing
	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111", OrgID: "123", TimeStamp: time.Now()}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123", OrgID: "1234", TimeStamp: time.Now()}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321", OrgID: "12345", TimeStamp: time.Now()}

	// Perform the WriteData operation
	err := sqsPump.WriteData(ctx, keys)

	// Assert that no error occurred during WriteData
	assert.NoError(t, err, "Unexpected error during WriteData")
}

func TestSQSPump_Chunks(t *testing.T) {
	var Calls int = 0
	// Mock SQS client
	mockSQS := &MockSQSSendMessageBatchAPI{
		// Implement the required functions for testing
		GetQueueUrlFunc: func(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
			// Implement the mock behavior for GetQueueUrl
			return &sqs.GetQueueUrlOutput{QueueUrl: aws.String("mockQueueUrl")}, nil
		},
		SendMessageBatchFunc: func(ctx context.Context, params *sqs.SendMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageBatchOutput, error) {
			// Implement the mock behavior for SendMessageBatch
			Calls++
			return &sqs.SendMessageBatchOutput{}, nil
		},
	}

	// Create an instance of SQSPump with the mock SQS client
	sqsPump := &SQSPump{
		SQSClient:   mockSQS,
		SQSQueueURL: aws.String("mockQueueUrl"),
		SQSConf: &SQSConf{
			QueueName:        "test-queue",
			AWSSQSBatchLimit: 1,
		},
		log:              log.WithField("prefix", SQSPrefix), // You might want to provide a mock logger for testing
		CommonPumpConfig: CommonPumpConfig{},                 // You might want to set CommonPumpConfig fields for testing
	}

	// Create a context for testing
	ctx := context.TODO()

	// Create mock data for testing
	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111", OrgID: "123", TimeStamp: time.Now()}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123", OrgID: "1234", TimeStamp: time.Now()}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321", OrgID: "12345", TimeStamp: time.Now()}

	// Perform the WriteData operation
	err := sqsPump.WriteData(ctx, keys)

	// Assert that no error occurred during WriteData
	assert.NoError(t, err, "Unexpected error during WriteData")
	assert.Equal(t, len(keys), Calls)
}
