package main

import "testing"

// Verifies: SW-REQ-005
// Verifies: SYS-REQ-013
// SW-REQ-005:error_handling:negative
// SYS-REQ-013:error_handling:negative
// Verifies: SYS-REQ-017
// SYS-REQ-017:error_handling:negative
// MCDC SW-REQ-005: instrumentation_enabled=F, statsd_sink_added=F => TRUE
// MCDC SW-REQ-005: instrumentation_enabled=T, statsd_sink_added=F => FALSE
// MCDC SW-REQ-005: instrumentation_enabled=T, statsd_sink_added=T => TRUE
// (This test drives the early-return path with a bad address — the StatsD
// sink construction fails before AddSink is called, so the
// instrumentation_enabled=T, statsd_sink_added=F arm is exercised. The
// instrumentation_enabled=F arm is covered by the default TYK_INSTRUMENTATION
// unset case in SetupInstrumentation; the T/T pair is covered by the
// integration scenario where StatsdConnectionString is valid and the sink
// is successfully added.)
func TestNewStatsDSink_InvalidAddress(t *testing.T) {
	// An unresolvable StatsD address must surface as an error rather than
	// silently producing a half-built sink.
	if _, err := NewStatsDSink("missing-port-address", nil); err == nil {
		t.Fatal("expected error for an unresolvable StatsD address")
	}
}
