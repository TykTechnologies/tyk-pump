package pumps

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func readPumpSource(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// TestSQLPumpWriteDataSwallowsBatchError_KI is a static tripwire for the
// standard SQL member of pump-writedata-swallows-per-batch-errors.
// Verifies: STK-REQ-002
// Verifies: KI:pump-writedata-swallows-per-batch-errors
// Reproduces: pump-writedata-swallows-per-batch-errors
func TestSQLPumpWriteDataSwallowsBatchError_KI(t *testing.T) {
	source := readPumpSource(t, "sql.go")

	require.Regexp(t,
		regexp.MustCompile(`(?s)func \(c \*SQLPump\) WriteData\(.*?c\.log\.Error\(tx\.Error\).*?return nil`),
		source,
	)
}

// TestSQSPumpBatchLimitZeroInfiniteLoop_KI is a static tripwire for the SQS
// zero-batch-limit loop. Executing the path with AWSSQSBatchLimit=0 would hang,
// so the test pins the loop shape instead.
// Verifies: SW-REQ-055
// Verifies: KI:sqs-batchlimit-zero-infinite-loop
// Reproduces: sqs-batchlimit-zero-infinite-loop
func TestSQSPumpBatchLimitZeroInfiniteLoop_KI(t *testing.T) {
	source := readPumpSource(t, "sqs.go")

	require.Contains(t, source, "AWSSQSBatchLimit")
	require.Contains(t, source, "i + s.SQSConf.AWSSQSBatchLimit")
	require.NotRegexp(t, regexp.MustCompile(`AWSSQSBatchLimit\s*(<=\s*0|<\s*1|==\s*0)`), source)
}

// TestLogzioPumpMissingShutdownFlush_KI is a static tripwire for the Logz.io
// half of logzio-segment-no-shutdown-flush.
// Verifies: STK-REQ-002
// Verifies: KI:logzio-segment-no-shutdown-flush
// Reproduces: logzio-segment-no-shutdown-flush
func TestLogzioPumpMissingShutdownFlush_KI(t *testing.T) {
	source := readPumpSource(t, "logzio.go")

	require.Contains(t, source, "CommonPumpConfig")
	require.NotRegexp(t,
		regexp.MustCompile(`func \([^)]*\*LogzioPump[^)]*\) Shutdown\(`),
		source,
	)
}

// TestSegmentPumpMissingShutdownFlush_KI is a static tripwire for the Segment
// half of logzio-segment-no-shutdown-flush.
// Verifies: STK-REQ-002
// Verifies: KI:logzio-segment-no-shutdown-flush
// Reproduces: logzio-segment-no-shutdown-flush
func TestSegmentPumpMissingShutdownFlush_KI(t *testing.T) {
	source := readPumpSource(t, "segment.go")

	require.Contains(t, source, "CommonPumpConfig")
	require.NotRegexp(t,
		regexp.MustCompile(`func \([^)]*\*SegmentPump[^)]*\) Shutdown\(`),
		source,
	)
}
