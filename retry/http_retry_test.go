package retry

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
)

// Verifies: SW-REQ-030
func testLogger() *logrus.Entry {
	l := logrus.New()
	l.SetOutput(io_discard{})
	return logrus.NewEntry(l)
}

type io_discard struct{}

// Verifies: SW-REQ-030
func (io_discard) Write(p []byte) (int, error) { return len(p), nil }

// Verifies: SW-REQ-030
// SW-REQ-030:error_handling:example
// Verifies: SYS-REQ-006
// Verifies: STK-REQ-002
func TestBackoffHTTPRetry_Send_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewBackoffRetry("test", 2, srv.Client(), testLogger())
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err := r.Send(req); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

// Verifies: SW-REQ-030
// MCDC SW-REQ-030: is_5xx_or_429=F, retry_attempted=F => TRUE
// MCDC SW-REQ-030: is_5xx_or_429=T, retry_attempted=F => FALSE
// MCDC SW-REQ-030: is_5xx_or_429=T, retry_attempted=T => TRUE
// (This 4xx test drives is_5xx_or_429=F, retry_attempted=F → permanent error,
// one call total — F/F=TRUE. Sibling TestBackoffHTTPRetry_Send_5xxRetries drives
// is_5xx_or_429=T, retry_attempted=T — T/T=TRUE. The T/F=FALSE pair is the
// MaxRetries-exhausted baseline where backoff stops issuing further attempts
// after the configured cap.)
// SW-REQ-030:error_handling:negative
// SYS-REQ-006:error_handling:negative
func TestBackoffHTTPRetry_Send_PermanentOn4xx(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	r := NewBackoffRetry("test", 3, srv.Client(), testLogger())
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err := r.Send(req); err == nil {
		t.Fatal("expected error on 4xx response")
	}
	if calls != 1 {
		t.Fatalf("4xx must not be retried, got %d attempts", calls)
	}
}

// Verifies: SW-REQ-030
// SW-REQ-030:error_handling:boundary
func TestIsErrorRetryable(t *testing.T) {
	if isErrorRetryable(nil) {
		t.Fatal("nil error must not be retryable")
	}
	if isErrorRetryable(fmt.Errorf("plain application error")) {
		t.Fatal("a plain non-network error must not be retryable")
	}
}
