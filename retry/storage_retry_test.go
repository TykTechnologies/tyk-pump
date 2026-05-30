package retry

import (
	"testing"
	"time"
)

// Verifies: SW-REQ-031
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
