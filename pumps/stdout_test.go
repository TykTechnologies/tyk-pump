package pumps

import "testing"

func TestRemoveWhitespaces(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no whitespaces",
			input:    "helloworld",
			expected: "helloworld",
		},
		{
			name:     "newlines replaced with spaces",
			input:    "hello\nworld\n",
			expected: "hello world ",
		},
		{
			name:     "carriage returns removed",
			input:    "hello\rworld\r",
			expected: "helloworld",
		},
		{
			name:     "tabs removed",
			input:    "hello\tworld\t",
			expected: "helloworld",
		},
		{
			name:     "mixed whitespaces",
			input:    "hello\r\t\nworld\n",
			expected: "hello world ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeWhitespaces(tt.input)
			if result != tt.expected {
				t.Errorf("removeWhitespaces() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTransformHTTPPayload(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no body, only headers",
			input:    "GET / HTTP/1.1\r\nHost: example.com\r\nAccept: */*",
			expected: "GET / HTTP/1.1 Host: example.com Accept: */*",
		},
		{
			name:     "valid JSON body",
			input:    "POST / HTTP/1.1\r\nHost: example.com\r\n\r\n{	\"key\": \"value\",\n	\"num\": 123}",
			expected: "POST / HTTP/1.1 Host: example.com {\"key\":\"value\",\"num\":123}",
		},
		{
			name:     "valid JSON array body",
			input:    "POST / HTTP/1.1\r\nHost: example.com\r\n\r\n[\n1,\n2,\n3\n]",
			expected: "POST / HTTP/1.1 Host: example.com [1,2,3]",
		},
		{
			name:     "invalid JSON body (fallback to removeWhitespaces)",
			input:    "POST / HTTP/1.1\r\nHost: example.com\r\n\r\n{\n\n\n\"key\": \"value\"\n",
			expected: "POST / HTTP/1.1 Host: example.com  {   \"key\": \"value\" ",
		},
		{
			name:     "plain text body (fallback to removeWhitespaces)",
			input:    "POST / HTTP/1.1\r\nHost: example.com\r\n\r\nHello\r\nWorld\t!",
			expected: "POST / HTTP/1.1 Host: example.com  Hello World!",
		},
		{
			name:     "multiple CRLF before valid JSON",
			input:    "POST / HTTP/1.1\r\n\r\n\r\n{\n\"key\": \"value\"\n}",
			expected: "POST / HTTP/1.1 {\"key\":\"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformHTTPPayload(tt.input)
			if result != tt.expected {
				t.Errorf("transformHTTPPayload() = %q, want %q", result, tt.expected)
			}
		})
	}
}
