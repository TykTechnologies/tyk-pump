package retry

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Verifies: SW-REQ-030
// SW-REQ-030:error_handling:negative
//
// MCDC SW-REQ-030: is_5xx_or_429=T, retry_attempted=T, transport_transient_err=T => TRUE
//
// SW-REQ-030 row 5 is the satisfied row where BOTH retry triggers are present
// across a single Send lifecycle and a retry is in fact attempted. We drive
// both signals honestly within one Send:
//
//   - attempt 1: the server replies 503 Service Unavailable. Send classifies
//     this via `resp.StatusCode >= http.StatusInternalServerError` (is_5xx_or_429=T)
//     and returns the error to backoff, which schedules a retry.
//   - attempt 2: the server hijacks the connection and forces a TCP RST
//     (SetLinger(0) before Close), so httpclient.Do returns a "connection
//     reset by peer" transport error which handleErr -> isErrorRetryable
//     classifies as retryable (transport_transient_err=T), so backoff
//     schedules another retry.
//   - attempt 3: the server replies 200 OK and Send returns nil.
//
// Asserting that the server saw exactly three attempts proves retry_attempted=T
// under the simultaneous presence of the 5xx trigger and the transient-transport
// trigger -> the requirement holds (TRUE).
//
//mcdc:ignore SW-REQ-030: is_5xx_or_429=F, retry_attempted=F, transport_transient_err=T => FALSE — http-retry.go:61-62 routes a transport error through handleErr, and isErrorRetryable (http-retry.go:124-153) returns true for transient transport errors, so opFn returns a plain (non-Permanent) error and backoff.RetryNotify always retries; retry_attempted is always T under a transient transport error [reviewed: human:leo] [category: defensive]
//mcdc:ignore SW-REQ-030: is_5xx_or_429=T, retry_attempted=F, transport_transient_err=F => FALSE — http-retry.go:88-89 returns a plain (non-Permanent) error for any 5xx/429 response, so backoff.RetryNotify always retries; retry_attempted is always T under a 5xx-or-429 response [reviewed: human:leo] [category: defensive]
//mcdc:ignore SW-REQ-030: is_5xx_or_429=T, retry_attempted=F, transport_transient_err=T => FALSE — both the 5xx/429 arm (http-retry.go:88-89) and the transient-transport arm (http-retry.go:61-62 + isErrorRetryable) yield a plain (non-Permanent) error, so backoff.RetryNotify always retries; retry_attempted is always T when either trigger holds [reviewed: human:leo] [category: defensive]
func TestBackoffHTTPRetry_Send_Retries_On5xxThenTransientTransport_Row5(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		switch n {
		case 1:
			// is_5xx_or_429 = T
			w.WriteHeader(http.StatusServiceUnavailable)
		case 2:
			// transport_transient_err = T: force a TCP reset so the client's
			// Do() returns a "connection reset by peer" transport error rather
			// than a status code.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Errorf("test server does not support hijacking")
				return
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Errorf("hijack failed: %v", err)
				return
			}
			if tcp, ok := conn.(*net.TCPConn); ok {
				// Linger 0 makes Close send an RST instead of a graceful FIN,
				// surfacing as "connection reset by peer" on the client.
				_ = tcp.SetLinger(0)
			}
			_ = conn.Close()
		default:
			// retry succeeds
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	r := NewBackoffRetry("row5", 5, srv.Client(), testLogger())
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"row":5}`))
	if err != nil {
		t.Fatalf("unexpected NewRequest error: %v", err)
	}

	if err := r.Send(req); err != nil {
		t.Fatalf("expected Send to succeed after retrying on 5xx and a transient transport error, got %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts (5xx -> transient transport -> success), got %d", got)
	}
}
