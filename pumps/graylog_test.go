// Package pumps — graylog-pump-specific unit tests.
//
// The bulk of the GELF round-trip coverage lives in
// udp_file_pumps_mcdc_test.go (TestGraylogPump_RoundTrip_TagFiltering,
// TestGraylogPump_Init_Defaults, TestGraylogPump_WriteData_FatalContract_KI).
// This file contains plain-unit-level checks for the pump's identity helpers
// and a couple of negative-path scenarios.
package pumps

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGraylogPump_New_GetName covers the New() + GetName() identity helpers.
//
// Verifies: SW-REQ-049
func TestGraylogPump_New_GetName(t *testing.T) {
	pump := (&GraylogPump{}).New().(*GraylogPump)
	require.NotNil(t, pump)
	assert.Equal(t, "Graylog Pump", pump.GetName())
}

// TestGraylogPump_GetEnvPrefix covers GetEnvPrefix() after Init.
//
// Verifies: SW-REQ-049
func TestGraylogPump_GetEnvPrefix(t *testing.T) {
	pump := &GraylogPump{}
	require.NoError(t, pump.Init(map[string]interface{}{
		"meta_env_prefix": "TYK_PMP_PUMPS_GRAYLOG_META_OVERRIDE",
	}))
	assert.Equal(t, "TYK_PMP_PUMPS_GRAYLOG_META_OVERRIDE", pump.GetEnvPrefix())
}

// TestGraylogPump_WriteData_ValidBase64 covers the happy-path base64-decode
// branches for both RawRequest and RawResponse — neither triggers log.Fatal.
//
// Verifies: SW-REQ-049
func TestGraylogPump_WriteData_ValidBase64(t *testing.T) {
	addr, sink := newUDPSink(t)
	host, port := graylogAddrParts(t, addr)

	pump := &GraylogPump{}
	require.NoError(t, pump.Init(map[string]interface{}{
		"host": host,
		"port": port,
		"tags": []string{"raw_request", "raw_response"},
	}))

	req := base64.StdEncoding.EncodeToString([]byte("GET /foo HTTP/1.1"))
	resp := base64.StdEncoding.EncodeToString([]byte("HTTP/1.1 200 OK"))

	require.NoError(t, pump.WriteData(context.Background(), []interface{}{
		analytics.AnalyticsRecord{
			Method:      "GET",
			Path:        "/foo",
			RawRequest:  req,
			RawResponse: resp,
		},
	}))

	got := drainBytes(sink, 2*time.Second)
	require.NotEmpty(t, got)
}
