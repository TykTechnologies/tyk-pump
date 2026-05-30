package retry

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type tempErr struct{ msg string }

// Verifies: SW-REQ-030
func (e *tempErr) Error() string { return e.msg }

// Verifies: SW-REQ-030
func (e *tempErr) Temporary() bool { return true }

type timeoutErr2 struct{ msg string }

// Verifies: SW-REQ-030
func (e *timeoutErr2) Error() string { return e.msg }

// Verifies: SW-REQ-030
func (e *timeoutErr2) Timeout() bool { return true }

type connErr struct {
	msg string
	is  bool
}

// Verifies: SW-REQ-030
func (e *connErr) Error() string { return e.msg }

// Verifies: SW-REQ-030
func (e *connErr) ConnectionError() bool { return e.is }

// Verifies: SW-REQ-030
// SW-REQ-030:error_handling:negative
func TestIsErrorRetryable_AllBranches(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"connection-error iface true", &connErr{"conn", true}, true},
		{"connection-error iface false (fall through)", &connErr{"conn-false", false}, false},
		{"connection reset substring", errors.New("dial tcp: connection reset"), true},
		{"url error: connection refused", &url.Error{Op: "Get", URL: "x", Err: errors.New("connection refused")}, true},
		{"url error: permanent inner (Unwrap to plain)", &url.Error{Op: "Get", URL: "x", Err: errors.New("perm")}, false},
		{"net.OpError dial", &net.OpError{Op: "dial", Err: errors.New("x")}, true},
		{"net.OpError read non-temp (Unwrap to plain)", &net.OpError{Op: "read", Err: errors.New("perm")}, false},
		{"temporary error", &tempErr{"temp"}, true},
		{"timeout error", &timeoutErr2{"timeout"}, true},
		{"plain error", errors.New("permanent"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := isErrorRetryable(tc.err); got != tc.want {
			t.Fatalf("%s: isErrorRetryable=%v want %v", tc.name, got, tc.want)
		}
	}
}

// Verifies: SW-REQ-030
func TestBackoffHTTPRetry_Send_WithBody_RetriesOn5xx(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewBackoffRetry("test", 3, srv.Client(), testLogger())
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Send(req); err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected at least one retry on 5xx, saw %d call(s)", calls)
	}
}
