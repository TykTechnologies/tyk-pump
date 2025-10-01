package pumps

import (
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

func TestKinesisPump_StaticCredentials_ConfigurationParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected KinesisConf
	}{
		{
			name: "Complete static credentials",
			input: map[string]interface{}{
				"stream_name":       "test-stream",
				"region":            "us-east-1",
				"access_key_id":     "AKIATEST123",
				"secret_access_key": "secretkey123",
				"session_token":     "sessiontoken123",
				"batch_size":        100,
			},
			expected: KinesisConf{
				StreamName:      "test-stream",
				Region:          "us-east-1",
				AccessKeyID:     "AKIATEST123",
				SecretAccessKey: "secretkey123",
				SessionToken:    "sessiontoken123",
				BatchSize:       100,
			},
		},
		{
			name: "Static credentials without session token",
			input: map[string]interface{}{
				"stream_name":       "test-stream",
				"region":            "us-east-1",
				"access_key_id":     "AKIATEST123",
				"secret_access_key": "secretkey123",
			},
			expected: KinesisConf{
				StreamName:      "test-stream",
				Region:          "us-east-1",
				AccessKeyID:     "AKIATEST123",
				SecretAccessKey: "secretkey123",
				SessionToken:    "",
			},
		},
		{
			name: "No static credentials (default chain)",
			input: map[string]interface{}{
				"stream_name": "test-stream",
				"region":      "us-west-2",
			},
			expected: KinesisConf{
				StreamName:      "test-stream",
				Region:          "us-west-2",
				AccessKeyID:     "",
				SecretAccessKey: "",
				SessionToken:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var conf KinesisConf
			err := mapstructure.Decode(tt.input, &conf)
			assert.NoError(t, err)

			assert.Equal(t, tt.expected.AccessKeyID, conf.AccessKeyID)
			assert.Equal(t, tt.expected.SecretAccessKey, conf.SecretAccessKey)
			assert.Equal(t, tt.expected.SessionToken, conf.SessionToken)
		})
	}
}

func TestKinesisPump_StaticCredentials_Logic(t *testing.T) {
	t.Run("Should use static credentials when both keys provided", func(t *testing.T) {
		config := map[string]interface{}{
			"stream_name":       "test-stream",
			"region":            "us-east-1",
			"access_key_id":     "AKIATEST123",
			"secret_access_key": "secretkey123",
		}

		pump := &KinesisPump{}
		pump.Init(config)

		if pump.kinesisConf != nil {
			hasStaticCreds := pump.kinesisConf.AccessKeyID != "" && pump.kinesisConf.SecretAccessKey != ""
			assert.True(t, hasStaticCreds, "Should have static credentials")
		}
	})

	t.Run("Should fall back to default chain when incomplete", func(t *testing.T) {
		config := map[string]interface{}{
			"stream_name":   "test-stream",
			"region":        "us-east-1",
			"access_key_id": "AKIATEST123",
		}

		pump := &KinesisPump{}
		pump.Init(config)

		if pump.kinesisConf != nil {
			hasStaticCreds := pump.kinesisConf.AccessKeyID != "" && pump.kinesisConf.SecretAccessKey != ""
			assert.False(t, hasStaticCreds, "Should use default credential chain")
		}
	})
}
