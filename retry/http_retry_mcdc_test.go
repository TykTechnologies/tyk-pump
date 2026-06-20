package retry

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/cenkalti/backoff/v4"
)

// File-level MC/DC witness rows for SW-REQ-030 retry predicate tests. Rows
// copied verbatim from `proof mcdc show SW-REQ-030`.
//
// MCDC SW-REQ-030: is_5xx_or_429=F, retry_attempted=F, transport_transient_err=F => TRUE
// MCDC SW-REQ-030: is_5xx_or_429=F, retry_attempted=F, transport_transient_err=T => FALSE
// MCDC SW-REQ-030: is_5xx_or_429=T, retry_attempted=F, transport_transient_err=F => FALSE
// MCDC SW-REQ-030: is_5xx_or_429=T, retry_attempted=F, transport_transient_err=T => FALSE
// MCDC SW-REQ-030: is_5xx_or_429=T, retry_attempted=T, transport_transient_err=T => TRUE

// onlyTempFalse implements only the Temporary() interface and returns false.
// It is used to force the false-side independence proof for the
// `tempErr.Temporary()` operand of the switch case at http-retry.go:143
// without accidentally matching any earlier case in the switch (e.g. it does
// not implement ConnectionError, does not unwrap to a url.Error or net.OpError,
// and its message does not contain "connection reset").
type onlyTempFalse struct{ msg string }

func (e *onlyTempFalse) Error() string { return e.msg }

func (e *onlyTempFalse) Temporary() bool { return false }

// onlyTimeoutFalse implements only the Timeout() interface and returns false.
// It is used to force the false-side independence proof for the
// `timeoutErr.Timeout()` operand of the switch case at http-retry.go:147.
// It must not also satisfy the earlier tempError or any other interface used
// by earlier switch cases.
type onlyTimeoutFalse struct{ msg string }

func (e *onlyTimeoutFalse) Error() string { return e.msg }

func (e *onlyTimeoutFalse) Timeout() bool { return false }

// errReader is an io.Reader that always returns an error on Read. It is used
// to drive the `err != nil` true-branch at http-retry.go:44 when Send's
// io.ReadAll(req.Body) fails before the retry loop ever begins.
type errReader struct{ err error }

func (r *errReader) Read(_ []byte) (int, error) { return 0, r.err }

// MC/DC: drives the T-side of `isErrorRetryable(errors.Unwrap(urlErr))` at
// http-retry.go:136. The wrapped url.Error's inner Err is itself retryable
// (a tempError reporting Temporary()=true), so the recursive call must
// return true, exiting before falling through to the netOpErr / tempErr /
// timeoutErr cases.
//
// Verifies: SW-REQ-030
func TestIsErrorRetryable_URLError_WrapsRetryableInner_True(t *testing.T) {
	inner := &tempErr{"transient"}
	err := &url.Error{Op: "Get", URL: "http://x", Err: inner}
	if !isErrorRetryable(err) {
		t.Fatalf("url.Error wrapping a retryable temporary error must be retryable")
	}
}

// MC/DC: drives the T-side of `isErrorRetryable(errors.Unwrap(netOpErr))` at
// http-retry.go:142. The outer net.OpError has Op="read" (not dial) and its
// Temporary() returns false because its immediate inner is itself another
// net.OpError that wraps a plain error. The recursive call on the unwrapped
// inner then hits the "dial" branch and reports retryable=true. This is the
// only retryable-inner shape that does not also short-circuit earlier cases
// (connection-reset substring, url.Error match, or netOpErr.Temporary T).
//
// Verifies: SW-REQ-030
func TestIsErrorRetryable_NetOpError_WrapsRetryableInner_True(t *testing.T) {
	inner := &net.OpError{Op: "dial", Err: errors.New("inner")}
	outer := &net.OpError{Op: "read", Err: inner}
	if !isErrorRetryable(outer) {
		t.Fatalf("net.OpError wrapping a dial net.OpError must be retryable")
	}
}

// MC/DC: drives the T-side of `netOpErr.Temporary()` at http-retry.go:139.
// Op != "dial" forces the left operand to false, so the right operand must
// be evaluated; the wrapped tempError has Temporary()=true so
// net.OpError.Temporary() also returns true and the overall decision is T.
//
// Verifies: SW-REQ-030
func TestIsErrorRetryable_NetOpError_NonDial_TemporaryTrue(t *testing.T) {
	err := &net.OpError{Op: "read", Err: &tempErr{"temp-inner"}}
	if !isErrorRetryable(err) {
		t.Fatalf("net.OpError with Op!=dial but a Temporary inner must be retryable")
	}
}

// MC/DC: drives the F-side of `tempErr.Temporary()` at http-retry.go:143.
// The error implements only Temporary() (so it matches `errors.As(err,
// &tempErr)`) and returns false, forcing the second operand to be evaluated
// and produce F. The decision overall must be false and the function must
// classify it as non-retryable.
//
// Verifies: SW-REQ-030
func TestIsErrorRetryable_TemporaryFalse_NotRetryable(t *testing.T) {
	err := &onlyTempFalse{"not-temp"}
	if isErrorRetryable(err) {
		t.Fatalf("an error whose Temporary() returns false must not be retryable")
	}
}

// MC/DC: drives the F-side of `timeoutErr.Timeout()` at http-retry.go:147.
// The error implements only Timeout() (so it matches `errors.As(err,
// &timeoutErr)`) and returns false, forcing the second operand to be
// evaluated to F and the case to fall through. The function must classify
// it as non-retryable.
//
// Verifies: SW-REQ-030
func TestIsErrorRetryable_TimeoutFalse_NotRetryable(t *testing.T) {
	err := &onlyTimeoutFalse{"not-timeout"}
	if isErrorRetryable(err) {
		t.Fatalf("an error whose Timeout() returns false must not be retryable")
	}
}

// MC/DC: drives the T-side of `isErrorRetryable(err)` at http-retry.go:103
// inside (*BackoffHTTPRetry).handleErr. When the err is retryable, handleErr
// must return the original err unchanged (no backoff.Permanent wrapping).
//
// Verifies: SW-REQ-030
func TestBackoffHTTPRetry_handleErr_RetryableReturnsOriginalErr(t *testing.T) {
	s := NewBackoffRetry("test", 1, http.DefaultClient, testLogger())
	in := &tempErr{"transient"}
	got := s.handleErr(in)
	if got != error(in) {
		t.Fatalf("handleErr must return the original retryable error unchanged; got %v want %v", got, in)
	}
}

// MC/DC: drives the F-side of `isErrorRetryable(err)` at http-retry.go:103
// inside (*BackoffHTTPRetry).handleErr. A non-retryable error must be
// wrapped in backoff.Permanent so the retry loop terminates immediately
// rather than reissuing the request.
//
// Verifies: SW-REQ-030
func TestBackoffHTTPRetry_handleErr_NonRetryableWrappedPermanent(t *testing.T) {
	s := NewBackoffRetry("test", 1, http.DefaultClient, testLogger())
	in := errors.New("plain permanent error")
	got := s.handleErr(in)
	if got == nil {
		t.Fatal("handleErr must return a non-nil error for a non-retryable input")
	}
	if got == in {
		t.Fatal("handleErr must wrap (not return verbatim) a non-retryable error so backoff treats it as permanent")
	}
	var perm *backoff.PermanentError
	if !errors.As(got, &perm) {
		t.Fatalf("handleErr must wrap non-retryable errors in backoff.Permanent; got %T", got)
	}
	if !errors.Is(got, in) {
		t.Fatalf("wrapped permanent error must still chain to the original cause; got %v", got)
	}
}

// MC/DC: drives the T-side of `err != nil` at http-retry.go:44 in Send.
// A non-nil request body whose Read returns an error must cause Send to
// abort before the retry loop begins, surfacing the read error directly.
//
// Verifies: SW-REQ-030
func TestBackoffHTTPRetry_Send_BodyReadError_ReturnedImmediately(t *testing.T) {
	wantErr := errors.New("boom-reading-body")
	r := NewBackoffRetry("test", 3, http.DefaultClient, testLogger())
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:1", io.NopCloser(&errReader{err: wantErr}))
	if err != nil {
		t.Fatalf("unexpected NewRequest error: %v", err)
	}
	got := r.Send(req)
	if got == nil {
		t.Fatal("expected Send to return the body-read error, got nil")
	}
	if !errors.Is(got, wantErr) {
		t.Fatalf("expected returned error to wrap %v, got %v", wantErr, got)
	}
}
