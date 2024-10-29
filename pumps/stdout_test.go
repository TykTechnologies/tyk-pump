package pumps

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
)

// Previous implementation was canceling the writes if the context was cancelled.
// This test ensures that the pump will continue to write data even if the context is cancelled.
func TestStdOutPump_WriteData_ContextCancellation(t *testing.T) {
    // Setup pump
    pump := &StdOutPump{
        conf: &StdOutConf{
            LogFieldName: "test-analytics",
            Format:      "json",
        },
    }

    // Setup logger
    logger := logrus.New()
    logger.SetLevel(logrus.DebugLevel)
    pump.log = logger.WithField("prefix", "test")

	// Create many records to test with
    data := make([]interface{}, 100)
    for i := range data {
        data[i] = analytics.AnalyticsRecord{
            Path:   fmt.Sprintf("/test/%d", i),
            Method: "GET",
        }
    }

    // Create an already cancelled context 
	// && cancel immediately
    ctx, cancel := context.WithCancel(context.Background())
    cancel() 


	// Capture logger output
	var buf bytes.Buffer
	pump.log.Logger.SetOutput(&buf)
	oldOut := pump.log.Logger.Out
	defer pump.log.Logger.SetOutput(oldOut)  // restore original output when done

    err := pump.WriteData(ctx, data)

	output := buf.String()

    if err != nil {
        t.Errorf("Expected no error, wanted the Pump to finish purging despite context cancellation, got %v", err)
    }

	// Verify the output contains the expected message
	attemptMsg := "Attempting to write 100 records"
	if !strings.Contains(output, attemptMsg) {
		t.Errorf("Expected output does not contain '%s'", attemptMsg)
	}
	purgeMsg := "Purged 100 records..."
	if !strings.Contains(output, purgeMsg) {
		t.Errorf("Expected output does not contain '%s'", purgeMsg)
	}
}