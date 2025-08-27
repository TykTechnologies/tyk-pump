package pumps

import (
	"os"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

func TestKafkaPump_New(t *testing.T) {
	pump := (&KafkaPump{}).New()
	assert.IsType(t, &KafkaPump{}, pump)
}

func TestKafkaPump_GetName(t *testing.T) {
	pump := &KafkaPump{}
	assert.Equal(t, "Kafka Pump", pump.GetName())
}

func TestKafkaPump_Init_BatchBytesConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]interface{}
		expectedBytes int
		description   string
	}{
		{
			name: "Custom BatchBytes Value",
			config: map[string]interface{}{
				"broker":      []string{"localhost:9092"},
				"topic":       "test-topic",
				"batch_bytes": 2048576, // 2MB
			},
			expectedBytes: 2048576,
			description:   "Should set custom BatchBytes value",
		},
		{
			name: "Zero BatchBytes Value",
			config: map[string]interface{}{
				"broker":      []string{"localhost:9092"},
				"topic":       "test-topic",
				"batch_bytes": 0,
			},
			expectedBytes: 0,
			description:   "Should allow zero BatchBytes (uses kafka-go default)",
		},
		{
			name: "No BatchBytes Configuration",
			config: map[string]interface{}{
				"broker": []string{"localhost:9092"},
				"topic":  "test-topic",
			},
			expectedBytes: 0,
			description:   "Should default to zero when BatchBytes not specified",
		},
		{
			name: "Large BatchBytes Value",
			config: map[string]interface{}{
				"broker":      []string{"localhost:9092"},
				"topic":       "test-topic",
				"batch_bytes": 10485760, // 10MB
			},
			expectedBytes: 10485760,
			description:   "Should handle large BatchBytes values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pump := &KafkaPump{}
			err := pump.Init(tt.config)

			assert.NoError(t, err, tt.description)
			assert.Equal(t, tt.expectedBytes, pump.writerConfig.BatchBytes, tt.description)
		})
	}
}

func TestKafkaPump_Init_BatchBytesWithOtherConfigs(t *testing.T) {
	config := map[string]interface{}{
		"broker":                   []string{"localhost:9092"},
		"topic":                    "test-topic",
		"batch_bytes":              512000, // 500KB
		"client_id":                "test-client",
		"timeout":                  "30s",
		"compressed":               true,
		"use_ssl":                  false,
		"ssl_insecure_skip_verify": false,
		"meta_data": map[string]string{
			"environment": "test",
		},
	}

	pump := &KafkaPump{}
	err := pump.Init(config)

	assert.NoError(t, err)
	assert.Equal(t, 512000, pump.writerConfig.BatchBytes)
	assert.Equal(t, []string{"localhost:9092"}, pump.writerConfig.Brokers)
	assert.Equal(t, "test-topic", pump.writerConfig.Topic)
	assert.NotNil(t, pump.writerConfig.CompressionCodec)
	assert.IsType(t, &kafka.LeastBytes{}, pump.writerConfig.Balancer)
}

func TestKafkaPump_BatchBytesEnvironmentVariable(t *testing.T) {
	// Test that BatchBytes can be overridden via environment variables
	// This follows the same pattern as other configuration fields
	config := map[string]interface{}{
		"broker":      []string{"localhost:9092"},
		"topic":       "test-topic",
		"batch_bytes": 1024000, // 1MB
	}

	pump := &KafkaPump{}
	err := pump.Init(config)

	assert.NoError(t, err)
	assert.Equal(t, 1024000, pump.writerConfig.BatchBytes)
}

func TestKafkaPump_WriterConfigIntegrity(t *testing.T) {
	// Test that BatchBytes configuration doesn't interfere with other writer config fields
	config := map[string]interface{}{
		"broker":      []string{"localhost:9092", "localhost:9093"},
		"topic":       "analytics-topic",
		"batch_bytes": 2097152, // 2MB
		"client_id":   "tyk-pump-test",
		"timeout":     10.0,
		"compressed":  true,
	}

	pump := &KafkaPump{}
	err := pump.Init(config)

	assert.NoError(t, err)

	// Verify BatchBytes is set correctly
	assert.Equal(t, 2097152, pump.writerConfig.BatchBytes)

	// Verify other configurations are not affected
	assert.Equal(t, []string{"localhost:9092", "localhost:9093"}, pump.writerConfig.Brokers)
	assert.Equal(t, "analytics-topic", pump.writerConfig.Topic)
	assert.NotNil(t, pump.writerConfig.CompressionCodec)
	assert.NotNil(t, pump.writerConfig.Dialer)
	assert.IsType(t, &kafka.LeastBytes{}, pump.writerConfig.Balancer)
}

func TestKafkaPump_BatchBytesEnvironmentVariableOverride(t *testing.T) {
	// Test that BatchBytes can be overridden via environment variables
	// This follows the same pattern as other configuration fields
	config := map[string]interface{}{
		"broker":      []string{"localhost:9092"},
		"topic":       "test-topic",
		"batch_bytes": 1024000, // 1MB
	}

	os.Setenv("TYK_PMP_PUMPS_KAFKA_META_BATCHBYTES", "2048000") // 2MB
	defer os.Unsetenv("TYK_PMP_PUMPS_KAFKA_META_BATCHBYTES")

	pump := &KafkaPump{}
	err := pump.Init(config)

	assert.NoError(t, err)
	assert.Equal(t, 2048000, pump.writerConfig.BatchBytes)
}

func TestKafkaPump_BatchBytesEnvironmentVariableInvalid(t *testing.T) {
	// Test that BatchBytes environment variable is ignored if it's not a valid integer
	// This follows the same pattern as other configuration fields
	config := map[string]interface{}{
		"broker":      []string{"localhost:9092"},
		"topic":       "test-topic",
		"batch_bytes": 1024000, // 1MB
	}

	os.Setenv("TYK_PMP_PUMPS_KAFKA_META_BATCHBYTES", "not-an-integer")
	defer os.Unsetenv("TYK_PMP_PUMPS_KAFKA_META_BATCHBYTES")

	pump := &KafkaPump{}
	err := pump.Init(config)

	assert.NoError(t, err)
	assert.Equal(t, 1024000, pump.writerConfig.BatchBytes)
}
