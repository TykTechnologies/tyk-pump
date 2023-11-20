package httpretry

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sirupsen/logrus"
)

type BackoffHTTPRetry struct {
	errMsg     string
	maxRetries uint64
	logger     *logrus.Entry
	httpclient *http.Client
}

// NewBackoffRetry Creates an exponential backoff retry to use httpClient for connections. Will retry if a temporary error or
// 5xx or 429 status code in response.
func NewBackoffRetry(errMsg string, maxRetries uint64, httpClient *http.Client, logger *logrus.Entry) *BackoffHTTPRetry {
	return &BackoffHTTPRetry{errMsg: errMsg, maxRetries: maxRetries, httpclient: httpClient, logger: logger}
}

func (s *BackoffHTTPRetry) Send(req *http.Request) error {
	opFn := func() error {
		resp, err := s.httpclient.Do(req)
		if err != nil {
			return s.handleErr(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		body, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("got status code %d and response '%s'", resp.StatusCode, body)

		// server error or rate limit hit - attempt retry
		if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
			return err
		}

		// any other error treat as permanent (i.e. auth error, invalid request) and don't retry
		return backoff.Permanent(err)
	}

	return backoff.RetryNotify(opFn, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), s.maxRetries), func(err error, t time.Duration) {
		s.logger.WithError(err).Errorf("%s retrying in %s", s.errMsg, t)
	})
}

func (s *BackoffHTTPRetry) handleErr(err error) error {
	if e, ok := err.(*url.Error); ok {
		if e.Temporary() {
			// temp error, attempt retry
			return err
		}
		// permanent error - don't retry
		return backoff.Permanent(err)
	}
	// anything else - retry
	return err
}
