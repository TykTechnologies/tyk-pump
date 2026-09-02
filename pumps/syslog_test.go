package pumps

import (
	"context"
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

// createTestSyslogPumpWithTags is createTestSyslogPump with the tags field enabled.
func createTestSyslogPumpWithTags(addr string) *SyslogPump {
	pump := createTestSyslogPump(addr)
	pump.syslogConf.IncludeTags = true

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
}

// TestSyslogPump_MessageShape renders one deterministic record and logs the exact
// message plus its byte length, at a range of tag counts.
//
// It is a diagnostic for comparing message shape across changes: capture its output
// before a change, apply the change, capture again, and diff. The logged length is
// useful because a syslog message is a single UDP datagram by default and RFC 3164
// caps messages at 1024 bytes. It logs rather than asserting an exact string, so it
// needs no updating when the message shape intentionally changes.
func TestSyslogPump_MessageShape(t *testing.T) {
	for _, tc := range []struct {
		name        string
		nTags       int
		includeTags bool
	}{
		{"Default_NoTagsField", 10, false},
		{"Enabled_NoTags", 0, true},
		{"Enabled_Tags3", 3, true},
		{"Enabled_Tags5", 5, true},
		{"Enabled_Tags10", 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, messages := mockSyslogServer(t)

			s := createTestSyslogPump(addr)
			if tc.includeTags {
				s = createTestSyslogPumpWithTags(addr)
			}

			rec := shapeRecord(tc.nTags)
			require.NoError(t, s.WriteData(context.Background(), []interface{}{rec}))

			select {
			case msg := <-messages:
				start := strings.Index(msg, "map[")
				require.True(t, start >= 0, "no map in message: %s", msg)
				rendered := strings.TrimSpace(msg[start:])

				t.Logf("SHAPE %s len=%d %s", tc.name, len(rendered), rendered)
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for syslog message")
			}
		})
	}
}

// shapeRecord mirrors benchRecord but is available to the test suite; fixed
// timestamp keeps the rendered output stable run to run.
func shapeRecord(nTags int) analytics.AnalyticsRecord {
	tags := []string{
		"key-test-api-key-aaaaaaaaaaaaaaaaaaaaaaa",
		"org-5e9d9544a1dcd60001d0ed20",
		"api-b84fe1a04e5648927971c0557971565c",
		"pol-6a1b2c3d4e5f60718293a4b5",
		"dev-9f8e7d6c5b4a39281706f5e4",
		"cached-response",
		"tier-gold",
		"region-emea",
		"team-payments",
		"env-production",
	}
	if nTags > len(tags) {
		nTags = len(tags)
	}
	var t []string
	if nTags > 0 {
		t = tags[:nTags]
	}

	return analytics.AnalyticsRecord{
		Method:        "POST",
		Host:          "api.example.com",
		Path:          "/api/v1/orders/12345",
		RawPath:       "/api/v1/orders/12345?expand=items",
		ContentLength: 512,
		UserAgent:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		ResponseCode:  200,
		APIKey:        "test-api-key-aaaaaaaaaaaaaaaaaaaaaaa",
		TimeStamp:     time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC),
		APIVersion:    "v1",
		APIName:       "Orders API",
		APIID:         "b84fe1a04e5648927971c0557971565c",
		OrgID:         "5e9d9544a1dcd60001d0ed20",
		RequestTime:   42,
		IPAddress:     "10.42.7.19",
		Tags:          t,
	}
}

// TestSyslogPump_WriteData_Tags covers the tags field: present for every record,
// rendered empty when the record carries none, and never breaking the single-line
// guarantee regardless of tag content.
func TestSyslogPump_WriteData_Tags(t *testing.T) {
	//nolint:govet // field alignment is irrelevant for a test table
	tests := []struct {
		name        string
		tags        []string
		wantContain string
	}{
		{
			name:        "multiple tags",
			tags:        []string{"key-abc123", "org-5e9d", "api-42"},
			wantContain: "tags:[key-abc123 org-5e9d api-42]",
		},
		{
			name:        "single tag",
			tags:        []string{"api-42"},
			wantContain: "tags:[api-42]",
		},
		{
			name:        "nil tags renders empty, never absent",
			tags:        nil,
			wantContain: "tags:[]",
		},
		{
			name:        "empty slice renders empty, never absent",
			tags:        []string{},
			wantContain: "tags:[]",
		},
		{
			name:        "tags containing spaces and colons",
			tags:        []string{"has space", "a:b", "trailing "},
			wantContain: "tags:[has space a:b trailing ]",
		},
		{
			// Tags are user-supplied. An unescaped newline would split the record
			// across two syslog lines and be read downstream as two records.
			name:        "newline in a tag is escaped, not emitted raw",
			tags:        []string{"bad\ntag", "ok"},
			wantContain: `tags:[bad\ntag ok]`,
		},
		{
			// Only \n is escaped, matching raw_request/raw_response. A tab passes
			// through literally; a lone \r does not start a new syslog line.
			name:        "tab and carriage return pass through without fragmenting",
			tags:        []string{"a\tb", "c\r\nd"},
			wantContain: "tags:[a\tb c\r\\nd]",
		},
		{
			name:        "unicode and emoji in tags",
			tags:        []string{"région-emea", "team-🚀"},
			wantContain: "tags:[région-emea team-🚀]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, messages := mockSyslogServer(t)
			s := createTestSyslogPumpWithTags(addr)

			record := analytics.AnalyticsRecord{
				Method:       "GET",
				Path:         "/api/test",
				ResponseCode: 200,
				TimeStamp:    time.Now(),
				Tags:         tt.tags,
			}

			require.NoError(t, s.WriteData(context.Background(), []interface{}{record}))

			select {
			case msg := <-messages:
				assert.Contains(t, msg, tt.wantContain)

				// Awkward tag content must not fragment the message across lines —
				// a multi-line syslog message is read as several separate records.
				var nonEmpty int
				for _, line := range strings.Split(msg, "\n") {
					if strings.TrimSpace(line) != "" {
						nonEmpty++
					}
				}
				assert.Equal(t, 1, nonEmpty, "message must stay on one line, got %d: %s", nonEmpty, msg)
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for syslog message")
			}
		})
	}
}

// TestSyslogPump_WriteData_PreservesExistingFields is the regression guard: adding a
// field must not rename, drop or alter any field the pump already emitted.
func TestSyslogPump_WriteData_PreservesExistingFields(t *testing.T) {
	existingFields := []string{
		"timestamp:", "method:", "path:", "raw_path:", "response_code:",
		"alias:", "api_key:", "api_version:", "api_name:", "api_id:",
		"org_id:", "oauth_id:", "raw_request:", "request_time_ms:",
		"raw_response:", "ip_address:", "host:", "content_length:", "user_agent:",
	}

	for _, tc := range []struct {
		name        string
		includeTags bool
		wantTags    bool
	}{
		{"tags disabled (default)", false, false},
		{"tags enabled", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, messages := mockSyslogServer(t)

			s := createTestSyslogPump(addr)
			if tc.includeTags {
				s = createTestSyslogPumpWithTags(addr)
			}

			require.NoError(t, s.WriteData(context.Background(), []interface{}{shapeRecord(3)}))

			select {
			case msg := <-messages:
				for _, field := range existingFields {
					assert.Contains(t, msg, field, "pre-existing field %q missing", field)
				}

				if tc.wantTags {
					assert.Contains(t, msg, "tags:", "tags missing when enabled")
				} else {
					assert.NotContains(t, msg, "tags:", "tags emitted while disabled")
				}

				// Values carried through unchanged, not just the keys.
				assert.Contains(t, msg, "method:POST")
				assert.Contains(t, msg, "api_name:Orders API")
				assert.Contains(t, msg, "ip_address:10.42.7.19")
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for syslog message")
			}
		})
	}
}

// TestSyslogPump_IncludeTags_EnvVar confirms the option is settable through the
// pump's environment-variable override path, not only through config.
func TestSyslogPump_IncludeTags_EnvVar(t *testing.T) {
	t.Setenv("TYK_PMP_PUMPS_SYSLOG_META_INCLUDETAGS", "true")

	addr, messages := mockSyslogServer(t)
	s := createTestSyslogPump(addr)

	processPumpEnvVars(s, s.log, s.syslogConf, syslogDefaultENV)
	require.True(t, s.syslogConf.IncludeTags, "env var did not set IncludeTags")

	require.NoError(t, s.WriteData(context.Background(), []interface{}{shapeRecord(3)}))

	select {
	case msg := <-messages:
		assert.Contains(t, msg, "tags:")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for syslog message")
	}
}

// TestSyslogPump_Init_IncludeTags exercises the real configuration path: Init decodes
// the pump's meta block with mapstructure, so this is what verifies the
// `include_tags` config key actually maps to the field. Tests that set the struct
// field directly bypass the decode and would pass even if the tag were misspelled.
func TestSyslogPump_Init_IncludeTags(t *testing.T) {
	addr, _ := mockSyslogServer(t)

	//nolint:govet // field alignment is irrelevant for a test table
	tests := []struct {
		name string
		meta map[string]interface{}
		want bool
	}{
		{
			name: "omitted defaults to false",
			meta: map[string]interface{}{},
			want: false,
		},
		{
			name: "explicitly false",
			meta: map[string]interface{}{"include_tags": false},
			want: false,
		},
		{
			name: "explicitly true",
			meta: map[string]interface{}{"include_tags": true},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := map[string]interface{}{
				"transport":    "udp",
				"network_addr": addr,
				"log_level":    6,
				"tag":          "test",
			}
			for k, v := range tt.meta {
				meta[k] = v
			}

			pump := &SyslogPump{}
			require.NoError(t, pump.Init(meta))
			assert.Equal(t, tt.want, pump.syslogConf.IncludeTags)
		})
	}
}

// TestSyslogPump_Init_IncludeTags_EndToEnd confirms a record written by a pump built
// from configuration alone carries tags only when the config asked for it.
func TestSyslogPump_Init_IncludeTags_EndToEnd(t *testing.T) {
	//nolint:govet // field alignment is irrelevant for a test table
	for _, tt := range []struct {
		name        string
		includeTags bool
	}{
		{"disabled by config", false},
		{"enabled by config", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			addr, messages := mockSyslogServer(t)

			pump := &SyslogPump{}
			require.NoError(t, pump.Init(map[string]interface{}{
				"transport":    "udp",
				"network_addr": addr,
				"log_level":    6,
				"tag":          "test",
				"include_tags": tt.includeTags,
			}))

			require.NoError(t, pump.WriteData(context.Background(), []interface{}{shapeRecord(3)}))

			select {
			case msg := <-messages:
				if tt.includeTags {
					assert.Contains(t, msg, "tags:")
				} else {
					assert.NotContains(t, msg, "tags:")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for syslog message")
			}
		})
	}
}

// TestSyslogPump_IncludeTags_EnvVarOverridesConfig covers the precedence the pump's
// env-var handling implies: an environment variable wins over the config file value.
func TestSyslogPump_IncludeTags_EnvVarOverridesConfig(t *testing.T) {
	addr, _ := mockSyslogServer(t)

	t.Run("env true overrides config false", func(t *testing.T) {
		t.Setenv("TYK_PMP_PUMPS_SYSLOG_META_INCLUDETAGS", "true")

		pump := &SyslogPump{}
		require.NoError(t, pump.Init(map[string]interface{}{
			"transport":    "udp",
			"network_addr": addr,
			"log_level":    6,
			"include_tags": false,
		}))
		assert.True(t, pump.syslogConf.IncludeTags, "env var should override config")
	})

	t.Run("env false with config true", func(t *testing.T) {
		t.Setenv("TYK_PMP_PUMPS_SYSLOG_META_INCLUDETAGS", "false")

		pump := &SyslogPump{}
		require.NoError(t, pump.Init(map[string]interface{}{
			"transport":    "udp",
			"network_addr": addr,
			"log_level":    6,
			"include_tags": true,
		}))
		assert.False(t, pump.syslogConf.IncludeTags, "env var should override config")
	})
}
