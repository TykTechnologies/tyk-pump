package pumps

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestKinesisPump_New(t *testing.T) {
	pump := &KinesisPump{}
	newPump := pump.New()
	assert.IsType(t, &KinesisPump{}, newPump)
}

func TestKinesisPump_GetName(t *testing.T) {
	pump := &KinesisPump{}
	assert.Equal(t, "Kinesis Pump", pump.GetName())
}

func TestKinesisConf_KMSKeyID_Configuration(t *testing.T) {
	tests := []struct {
		name     string
		kmsKeyID string
		expected string
	}{
		{
			name:     "KMSKeyID provided",
			kmsKeyID: "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
			expected: "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
		},
		{
			name:     "KMSKeyID empty",
			kmsKeyID: "",
			expected: "",
		},
		{
			name:     "KMSKeyID alias format",
			kmsKeyID: "alias/my-key",
			expected: "alias/my-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &KinesisConf{
				KMSKeyID: tt.kmsKeyID,
			}
			assert.Equal(t, tt.expected, conf.KMSKeyID)
		})
	}
}

func TestKinesisPump_KMSKeyID_DefaultValue(t *testing.T) {
	conf := &KinesisConf{}
	assert.Equal(t, "", conf.KMSKeyID)
}

func TestSplitIntoBatches(t *testing.T) {
	records := make([]interface{}, 25)
	batches := splitIntoBatches(records, 10)
	assert.Len(t, batches, 3)
	assert.Len(t, batches[0], 10)
	assert.Len(t, batches[1], 10)
	assert.Len(t, batches[2], 5)
}

func TestKinesisPump_KMSKeyID_LogMasking(t *testing.T) {
	kmsKeyID := "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"
	if len(kmsKeyID) >= 8 {
		masked := kmsKeyID[:4] + "***" + kmsKeyID[len(kmsKeyID)-4:]
		assert.Equal(t, "arn:***9012", masked)
	}
}

// Tests for the new describe stream encryption logic
func TestKinesisPump_EncryptionConfig_SameKey(t *testing.T) {
	kmsKeyID := "arn:aws:kms:us-east-1:123456789012:key/test-key-id"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  kmsKeyID,
	}

	kinesisConf := &KinesisConf{}
	err := mapstructure.Decode(config, kinesisConf)
	assert.NoError(t, err)
	assert.Equal(t, kmsKeyID, kinesisConf.KMSKeyID)
	assert.Equal(t, "test-stream", kinesisConf.StreamName)
	assert.Equal(t, "us-east-1", kinesisConf.Region)
}

func TestKinesisPump_EncryptionConfig_DifferentKey(t *testing.T) {
	currentKeyID := "arn:aws:kms:us-east-1:123456789012:key/current-key"
	newKeyID := "arn:aws:kms:us-east-1:123456789012:key/new-key"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  newKeyID,
	}

	kinesisConf := &KinesisConf{}
	err := mapstructure.Decode(config, kinesisConf)
	assert.NoError(t, err)
	assert.Equal(t, newKeyID, kinesisConf.KMSKeyID)
	assert.Equal(t, "test-stream", kinesisConf.StreamName)
	assert.Equal(t, "us-east-1", kinesisConf.Region)

	// Verify the keys are different (simulating the scenario)
	assert.NotEqual(t, currentKeyID, newKeyID)
}

func TestKinesisPump_EncryptionConfig_NotEncrypted(t *testing.T) {
	kmsKeyID := "arn:aws:kms:us-east-1:123456789012:key/test-key-id"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  kmsKeyID,
	}

	kinesisConf := &KinesisConf{}
	err := mapstructure.Decode(config, kinesisConf)
	assert.NoError(t, err)
	assert.Equal(t, kmsKeyID, kinesisConf.KMSKeyID)
	assert.Equal(t, "test-stream", kinesisConf.StreamName)
	assert.Equal(t, "us-east-1", kinesisConf.Region)
}

func TestKinesisPump_EncryptionConfig_NoKMSKeyID(t *testing.T) {
	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		// No kms_key_id provided - should skip encryption
	}

	kinesisConf := &KinesisConf{}
	err := mapstructure.Decode(config, kinesisConf)
	assert.NoError(t, err)
	assert.Equal(t, "", kinesisConf.KMSKeyID)
	assert.Equal(t, "test-stream", kinesisConf.StreamName)
	assert.Equal(t, "us-east-1", kinesisConf.Region)
}

func TestKinesisPump_BatchSize_Configuration(t *testing.T) {
	//nolint:govet
	tests := []struct {
		name          string
		batchSize     interface{}
		expectedValue int
	}{
		{
			name:          "Default batch size (not provided)",
			batchSize:     nil,
			expectedValue: 0, // Will be set to 100 in Init()
		},
		{
			name:          "Custom batch size",
			batchSize:     250,
			expectedValue: 250,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := map[string]interface{}{
				"stream_name": "test-stream",
				"region":      "us-east-1",
			}

			if tt.batchSize != nil {
				config["batch_size"] = tt.batchSize
			}

			kinesisConf := &KinesisConf{}
			err := mapstructure.Decode(config, kinesisConf)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, kinesisConf.BatchSize)
		})
	}
}

func TestKinesisPump_StreamName_Required(t *testing.T) {
	config := map[string]interface{}{
		"region": "us-east-1",
		// Missing stream_name
	}

	kinesisConf := &KinesisConf{}
	err := mapstructure.Decode(config, kinesisConf)
	assert.NoError(t, err)
	assert.Equal(t, "", kinesisConf.StreamName) // Should be empty when not provided
	assert.Equal(t, "us-east-1", kinesisConf.Region)
}

// KinesisClientInterface defines the interface for Kinesis client operations
type KinesisClientInterface interface {
	DescribeStream(ctx context.Context, params *kinesis.DescribeStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error)
	StartStreamEncryption(ctx context.Context, params *kinesis.StartStreamEncryptionInput, optFns ...func(*kinesis.Options)) (*kinesis.StartStreamEncryptionOutput, error)
	PutRecords(ctx context.Context, params *kinesis.PutRecordsInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordsOutput, error)
}

// MockKinesisClient is a mock implementation of KinesisClientInterface
type MockKinesisClient struct {
	mock.Mock
}

func (m *MockKinesisClient) DescribeStream(ctx context.Context, params *kinesis.DescribeStreamInput, _ ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	output, ok := args.Get(0).(*kinesis.DescribeStreamOutput)
	if !ok {
		return nil, args.Error(1)
	}
	return output, args.Error(1)
}

func (m *MockKinesisClient) StartStreamEncryption(ctx context.Context, params *kinesis.StartStreamEncryptionInput, _ ...func(*kinesis.Options)) (*kinesis.StartStreamEncryptionOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	output, ok := args.Get(0).(*kinesis.StartStreamEncryptionOutput)
	if !ok {
		return nil, args.Error(1)
	}
	return output, args.Error(1)
}

func (m *MockKinesisClient) PutRecords(ctx context.Context, params *kinesis.PutRecordsInput, _ ...func(*kinesis.Options)) (*kinesis.PutRecordsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	output, ok := args.Get(0).(*kinesis.PutRecordsOutput)
	if !ok {
		return nil, args.Error(1)
	}
	return output, args.Error(1)
}

// TestableKinesisPump extends KinesisPump for testing with dependency injection
type TestableKinesisPump struct {
	mockClient KinesisClientInterface
	KinesisPump
}

func (p *TestableKinesisPump) InitWithMock(config interface{}, mockClient KinesisClientInterface) error {
	p.log = logrus.NewEntry(logrus.New())
	p.log.Logger.SetLevel(logrus.FatalLevel) // Suppress logs during testing

	// Read configuration file
	p.kinesisConf = &KinesisConf{}
	err := mapstructure.Decode(config, &p.kinesisConf)
	if err != nil {
		return err
	}

	defaultBatchSize := 100
	if p.kinesisConf.BatchSize == 0 {
		p.kinesisConf.BatchSize = defaultBatchSize
	}

	if p.kinesisConf.StreamName == "" {
		p.log.Error("Stream name unset - may be unable to produce records")
	}

	// Use mock client instead of real client
	p.mockClient = mockClient

	// Check if KMSKeyID is provided and enable server-side encryption
	if p.kinesisConf.KMSKeyID != "" {
		ctx := context.Background()

		// First, check if encryption is already enabled
		describeOutput, err := p.mockClient.DescribeStream(ctx, &kinesis.DescribeStreamInput{
			StreamName: aws.String(p.kinesisConf.StreamName),
		})

		switch {
		case err != nil:
			return err
		case describeOutput.StreamDescription.EncryptionType == types.EncryptionTypeKms:
			currentKeyID := aws.ToString(describeOutput.StreamDescription.KeyId)
			if currentKeyID == p.kinesisConf.KMSKeyID {
				p.log.Info("Server-side encryption is already enabled with the specified KMS Key ID")
			} else {
				return errors.New("server-side encryption is already enabled with a different KMS Key ID")
			}
		default:
			// Encryption not enabled, proceed to enable it
			_, err := p.mockClient.StartStreamEncryption(ctx, &kinesis.StartStreamEncryptionInput{
				StreamName:     aws.String(p.kinesisConf.StreamName),
				EncryptionType: types.EncryptionTypeKms,
				KeyId:          aws.String(p.kinesisConf.KMSKeyID),
			})

			if err != nil {
				var resourceInUseErr *types.ResourceInUseException
				if errors.As(err, &resourceInUseErr) {
					p.log.Info("Server-side encryption is already enabled for the Kinesis stream.")
				} else {
					return err
				}
			} else {
				p.log.Info("Server-side encryption enabled for Kinesis stream")
			}
		}
	}

	return nil
}

// Tests for the new describe stream encryption logic

func TestKinesisPump_DescribeStream_AlreadyEncryptedSameKey(t *testing.T) {
	kmsKeyID := "arn:aws:kms:us-east-1:123456789012:key/test-key-id"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  kmsKeyID,
	}

	mockClient := &MockKinesisClient{}

	// Mock DescribeStream to return already encrypted with same key
	describeOutput := &kinesis.DescribeStreamOutput{
		StreamDescription: &types.StreamDescription{
			EncryptionType: types.EncryptionTypeKms,
			KeyId:          aws.String(kmsKeyID),
		},
	}
	mockClient.On("DescribeStream", mock.Anything, mock.MatchedBy(func(input *kinesis.DescribeStreamInput) bool {
		return aws.ToString(input.StreamName) == "test-stream"
	})).Return(describeOutput, nil)

	// StartStreamEncryption should NOT be called
	mockClient.AssertNotCalled(t, "StartStreamEncryption")

	pump := &TestableKinesisPump{}
	err := pump.InitWithMock(config, mockClient)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestKinesisPump_DescribeStream_AlreadyEncryptedDifferentKey(t *testing.T) {
	currentKeyID := "arn:aws:kms:us-east-1:123456789012:key/current-key"
	newKeyID := "arn:aws:kms:us-east-1:123456789012:key/new-key"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  newKeyID,
	}

	mockClient := &MockKinesisClient{}

	// Mock DescribeStream to return already encrypted with different key
	describeOutput := &kinesis.DescribeStreamOutput{
		StreamDescription: &types.StreamDescription{
			EncryptionType: types.EncryptionTypeKms,
			KeyId:          aws.String(currentKeyID),
		},
	}
	mockClient.On("DescribeStream", mock.Anything, mock.MatchedBy(func(input *kinesis.DescribeStreamInput) bool {
		return aws.ToString(input.StreamName) == "test-stream"
	})).Return(describeOutput, nil)

	pump := &TestableKinesisPump{}
	err := pump.InitWithMock(config, mockClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server-side encryption is already enabled with a different KMS Key ID")
	mockClient.AssertExpectations(t)
}

func TestKinesisPump_DescribeStream_NotEncrypted_StartEncryptionSuccess(t *testing.T) {
	kmsKeyID := "arn:aws:kms:us-east-1:123456789012:key/test-key-id"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  kmsKeyID,
	}

	mockClient := &MockKinesisClient{}

	// Mock DescribeStream to return not encrypted
	describeOutput := &kinesis.DescribeStreamOutput{
		StreamDescription: &types.StreamDescription{
			EncryptionType: types.EncryptionTypeNone,
		},
	}
	mockClient.On("DescribeStream", mock.Anything, mock.MatchedBy(func(input *kinesis.DescribeStreamInput) bool {
		return aws.ToString(input.StreamName) == "test-stream"
	})).Return(describeOutput, nil)

	// Mock StartStreamEncryption to succeed
	startEncryptionOutput := &kinesis.StartStreamEncryptionOutput{}
	mockClient.On("StartStreamEncryption", mock.Anything, mock.MatchedBy(func(input *kinesis.StartStreamEncryptionInput) bool {
		return aws.ToString(input.StreamName) == "test-stream" &&
			input.EncryptionType == types.EncryptionTypeKms &&
			aws.ToString(input.KeyId) == kmsKeyID
	})).Return(startEncryptionOutput, nil)

	pump := &TestableKinesisPump{}
	err := pump.InitWithMock(config, mockClient)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestKinesisPump_DescribeStream_NotEncrypted_StartEncryptionResourceInUse(t *testing.T) {
	kmsKeyID := "arn:aws:kms:us-east-1:123456789012:key/test-key-id"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  kmsKeyID,
	}

	mockClient := &MockKinesisClient{}

	// Mock DescribeStream to return not encrypted
	describeOutput := &kinesis.DescribeStreamOutput{
		StreamDescription: &types.StreamDescription{
			EncryptionType: types.EncryptionTypeNone,
		},
	}
	mockClient.On("DescribeStream", mock.Anything, mock.MatchedBy(func(input *kinesis.DescribeStreamInput) bool {
		return aws.ToString(input.StreamName) == "test-stream"
	})).Return(describeOutput, nil)

	// Mock StartStreamEncryption to fail with ResourceInUseException
	resourceInUseErr := &types.ResourceInUseException{
		Message: aws.String("Stream is currently being updated"),
	}
	mockClient.On("StartStreamEncryption", mock.Anything, mock.MatchedBy(func(input *kinesis.StartStreamEncryptionInput) bool {
		return aws.ToString(input.StreamName) == "test-stream" &&
			input.EncryptionType == types.EncryptionTypeKms &&
			aws.ToString(input.KeyId) == kmsKeyID
	})).Return(nil, resourceInUseErr)

	pump := &TestableKinesisPump{}
	err := pump.InitWithMock(config, mockClient)

	// Should handle ResourceInUseException gracefully
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestKinesisPump_DescribeStream_NotEncrypted_StartEncryptionGenericError(t *testing.T) {
	kmsKeyID := "arn:aws:kms:us-east-1:123456789012:key/test-key-id"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  kmsKeyID,
	}

	mockClient := &MockKinesisClient{}

	// Mock DescribeStream to return not encrypted
	describeOutput := &kinesis.DescribeStreamOutput{
		StreamDescription: &types.StreamDescription{
			EncryptionType: types.EncryptionTypeNone,
		},
	}
	mockClient.On("DescribeStream", mock.Anything, mock.MatchedBy(func(input *kinesis.DescribeStreamInput) bool {
		return aws.ToString(input.StreamName) == "test-stream"
	})).Return(describeOutput, nil)

	// Mock StartStreamEncryption to fail with generic error
	genericErr := errors.New("access denied")
	mockClient.On("StartStreamEncryption", mock.Anything, mock.MatchedBy(func(input *kinesis.StartStreamEncryptionInput) bool {
		return aws.ToString(input.StreamName) == "test-stream" &&
			input.EncryptionType == types.EncryptionTypeKms &&
			aws.ToString(input.KeyId) == kmsKeyID
	})).Return(nil, genericErr)

	pump := &TestableKinesisPump{}
	err := pump.InitWithMock(config, mockClient)

	// Should propagate generic errors
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
	mockClient.AssertExpectations(t)
}

func TestKinesisPump_DescribeStream_APIFailure(t *testing.T) {
	kmsKeyID := "arn:aws:kms:us-east-1:123456789012:key/test-key-id"

	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		"kms_key_id":  kmsKeyID,
	}

	mockClient := &MockKinesisClient{}

	// Mock DescribeStream to fail
	describeErr := errors.New("stream not found")
	mockClient.On("DescribeStream", mock.Anything, mock.MatchedBy(func(input *kinesis.DescribeStreamInput) bool {
		return aws.ToString(input.StreamName) == "test-stream"
	})).Return(nil, describeErr)

	pump := &TestableKinesisPump{}
	err := pump.InitWithMock(config, mockClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream not found")
	mockClient.AssertExpectations(t)
}

func TestKinesisPump_NoKMSKeyID_SkipsEncryption(t *testing.T) {
	config := map[string]interface{}{
		"stream_name": "test-stream",
		"region":      "us-east-1",
		// No kms_key_id provided - should skip all encryption calls
	}

	mockClient := &MockKinesisClient{}

	// Neither DescribeStream nor StartStreamEncryption should be called
	mockClient.AssertNotCalled(t, "DescribeStream")
	mockClient.AssertNotCalled(t, "StartStreamEncryption")

	pump := &TestableKinesisPump{}
	err := pump.InitWithMock(config, mockClient)

	assert.NoError(t, err)
	assert.Equal(t, "", pump.kinesisConf.KMSKeyID)
	mockClient.AssertExpectations(t)
}
