package pumps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

func TestSyslogPump_Init(t *testing.T) {
	pump := &SyslogPump{}
	
	// Test with default configuration
	config := map[string]interface{}{
		"transport":     "udp",
		"network_addr":  "localhost:5140",
		"log_level":     6,
		"tag":           "test-tag",
	}
	
	err := pump.Init(config)
	require.NoError(t, err)
	
	assert.Equal(t, "udp", pump.syslogConf.Transport)
	assert.Equal(t, "localhost:5140", pump.syslogConf.NetworkAddr)
	assert.Equal(t, 6, pump.syslogConf.LogLevel)
	assert.Equal(t, "test-tag", pump.syslogConf.Tag)
	assert.False(t, pump.syslogConf.SyslogFragmentation) // Should default to false
}

func TestSyslogPump_InitWithFragmentation(t *testing.T) {
	pump := &SyslogPump{}
	
	// Test with syslog_fragmentation enabled
	config := map[string]interface{}{
		"transport":             "udp",
		"network_addr":          "localhost:5140",
		"log_level":             6,
		"syslog_fragmentation":  true,
	}
	
	err := pump.Init(config)
	require.NoError(t, err)
	
	assert.True(t, pump.syslogConf.SyslogFragmentation)
}

func TestSyslogPump_GetName(t *testing.T) {
	pump := &SyslogPump{}
	assert.Equal(t, "Syslog Pump", pump.GetName())
}

func TestSyslogPump_New(t *testing.T) {
	pump := &SyslogPump{}
	newPump := pump.New()
	
	assert.IsType(t, &SyslogPump{}, newPump)
}

func TestSyslogPump_GetEnvPrefix(t *testing.T) {
	pump := &SyslogPump{
		syslogConf: &SyslogConf{
			EnvPrefix: "TEST_PREFIX",
		},
	}
	
	assert.Equal(t, "TEST_PREFIX", pump.GetEnvPrefix())
}

func TestSyslogPump_SetAndGetTimeout(t *testing.T) {
	pump := &SyslogPump{}
	
	pump.SetTimeout(30)
	assert.Equal(t, 30, pump.GetTimeout())
}

func TestSyslogPump_SetAndGetFilters(t *testing.T) {
	pump := &SyslogPump{}
	filters := analytics.AnalyticsFilters{
		APIIDs: []string{"api1", "api2"},
	}
	
	pump.SetFilters(filters)
	assert.Equal(t, filters, pump.GetFilters())
}
