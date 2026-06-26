package retry

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type tempErr struct{ msg string }

func (e *tempErr) Error() string { return e.msg }

func (e *tempErr) Temporary() bool { return true }

type timeoutErr2 struct{ msg string }

func (e *timeoutErr2) Error() string { return e.msg }

func (e *timeoutErr2) Timeout() bool { return true }

type connErr struct {
	msg string
	is  bool
}

func (e *connErr) Error() string { return e.msg }

func (e *connErr) ConnectionError() bool { return e.is }

// SW-REQ-030:retry_policy_explicit:nominal
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

// Verifies: SYS-REQ-006
// MCDC SYS-REQ-006: retry_attempted=T, transient_failure=T => TRUE
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

// SW-REQ-030:request_body_replay_preserved:nominal
func TestBackoffHTTPRetry_Send_ReplaysRequestBodyOnRetry(t *testing.T) {
	const payload = `{"event":"splunk-retry"}`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("attempt %d: read body: %v", calls, err)
		}
		if got := string(body); got != payload {
			t.Fatalf("attempt %d body = %q, want %q", calls, got, payload)
		}
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewBackoffRetry("test", 1, srv.Client(), testLogger())
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Send(req); err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected initial attempt plus one retry, got %d attempts", calls)
	}
}

// Verifies: SYS-REQ-023
// Verifies: SYS-REQ-006
// SYS-REQ-023:error_handling:negative
// MCDC SYS-REQ-023: error_surfaced_to_caller=F, retry_attempts_exhausted=F => TRUE
// MCDC SYS-REQ-023: error_surfaced_to_caller=F, retry_attempts_exhausted=T => FALSE
// MCDC SYS-REQ-023: error_surfaced_to_caller=T, retry_attempts_exhausted=T => TRUE
// MCDC SYS-REQ-006: retry_attempted=F, transient_failure=F => TRUE
// MCDC SYS-REQ-006: retry_attempted=F, transient_failure=T => FALSE
// MCDC SYS-REQ-006: retry_attempted=T, transient_failure=T => TRUE
//
// 5xx-every-time forces transient_failure=T and the maxRetries cap is hit
// (retry_attempts_exhausted=T). The err==nil assertion fails iff the error wasn't
// surfaced -> proves error_surfaced_to_caller=T -> TRUE row. The calls=maxRetries+1
// assertion proves retry_attempted=T -> TRUE row for SYS-REQ-006 too. The FALSE row
// (exhausted but no error returned) is exactly what the err==nil assertion blocks.
// The vacuous TRUE arm corresponds to the success path (TestBackoffHTTPRetry_Send_Success).
func TestBackoffHTTPRetry_Send_ErrorSurfacedAfterRetriesExhausted(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError) // 5xx every time
	}))
	defer srv.Close()

	const maxRetries = 2
	r := NewBackoffRetry("test", maxRetries, srv.Client(), testLogger())
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err := r.Send(req); err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if calls != int(maxRetries)+1 {
		t.Fatalf("expected %d total attempts (initial + %d retries), got %d", maxRetries+1, maxRetries, calls)
	}
}
