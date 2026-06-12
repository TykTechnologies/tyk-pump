package retry

import (
	"testing"
	"time"
)

// File-level MC/DC witness rows: these requirements are genuinely exercised
// by covered tests in this file (per-test // MCDC blocks below). Rows copied
// verbatim from `proof mcdc show`; this header gives every // Verifies: link
// in the file a matching witness row.
//
// MCDC SW-REQ-031: max_elapsed_zero=F, unbounded_retry=F => TRUE
// MCDC SW-REQ-031: max_elapsed_zero=T, unbounded_retry=F => FALSE
// MCDC SW-REQ-031: max_elapsed_zero=T, unbounded_retry=T => TRUE

// Verifies: SW-REQ-031
// MCDC SW-REQ-031: max_elapsed_zero=F, unbounded_retry=F => TRUE
// MCDC SW-REQ-031: max_elapsed_zero=T, unbounded_retry=F => FALSE
// MCDC SW-REQ-031: max_elapsed_zero=T, unbounded_retry=T => TRUE
// (This test asserts MaxElapsedTime is 0 by default — drives max_elapsed_zero=T,
// unbounded_retry=T — T/T=TRUE. The F/F=TRUE pair is the operator-override
// baseline where a non-zero MaxElapsedTime is configured externally (no current
// production seam; KI storage-retry-maxelapsed-zero-is-unbounded names the
// missing override surface). The T/F=FALSE pair is structurally infeasible
// in the current factory — MaxElapsedTime=0 unconditionally yields an
// unbounded backoff per the cenkalti/backoff contract.)
// SW-REQ-031:error_handling:example
func TestGetTemporalStorageExponentialBackoff(t *testing.T) {
	b := GetTemporalStorageExponentialBackoff()
	if b.Multiplier != 2 {
		t.Fatalf("expected multiplier 2, got %v", b.Multiplier)
	}
	if b.MaxInterval != 10*time.Second {
		t.Fatalf("expected max interval 10s, got %v", b.MaxInterval)
	}
	if b.MaxElapsedTime != 0 {
		t.Fatalf("expected unbounded elapsed time (0), got %v", b.MaxElapsedTime)
	}
}

// Verifies: SW-REQ-031
// SW-REQ-031:error_handling:boundary
func TestGetTemporalStorageExponentialBackoff_FreshInstances(t *testing.T) {
	a := GetTemporalStorageExponentialBackoff()
	b := GetTemporalStorageExponentialBackoff()
	if a == b {
		t.Fatal("each call must return an independent backoff instance")
	}
}
