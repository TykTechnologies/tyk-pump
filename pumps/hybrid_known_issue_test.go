package pumps

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Verifies: SW-REQ-029
// Verifies: KI:hybrid-getdialfn-leaks-conn-on-handshake-fail
// Reproduces: hybrid-getdialfn-leaks-conn-on-handshake-fail
func TestHybridGetDialFnWriteErrorDoesNotCloseConn_KI(t *testing.T) {
	sourceBytes, err := os.ReadFile("hybrid.go")
	require.NoError(t, err)
	source := string(sourceBytes)

	start := strings.Index(source, "func getDialFn(connID string, config *HybridPumpConf) func(addr string) (conn net.Conn, err error) {")
	require.NotEqual(t, -1, start, "getDialFn must remain present while this KI is open")
	end := strings.Index(source[start:], "\n// reqproof:implements SW-REQ-029\nfunc (p *HybridPump) WriteData")
	require.NotEqual(t, -1, end, "getDialFn end marker not found")
	getDialFnSource := source[start : start+end]

	writeFailure := `if _, err := conn.Write(data); err != nil {
				return nil, err
			}`
	require.Contains(t, getDialFnSource, writeFailure,
		"KI active: getDialFn still returns immediately on handshake Write failure")
	require.NotContains(t, getDialFnSource, "conn.Close()",
		"KI active: getDialFn write-failure path still lacks conn.Close before returning")
}
