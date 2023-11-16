package pumps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sirupsen/logrus"
)

var errPerm = errors.New("bad request - not retrying")

type httpSender func(ctx context.Context, data []byte) (*http.Response, error)

type backoffRetry struct {
	errMsg     string
	maxRetries uint64
	logger     *logrus.Entry
	httpsend   httpSender
}

func newBackoffRetry(errMsg string, maxRetries uint64, httpSend httpSender, logger *logrus.Entry) *backoffRetry {
	return &backoffRetry{errMsg: errMsg, maxRetries: maxRetries, httpsend: httpSend, logger: logger}
}

func (s *backoffRetry) send(ctx context.Context, data []byte) error {
	opFn := func() error {
		resp, err := s.httpsend(ctx, data)
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
		if err != nil {
			s.logger.WithError(err).Errorf("%s retrying in %s", s.errMsg, t)
		}
	})
}

func (s *backoffRetry) handleErr(err error) error {
	if e, ok := err.(*url.Error); ok {
		if e.Temporary() {
			// temp error, attempt retry
			return err
		}
		// permanent error - don't retry
		return backoff.Permanent(err)
	}
	return err
}
