package metrics

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/ci/tests/metrics/promtest"
)

// The Gateway's two keyless APIs (configs/gateway/apps). The pumps profile's
// "filtered" pump skips api1's records.
const (
	api1Path = "/api-1/"
	api2Path = "/api-2/"
)

// gatewayURL defaults to the port docker-compose publishes.
var gatewayURL = envOr("GATEWAY_URL", "http://localhost:"+envOr("GATEWAY_PORT", "8080"))

// waitForGateway blocks until the Gateway answers /hello with 200, i.e. it is
// up and connected to Redis.
func waitForGateway(t *testing.T) {
	t.Helper()
	promtest.Eventually(t, pollTimeout, pollInterval, func(ctx context.Context) (bool, string) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, gatewayURL+"/hello", nil)
		if err != nil {
			return false, err.Error()
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return false, err.Error()
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false, "gateway /hello: " + resp.Status
		}
		return true, ""
	})
}

// sendTraffic sends n requests through the Gateway API at listenPath, each to
// a distinct path under prefix (no leading slash). Every request becomes one
// analytics record, whatever its status code.
func sendTraffic(t *testing.T, listenPath, prefix string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		url := fmt.Sprintf("%s%s%s/%d?run=%d", gatewayURL, listenPath, prefix, i, time.Now().UnixNano())
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		// A different key on every request: keyless APIs ignore it, and it
		// must not show up in any label.
		req.Header.Set("Authorization", fmt.Sprintf("key-%s-%d", prefix, i))
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("request %s: %v", url, err)
		}
		resp.Body.Close()
	}
}

// pushToAnalytics RPUSHes raw values onto the analytics list the Gateway
// writes to, next to its real records.
func pushToAnalytics(t *testing.T, values ...string) {
	t.Helper()
	compose(t, append([]string{"exec", "-T", "redis", "redis-cli", "RPUSH", "analytics-tyk-system-analytics"}, values...)...)
}

// points returns the latest value of every Pump series of family, keyed by
// the values of keys joined with "/".
func points(ctx context.Context, family string, keys ...string) (map[string]float64, error) {
	series, err := prom.Query(ctx, pumpSelector(family))
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, s := range series {
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = s.Labels[k]
		}
		out[strings.Join(parts, "/")] += s.Value
	}
	return out, nil
}

// mustPoints is points that fails the test on a query error.
func mustPoints(t *testing.T, family string, keys ...string) map[string]float64 {
	t.Helper()
	p, err := points(context.Background(), family, keys...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// waitForPoints waits until points(family, keys...) equals want exactly.
func waitForPoints(t *testing.T, family string, want map[string]float64, keys ...string) {
	t.Helper()
	promtest.Eventually(t, pollTimeout, pollInterval, func(ctx context.Context) (bool, string) {
		got, err := points(ctx, family, keys...)
		if err != nil {
			return false, err.Error()
		}
		if !maps.Equal(got, want) {
			return false, fmt.Sprintf("%s = %v, want %v", family, got, want)
		}
		return true, ""
	})
}
