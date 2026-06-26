//go:build race

package storage

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verifies: SW-REQ-007
// Verifies: SYS-REQ-006
// Verifies: KI:storage-connector-singleton-race
// Reproduces: storage-connector-singleton-race
func TestTemporalStorageHandler_Init_ConnectorSingletonRace_KI(t *testing.T) {
	if os.Getenv("TYK_PUMP_STORAGE_SINGLETON_RACE_CHILD") == "1" {
		runTemporalStorageConnectorSingletonRaceChild(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestTemporalStorageHandler_Init_ConnectorSingletonRace_KI$")
	cmd.Env = append(os.Environ(), "TYK_PUMP_STORAGE_SINGLETON_RACE_CHILD=1")
	output, err := cmd.CombinedOutput()

	require.Error(t, err, "KI active: race-instrumented child should report connectorSingleton/logPrefix data races")
	text := string(output)
	assert.Contains(t, text, "DATA RACE", "child output should contain race detector output:\n%s", text)
	assert.True(t,
		strings.Contains(text, "temporal_storage.go:98") ||
			strings.Contains(text, "temporal_storage.go:143") ||
			strings.Contains(text, "temporal_storage.go:194"),
		"race should originate in temporal storage package globals:\n%s", text)
}

func runTemporalStorageConnectorSingletonRaceChild(t *testing.T) {
	t.Helper()
	resetSingletonForTest(t)
	t.Cleanup(func() { resetSingletonForTest(t) })

	const workers = 8
	const iterations = 50
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				cfg := &TemporalStorageConfig{
					Type: "redis",
					Host: "127.0.0.1",
					Port: 1,
				}
				if (id+j)%2 == 0 {
					cfg.Type = ""
				}
				handler, err := NewTemporalStorageHandler(cfg, true)
				if err != nil {
					t.Errorf("construct temporal storage handler: %v", err)
					continue
				}
				_ = handler.Init()
			}
		}(i)
	}
	close(start)
	wg.Wait()
}
