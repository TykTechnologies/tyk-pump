package pumps

import (
	"os"
	"testing"
)

// Verifies: SW-REQ-034
//
// TestMain is the single entry point for the pumps test binary. Its job is
// post-test cleanup: tear down every testcontainer that any test in this
// package spun up, so a CI worker (or a dev laptop) doesn't leak ~2GB of
// idle containers between consecutive `go test ./pumps` runs.
//
// testcontainers-go also ships a Reaper sidecar that cleans up after
// crashed test processes; this TestMain is the clean-exit counterpart —
// it fires faster (no 10-second Reaper grace) and leaves the daemon in a
// state where `docker ps` is empty when the test binary returns 0.
func TestMain(m *testing.M) {
	code := m.Run()
	terminateSharedContainers()
	os.Exit(code)
}
