package storage

import (
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
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
	sharedRedisC    *tcredis.RedisContainer
)

// startSharedRedis spins one Redis container per test process and reuses it.
// Returns the host and port the container is reachable on.
// Tests should call this via redisHostPort(t) which will t.Skip() if Docker
// isn't available.
func startSharedRedis(ctx context.Context) (string, int, error) {
	sharedRedisOnce.Do(func() {
		if host := strings.TrimSpace(os.Getenv("REDIS_HOST")); host != "" {
			port := 6379
			if portStr := strings.TrimSpace(os.Getenv("REDIS_PORT")); portStr != "" {
				var err error
				port, err = strconv.Atoi(portStr)
				if err != nil {
					sharedRedisErr = err
					return
				}
			}
			sharedRedisHost = host
			sharedRedisPort = port
			return
		}

		container, err := tcredis.Run(ctx, "redis:7-alpine")
		if err != nil {
			sharedRedisErr = err
			return
		}
		sharedRedisC = container
		endpoint, err := container.Endpoint(ctx, "")
		if err != nil {
			_ = container.Terminate(context.Background())
			sharedRedisC = nil
			sharedRedisErr = err
			return
		}
		host, portStr, err := net.SplitHostPort(endpoint)
		if err != nil {
			_ = container.Terminate(context.Background())
			sharedRedisC = nil
			sharedRedisErr = err
			return
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			_ = container.Terminate(context.Background())
			sharedRedisC = nil
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
func redisHostPort(t *testing.T) (string, int) {
	t.Helper()
	if testing.Short() && strings.TrimSpace(os.Getenv("REDIS_HOST")) == "" {
		t.Skip("skipping redis testcontainer in short mode")
	}
	if strings.TrimSpace(os.Getenv("REDIS_HOST")) == "" && sharedRedisC == nil && sharedRedisHost == "" && sharedRedisErr == nil {
		requireTestcontainerMemory(t, "redis")
	}
	host, port, err := startSharedRedis(t.Context())
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("Docker not available for testcontainer Redis: %v", err)
		}
		t.Fatalf("failed to start testcontainer Redis: %v", err)
	}
	return host, port
}

// terminateSharedRedis removes the per-process Redis testcontainer on clean
// package exit. It intentionally leaves REDIS_HOST/REDIS_PORT overrides alone.
func terminateSharedRedis() {
	if sharedRedisC != nil {
		_ = sharedRedisC.Terminate(context.Background())
	}
}

// isDockerUnavailable returns true when the error is the well-known
// testcontainers-go message for an unreachable Docker daemon. We intentionally
// match on the message rather than a typed error because testcontainers-go
// wraps the underlying client error and exposes only the string.
func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Cannot connect to the Docker daemon") ||
		strings.Contains(msg, "Cannot connect to docker") ||
		strings.Contains(msg, "docker daemon")
}

func requireTestcontainerMemory(t *testing.T, name string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		return
	}
	minMiB := testcontainerMinFreeMiB()
	if minMiB <= 0 {
		return
	}
	freeMiB, err := macFreePlusSpeculativeMiB()
	if err != nil || freeMiB >= minMiB {
		return
	}
	t.Skipf("skipping %s testcontainer: macOS free+speculative memory is %d MiB, below %d MiB; set TYK_TESTCONTAINERS_MIN_FREE_MIB=0 to override", name, freeMiB, minMiB)
}

func testcontainerMinFreeMiB() int {
	const defaultMiB = 1024
	value := strings.TrimSpace(os.Getenv("TYK_TESTCONTAINERS_MIN_FREE_MIB"))
	if value == "" {
		return defaultMiB
	}
	minMiB, err := strconv.Atoi(value)
	if err != nil {
		return defaultMiB
	}
	return minMiB
}

func macFreePlusSpeculativeMiB() (int, error) {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, err
	}
	pageSize := int64(4096)
	var freePages, speculativePages int64
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if strings.Contains(line, "page size of") {
			for i, field := range fields {
				if field == "of" && i+1 < len(fields) {
					if parsed, err := strconv.ParseInt(strings.Trim(fields[i+1], ".)"), 10, 64); err == nil {
						pageSize = parsed
					}
					break
				}
			}
			continue
		}
		if len(fields) < 3 {
			continue
		}
		pages, err := strconv.ParseInt(strings.TrimRight(fields[len(fields)-1], "."), 10, 64)
		if err != nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Pages free:"):
			freePages = pages
		case strings.HasPrefix(line, "Pages speculative:"):
			speculativePages = pages
		}
	}
	return int((freePages + speculativePages) * pageSize / 1048576), nil
}

// Ensure the testcontainers import doesn't become orphaned if we later
// move startSharedRedis. testcontainers is the umbrella we depend on; keeping
// a reference here means a future-add of another container family compiles
// without re-import.
var _ = testcontainers.WithLogger
