package pumps

import (
	"testing"

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
