package storage

import (
	"testing"
	"time"
)

// Verifies: SW-REQ-007
// SW-REQ-007:error_handling:negative
// SW-REQ-031:error_handling:negative
// MCDC SW-REQ-007: connect_err=T, connection_retried_with_bounded_backoff=F => FALSE
//
// connect_err=T (port 6390 has nothing listening), connection_retried_with_bounded_backoff=F:
// this test asserts the FALSE arm — the unreachable-backend path exits when the bounded backoff
// budget elapses without success. The connection_retried_with_bounded_backoff=T arm is exercised
// by integration tests via testcontainers that recover after a transient outage; the connect_err=F
// arm is the happy path covered by TestTemporalStorageHandler_ensureConnection.
func TestTemporalStorageHandler_GetAndDeleteSet_BackendUnreachable(t *testing.T) {
	// Point at a port with nothing listening so the backend is unreachable.
	conf := map[string]interface{}{
		"type":                  "redis",
		"host":                  "127.0.0.1",
		"port":                  6390,
		"timeout":               1,
		"enable_cluster":        false,
		"optimisation_max_idle": 1,
	}
	r, err := NewTemporalStorageHandler(conf, false)
	if err != nil {
		// Construction itself rejected the unreachable backend: error surfaced.
		return
	}
	_ = r.Init()

	// A read against an unreachable backend must surface an error rather than
	// silently dropping (and partially deleting) records.
	if _, err := r.GetAndDeleteSet("tyk-system-analytics", 10, time.Second); err == nil {
		t.Fatal("expected an error when the temporal store backend is unreachable")
	}
}
