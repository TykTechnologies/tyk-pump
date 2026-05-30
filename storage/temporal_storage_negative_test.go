package storage

import (
	"testing"
	"time"
)

// Verifies: SW-REQ-006
// Verifies: SW-REQ-007
// SW-REQ-006:atomicity:negative
// SW-REQ-007:error_handling:negative
// SW-REQ-031:error_handling:negative
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
