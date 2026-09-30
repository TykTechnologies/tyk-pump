package promtest

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Eventually polls check until it reports done or timeout elapses. The
// string returned by check is the reason the last attempt was not done and
// becomes part of the failure message.
func Eventually(t testing.TB, timeout, interval time.Duration, check func(ctx context.Context) (done bool, reason string)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var reason string
	for {
		ctx, cancel := context.WithTimeout(context.Background(), interval*5)
		var done bool
		done, reason = check(ctx)
		cancel()
		if done {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(interval)
	}
	t.Fatalf("condition not met within %s: %s", timeout, reason)
}

// WaitForSeries polls until query returns at least one series and returns
// them. It fails the test on timeout.
func WaitForSeries(t testing.TB, c *Client, query string, timeout, interval time.Duration) []Series {
	t.Helper()
	var out []Series
	Eventually(t, timeout, interval, func(ctx context.Context) (bool, string) {
		series, err := c.Query(ctx, query)
		if err != nil {
			return false, err.Error()
		}
		if len(series) == 0 {
			return false, "query " + query + " returned no series"
		}
		out = series
		return true, ""
	})
	return out
}

// WaitForMetadata polls until Prometheus reports metadata for the family.
func WaitForMetadata(t testing.TB, c *Client, name string, timeout, interval time.Duration) Metadata {
	t.Helper()
	var out Metadata
	Eventually(t, timeout, interval, func(ctx context.Context) (bool, string) {
		md, ok, err := c.Metadata(ctx, name)
		if err != nil {
			return false, err.Error()
		}
		if !ok {
			return false, "no metadata for " + name + " yet"
		}
		out = md
		return true, ""
	})
	return out
}

// WaitForValue polls until query returns at least one series and every series
// reads exactly want, then returns them. It fails the test on timeout, naming
// the series that still differ.
func WaitForValue(t testing.TB, c *Client, query string, want float64, timeout, interval time.Duration) []Series {
	t.Helper()
	var out []Series
	Eventually(t, timeout, interval, func(ctx context.Context) (bool, string) {
		series, err := c.Query(ctx, query)
		if err != nil {
			return false, err.Error()
		}
		if len(series) == 0 {
			return false, "query " + query + " returned no series"
		}
		for _, s := range series {
			if s.Value != want {
				return false, fmt.Sprintf("%s = %v, want %v", s.LabelsString(), s.Value, want)
			}
		}
		out = series
		return true, ""
	})
	return out
}
