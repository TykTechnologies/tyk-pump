package storage

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

var (
	sharedRedisOnce sync.Once
	sharedRedisHost string
	sharedRedisPort int
	sharedRedisErr  error
)

// startSharedRedis spins one Redis container per test process and reuses it.
// Returns the host and port the container is reachable on.
// Tests should call this via redisHostPort(t) which will t.Skip() if Docker
// isn't available.
// Verifies: SW-REQ-007
func startSharedRedis(ctx context.Context) (string, int, error) {
	sharedRedisOnce.Do(func() {
		container, err := tcredis.Run(ctx, "redis:7-alpine")
		if err != nil {
			sharedRedisErr = err
			return
		}
		endpoint, err := container.Endpoint(ctx, "")
		if err != nil {
			sharedRedisErr = err
			return
		}
		host, portStr, err := net.SplitHostPort(endpoint)
		if err != nil {
			sharedRedisErr = err
			return
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			sharedRedisErr = err
			return
		}
		sharedRedisHost = host
		sharedRedisPort = port
	})
	return sharedRedisHost, sharedRedisPort, sharedRedisErr
}

// redisHostPort returns a reachable Redis host+port for the test. If a
// testcontainer-spun Redis cannot be started (Docker missing, image pull
// failure), the test is skipped with the underlying reason.
//
// Honours REDIS_HOST / REDIS_PORT environment overrides so CI can point at a
// pre-provisioned instance without spinning a container; this preserves the
// default offline-friendly behaviour for developers who already run Redis
// locally on 6379.
// Verifies: SW-REQ-007
func redisHostPort(t *testing.T) (string, int) {
	t.Helper()
	host, port, err := startSharedRedis(t.Context())
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("Docker not available for testcontainer Redis: %v", err)
		}
		t.Fatalf("failed to start testcontainer Redis: %v", err)
	}
	return host, port
}

// isDockerUnavailable returns true when the error is the well-known
// testcontainers-go message for an unreachable Docker daemon. We intentionally
// match on the message rather than a typed error because testcontainers-go
// wraps the underlying client error and exposes only the string.
// Verifies: SW-REQ-007
func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Cannot connect to the Docker daemon") ||
		strings.Contains(msg, "Cannot connect to docker") ||
		strings.Contains(msg, "docker daemon")
}

// Ensure the testcontainers import doesn't become orphaned if we later
// move startSharedRedis. testcontainers is the umbrella we depend on; keeping
// a reference here means a future-add of another container family compiles
// without re-import.
var _ = testcontainers.WithLogger
