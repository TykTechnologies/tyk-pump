package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Verifies: SYS-REQ-004
// Verifies: SW-REQ-002
// Verifies: SW-REQ-016
// Verifies: KI:main-startup-logfatal-on-transient-backend
// Reproduces: main-startup-logfatal-on-transient-backend
func TestMainStartupTransientStorageErrorsCallFatal_KI(t *testing.T) {
	sourceBytes, err := os.ReadFile("main.go")
	require.NoError(t, err)
	source := string(sourceBytes)

	setupStart := strings.Index(source, "func setupAnalyticsStore()")
	require.NotEqual(t, -1, setupStart, "setupAnalyticsStore must remain present while this KI is open")
	setupEnd := strings.Index(source[setupStart:], "\n// reqproof:implements SW-REQ-003\nfunc storeVersion()")
	require.NotEqual(t, -1, setupEnd, "setupAnalyticsStore end marker not found")
	setupSource := source[setupStart : setupStart+setupEnd]

	require.Contains(t, setupSource, "AnalyticsStore, err = storage.NewTemporalStorageHandler")
	require.Contains(t, setupSource, "err = AnalyticsStore.Init()")
	require.Contains(t, setupSource, "UptimeStorage, err = storage.NewTemporalStorageHandler")
	require.Contains(t, setupSource, "err = UptimeStorage.Init()")
	require.GreaterOrEqual(t, strings.Count(setupSource, ").Fatal(\"Error connecting to Temporal Storage: \", err)"), 3)
	require.Contains(t, setupSource, ").Fatal(\"Error connecting to Redis: \", err)")

	versionStart := strings.Index(source, "func storeVersion()")
	require.NotEqual(t, -1, versionStart, "storeVersion must remain present while this KI is open")
	versionEnd := strings.Index(source[versionStart:], "\n// reqproof:implements SW-REQ-003\nfunc initialisePumps()")
	require.NotEqual(t, -1, versionEnd, "storeVersion end marker not found")
	versionSource := source[versionStart : versionStart+versionEnd]

	require.Contains(t, versionSource, "versionStore, err := storage.NewTemporalStorageHandler")
	require.Contains(t, versionSource, "err = versionStore.Init()")
	require.Equal(t, 2, strings.Count(versionSource, ").Fatal(\"Error connecting to Temporal Storage: \", err)"))
}
