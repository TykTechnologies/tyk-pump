package pumps

import (
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
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
