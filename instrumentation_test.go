package main

import "testing"

// Verifies: SW-REQ-005
// Verifies: SYS-REQ-013
// SW-REQ-005:error_handling:negative
// SYS-REQ-013:error_handling:negative
// Verifies: SYS-REQ-017
// SYS-REQ-017:error_handling:negative
func TestNewStatsDSink_InvalidAddress(t *testing.T) {
	// An unresolvable StatsD address must surface as an error rather than
	// silently producing a half-built sink.
	if _, err := NewStatsDSink("missing-port-address", nil); err == nil {
		t.Fatal("expected error for an unresolvable StatsD address")
	}
}
