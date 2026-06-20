// Package pumps — dogstatsd-pump-specific unit tests.
//
// The bulk of the round-trip coverage lives in udp_file_pumps_mcdc_test.go
// (TestDogStatsdPump_RoundTrip, TestDogStatsdPump_Init_*,
// TestDogStatsdPump_WriteData_CustomTags, TestDogStatsdPump_Shutdown_*).
// This file contains plain-unit-level checks for the pump's identity helpers.
package pumps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDogStatsdPump_New_GetName covers the New() + GetName() identity helpers.
//
// Verifies: SW-REQ-017
// SW-REQ-017:nominal:nominal
func TestDogStatsdPump_New_GetName(t *testing.T) {
	pump := (&DogStatsdPump{}).New().(*DogStatsdPump)
	require.NotNil(t, pump)
	assert.Equal(t, "DogStatsd Pump", pump.GetName())
}

// TestDogStatsdPump_GetEnvPrefix covers the GetEnvPrefix() reader after Init.
//
// Verifies: INT-REQ-004
// MCDC INT-REQ-004: contract_honoured=T, pump_methods_called=T => TRUE
func TestDogStatsdPump_GetEnvPrefix(t *testing.T) {
	addr, _ := newUDPSink(t)
	pump := &DogStatsdPump{}
	require.NoError(t, pump.Init(map[string]interface{}{
		"address":         addr,
		"meta_env_prefix": "MY_CUSTOM_PREFIX",
	}))
	assert.Equal(t, "MY_CUSTOM_PREFIX", pump.GetEnvPrefix())
}
