package pumps

import (
	"context"
<<<<<<< HEAD
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

func TestSyslogPump_Init(t *testing.T) {
	pump := &SyslogPump{}
	
	// Test with default configuration
	config := map[string]interface{}{
		"transport":     "udp",
		"network_addr":  "localhost:5140",
		"log_level":     6,
		"tag":           "test-tag",
	}
	
	err := pump.Init(config)
	require.NoError(t, err)
	
	assert.Equal(t, "udp", pump.syslogConf.Transport)
	assert.Equal(t, "localhost:5140", pump.syslogConf.NetworkAddr)
	assert.Equal(t, 6, pump.syslogConf.LogLevel)
	assert.Equal(t, "test-tag", pump.syslogConf.Tag)
	assert.False(t, pump.syslogConf.SyslogFragmentation) // Should default to false
}

func TestSyslogPump_InitWithFragmentation(t *testing.T) {
	pump := &SyslogPump{}
	
	// Test with syslog_fragmentation enabled
	config := map[string]interface{}{
		"transport":             "udp",
		"network_addr":          "localhost:5140",
		"log_level":             6,
		"syslog_fragmentation":  true,
	}
	
	err := pump.Init(config)
	require.NoError(t, err)
	
	assert.True(t, pump.syslogConf.SyslogFragmentation)
}

func TestSyslogPump_GetName(t *testing.T) {
	pump := &SyslogPump{}
	assert.Equal(t, "Syslog Pump", pump.GetName())
}

func TestSyslogPump_New(t *testing.T) {
	pump := &SyslogPump{}
	newPump := pump.New()
	
	assert.IsType(t, &SyslogPump{}, newPump)
}

func TestSyslogPump_GetEnvPrefix(t *testing.T) {
	pump := &SyslogPump{
		syslogConf: &SyslogConf{
			EnvPrefix: "TEST_PREFIX",
		},
	}
	
	assert.Equal(t, "TEST_PREFIX", pump.GetEnvPrefix())
}

func TestSyslogPump_SetAndGetTimeout(t *testing.T) {
	pump := &SyslogPump{}
	
	pump.SetTimeout(30)
	assert.Equal(t, 30, pump.GetTimeout())
}

func TestSyslogPump_SetAndGetFilters(t *testing.T) {
	pump := &SyslogPump{}
	filters := analytics.AnalyticsFilters{
		APIIDs: []string{"api1", "api2"},
	}
	
	pump.SetFilters(filters)
	assert.Equal(t, filters, pump.GetFilters())
=======
	"net"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSyslogServer creates a simple UDP syslog server for testing
func mockSyslogServer(t *testing.T) (string, chan string) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)

	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)

	messages := make(chan string, 100)

	go func() {
		defer conn.Close()
		buffer := make([]byte, 1024)
		for {
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				return
			}
			messages <- string(buffer[:n])
		}
	}()

	return conn.LocalAddr().String(), messages
}

// Helper function to create a SyslogPump with test configuration
func createTestSyslogPump(addr string) *SyslogPump {
	pump := &SyslogPump{
		syslogConf: &SyslogConf{
			Transport:   "udp",
			NetworkAddr: addr,
			LogLevel:    6, // Info level
			Tag:         "test",
		},
		CommonPumpConfig: CommonPumpConfig{
			log: log.WithField("prefix", "test"),
		},
	}

	// Initialize the writer
	pump.initWriter()
	return pump
}

func TestSyslogPump_WriteData(t *testing.T) {
	tests := []struct {
		name     string
		data     []interface{}
		wantLogs int // expected number of log entries
	}{
		{
			name: "Single valid record",
			data: []interface{}{
				analytics.AnalyticsRecord{
					Method:       "GET",
					Path:         "/api/test",
					ResponseCode: 200,
					TimeStamp:    time.Now(),
				},
			},
			wantLogs: 1,
		},
		{
			name: "Multiple valid records",
			data: []interface{}{
				analytics.AnalyticsRecord{
					Method:       "GET",
					Path:         "/api/test1",
					ResponseCode: 200,
					TimeStamp:    time.Now(),
				},
				analytics.AnalyticsRecord{
					Method:       "POST",
					Path:         "/api/test2",
					ResponseCode: 201,
					TimeStamp:    time.Now(),
				},
			},
			wantLogs: 2,
		},
		{
			name:     "Empty data slice",
			data:     []interface{}{},
			wantLogs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock syslog server
			addr, messages := mockSyslogServer(t)

			// Create syslog pump with test configuration
			s := createTestSyslogPump(addr)

			// Call WriteData
			err := s.WriteData(context.Background(), tt.data)
			assert.NoError(t, err)

			if tt.wantLogs == 0 {
				// Give a small amount of time for any potential messages
				select {
				case <-messages:
					t.Error("Expected no messages but received one")
				case <-time.After(100 * time.Millisecond):
					// Good, no messages received
				}
				return
			}

			// Collect messages
			var receivedMessages []string
			timeout := time.After(2 * time.Second)
			for len(receivedMessages) < tt.wantLogs {
				select {
				case msg := <-messages:
					receivedMessages = append(receivedMessages, msg)
				case <-timeout:
					break
				}
			}

			assert.Equal(t, tt.wantLogs, len(receivedMessages), "Expected %d log entries, got %d", tt.wantLogs, len(receivedMessages))

			// Verify each message contains the original map format
			for i, msg := range receivedMessages {
				// Syslog messages have a header, extract the map part
				// Look for the map starting with 'map['
				mapStart := strings.Index(msg, "map[")
				require.True(t, mapStart >= 0, "Message should contain map format: %s", msg)
				mapPart := strings.TrimSpace(msg[mapStart:])

				// Verify it's the expected map format
				assert.True(t, strings.HasPrefix(mapPart, "map["), "Log entry %d should start with 'map[': %s", i, mapPart)
				assert.True(t, strings.HasSuffix(mapPart, "]"), "Log entry %d should end with ']': %s", i, mapPart)
			}
		})
	}
}

func TestSyslogPump_WriteData_WithMultilineHTTP(t *testing.T) {
	// Test data with realistic multiline HTTP requests/responses that would cause fragmentation
	record := analytics.AnalyticsRecord{
		Method:       "POST",
		Path:         "/api/users",
		ResponseCode: 201,
		TimeStamp:    time.Now(),
		// Real HTTP request with headers and body
		RawRequest: `POST /api/users HTTP/1.1
Host: api.example.com
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36
Content-Type: application/json
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9
Content-Length: 67

{
  "name": "John Doe",
  "email": "john@example.com",
  "age": 30
}`,
		// Real HTTP response with headers and body
		RawResponse: `HTTP/1.1 201 Created
Date: Wed, 15 Aug 2024 10:30:00 GMT
Content-Type: application/json
Server: nginx/1.18.0
Content-Length: 156

{
  "id": 12345,
  "name": "John Doe",
  "email": "john@example.com",
  "age": 30,
  "created_at": "2024-08-15T10:30:00Z"
}`,
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}

	// Create mock syslog server
	addr, messages := mockSyslogServer(t)

	// Create syslog pump with test configuration
	s := createTestSyslogPump(addr)

	err := s.WriteData(context.Background(), []interface{}{record})
	assert.NoError(t, err)

	// Wait for message
	select {
	case msg := <-messages:
		// Extract map from syslog message
		// Look for the map starting with 'map['
		mapStart := strings.Index(msg, "map[")
		require.True(t, mapStart >= 0, "Message should contain map format: %s", msg)
		mapPart := strings.TrimSpace(msg[mapStart:])

		// Verify the syslog message itself is a single line (no fragmentation)
		lines := strings.Split(msg, "\n")
		nonEmptyLines := []string{}
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				nonEmptyLines = append(nonEmptyLines, line)
			}
		}
		assert.Equal(t, 1, len(nonEmptyLines), "Syslog message should be a single line, got %d lines", len(nonEmptyLines))

		// Verify it's the expected map format
		assert.True(t, strings.HasPrefix(mapPart, "map["), "Should be map format: %s", mapPart)
		// Note: May be truncated due to UDP packet size limits, so don't require ending with "]"

		// Verify newlines are properly escaped (should appear as \n not actual newlines)
		assert.Contains(t, mapPart, "\\n", "Newlines should be escaped in map output")

		// The key test: ensure the syslog message itself doesn't contain raw newlines that would cause fragmentation
		// We check this by ensuring the raw multiline content appears escaped in the single-line syslog message
		assert.Contains(t, mapPart, "raw_request:POST /api/users HTTP/1.1\\n", "Should contain escaped newlines in raw_request")

		// Verify the original multiline content is present but escaped
		assert.Contains(t, mapPart, "raw_request:", "Should contain raw_request field")
		assert.Contains(t, mapPart, "raw_response:", "Should contain raw_response field")
		assert.Contains(t, mapPart, "POST /api/users HTTP/1.1", "Should contain HTTP request line")
		assert.Contains(t, mapPart, "HTTP/1.1 201 Created", "Should contain HTTP status line")

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for syslog message")
	}
}

func TestSyslogPump_WriteData_SpecialCharacters(t *testing.T) {
	// Test data with special characters that could break output
	record := analytics.AnalyticsRecord{
		Method:       "POST",
		Path:         "/api/test",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
		RawRequest:   `{"message": "Hello \"World\"", "data": "line1\nline2\ttab\rcarriage"}`,
		RawResponse:  "Response with unicode: 测试 and emoji: 🚀",
		UserAgent:    "Agent/1.0 (Compatible; Special chars: []{}();)",
		APIKey:       "key_with_quotes_\"and\"_backslashes\\",
	}

	// Create mock syslog server
	addr, messages := mockSyslogServer(t)

	// Create syslog pump with test configuration
	s := createTestSyslogPump(addr)

	err := s.WriteData(context.Background(), []interface{}{record})
	assert.NoError(t, err)

	// Wait for message
	select {
	case msg := <-messages:
		// Extract map from syslog message
		mapStart := strings.Index(msg, "map[")
		require.True(t, mapStart >= 0, "Message should contain map format: %s", msg)
		mapPart := strings.TrimSpace(msg[mapStart:])

		// Verify special characters and unicode are handled properly
		assert.Contains(t, mapPart, "raw_request:", "Should contain raw_request field")
		assert.Contains(t, mapPart, "raw_response:", "Should contain raw_response field")
		assert.Contains(t, mapPart, "测试", "Should preserve unicode characters")
		assert.Contains(t, mapPart, "🚀", "Should preserve emoji")

		// Verify newlines are escaped in the raw_request field
		assert.Contains(t, mapPart, "\\n", "Newlines should be escaped")

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for syslog message")
	}
}

func TestSyslogPump_WriteData_ContextCancellation(t *testing.T) {
	record := analytics.AnalyticsRecord{
		Method:       "GET",
		Path:         "/api/test",
		ResponseCode: 200,
		TimeStamp:    time.Now(),
	}

	// Create mock syslog server
	addr, messages := mockSyslogServer(t)

	// Create syslog pump with test configuration
	s := createTestSyslogPump(addr)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.WriteData(ctx, []interface{}{record})
	assert.NoError(t, err) // Should not error, just return early

	// Should not have written anything due to context cancellation
	select {
	case <-messages:
		t.Error("Should not receive any messages when context is cancelled")
	case <-time.After(100 * time.Millisecond):
		// Good, no messages received
	}
>>>>>>> 0596e82... [TT-15532] Alternative backward-compatible fix for syslog pump log fragmentation (#886)
}
