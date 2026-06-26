package pumps

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func readPumpSource(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func sourceBetween(t *testing.T, source, startMarker, endMarker string) string {
	t.Helper()

	start := strings.Index(source, startMarker)
	require.NotEqual(t, -1, start, "start marker %q not found", startMarker)
	end := strings.Index(source[start:], endMarker)
	require.NotEqual(t, -1, end, "end marker %q not found", endMarker)
	return source[start : start+end]
}

func sourceFrom(t *testing.T, source, startMarker string) string {
	t.Helper()

	start := strings.Index(source, startMarker)
	require.NotEqual(t, -1, start, "start marker %q not found", startMarker)
	return source[start:]
}

// TestSQLPumpWriteDataSwallowsBatchError_KI is a static tripwire for the
// remaining true members of pump-writedata-swallows-per-batch-errors.
// Verifies: INT-REQ-004
// Verifies: SW-REQ-035
// Verifies: SW-REQ-040
// Verifies: SW-REQ-046
// Verifies: SW-REQ-047
// Verifies: SW-REQ-049
// Verifies: SW-REQ-050
// Verifies: SW-REQ-051
// Verifies: SW-REQ-053
// Verifies: SW-REQ-056
// Verifies: KI:pump-writedata-swallows-per-batch-errors
// Reproduces: pump-writedata-swallows-per-batch-errors
func TestSQLPumpWriteDataSwallowsBatchError_KI(t *testing.T) {
	t.Run("mongo selective insert and index errors log then return nil", func(t *testing.T) {
		source := readPumpSource(t, "mongo_selective.go")
		require.Regexp(t,
			regexp.MustCompile(`(?s)func \(m \*MongoSelectivePump\) WriteData\(.*?m\.ensureIndexes\(colName\).*?m\.log\.WithField\("collection", colName\)\.Error\(indexCreateErr\).*?m\.store\.Insert\(context\.Background\(\), dataSet\.\.\.\).*?m\.log\.WithField\("collection", colName\)\.Error\("Problem inserting to mongo collection: ", err\).*?return nil`),
			source,
		)
	})

	t.Run("mongo aggregate is excluded because WriteData returns backend errors", func(t *testing.T) {
		source := readPumpSource(t, "mongo_aggregate.go")
		writeData := sourceBetween(t, source,
			"func (m *MongoAggregatePump) WriteData(ctx context.Context, data []interface{}) error",
			"\n// reqproof:implements SW-REQ-059,SW-REQ-060,SW-REQ-084,SW-REQ-096\nfunc (m *MongoAggregatePump) DoAggregatedWriting",
		)
		require.Contains(t, writeData, "err := m.DoAggregatedWriting(ctx, &filteredData, isMixedCollection)")
		require.Contains(t, writeData, "return err")
	})

	t.Run("kinesis PutRecords API errors log then return nil", func(t *testing.T) {
		source := readPumpSource(t, "kinesis.go")
		writeData := sourceBetween(t, source,
			"func (p *KinesisPump) WriteData(ctx context.Context, records []interface{}) error",
			"\n// splitIntoBatches splits",
		)
		require.Contains(t, writeData, "output, err := p.client.PutRecords(ctx, input)")
		require.Contains(t, writeData, `p.log.Error("failed to put records to Kinesis: ", err)`)
		require.Contains(t, writeData, "return nil")
		require.NotContains(t, writeData, "return err")
	})

	t.Run("sql create errors log then return nil", func(t *testing.T) {
		source := readPumpSource(t, "sql.go")
		require.Regexp(t,
			regexp.MustCompile(`(?s)func \(c \*SQLPump\) WriteData\(.*?c\.log\.Error\(tx\.Error\).*?return nil`),
			source,
		)
	})

	t.Run("graylog client log has no error channel and WriteData returns nil", func(t *testing.T) {
		source := readPumpSource(t, "graylog.go")
		writeData := sourceFrom(t, source,
			"func (p *GraylogPump) WriteData(ctx context.Context, data []interface{}) error",
		)
		require.Contains(t, writeData, "p.client.Log(string(gelfString))")
		require.Contains(t, writeData, "return nil")
	})

	t.Run("syslog writer error is explicitly discarded", func(t *testing.T) {
		source := readPumpSource(t, "syslog.go")
		writeData := sourceBetween(t, source,
			"func (s *SyslogPump) WriteData(ctx context.Context, data []interface{}) error",
			"\nfunc (s *SyslogPump) SetTimeout",
		)
		require.Contains(t, writeData, `_, _ = fmt.Fprintf(s.writer, "%s", message)`)
		require.Contains(t, writeData, "return nil")
	})

	t.Run("logzio sender error is ignored", func(t *testing.T) {
		source := readPumpSource(t, "logzio.go")
		writeData := sourceFrom(t, source,
			"func (p *LogzioPump) WriteData(ctx context.Context, data []interface{}) error",
		)
		require.Contains(t, writeData, "p.sender.Send(event)")
		require.NotContains(t, writeData, "return p.sender.Send")
		require.Contains(t, writeData, "return nil")
	})

	t.Run("segment WriteData ignores WriteDataRecord errors", func(t *testing.T) {
		source := readPumpSource(t, "segment.go")
		writeData := sourceBetween(t, source,
			"func (s *SegmentPump) WriteData(ctx context.Context, data []interface{}) error",
			"\n// reqproof:implements SW-REQ-053\nfunc (s *SegmentPump) WriteDataRecord",
		)
		writeDataRecord := sourceBetween(t, source,
			"func (s *SegmentPump) WriteDataRecord(record analytics.AnalyticsRecord) error",
			"\nfunc (s *SegmentPump) ToJSONMap",
		)
		require.Contains(t, writeData, "s.WriteDataRecord(v.(analytics.AnalyticsRecord))")
		require.Contains(t, writeData, "return nil")
		require.Contains(t, writeDataRecord, `s.log.Error("Couldn't track record:", err)`)
		require.Contains(t, writeDataRecord, "return nil")
	})

	t.Run("influx v1 batch write error is ignored", func(t *testing.T) {
		source := readPumpSource(t, "influx.go")
		writeData := sourceFrom(t, source,
			"func (i *InfluxPump) WriteData(ctx context.Context, data []interface{}) error",
		)
		require.Contains(t, writeData, "c.Write(bp)")
		require.NotContains(t, writeData, "err = c.Write")
		require.NotContains(t, writeData, "return c.Write")
		require.Contains(t, writeData, "return nil")
	})

	t.Run("influx v2 async write errors are never drained", func(t *testing.T) {
		source := readPumpSource(t, "influx2.go")
		writeData := sourceFrom(t, source,
			"func (i *Influx2Pump) WriteData(ctx context.Context, data []interface{}) error",
		)
		require.Contains(t, writeData, "writeApi.WritePoint(")
		require.Contains(t, writeData, "writeApi.Flush()")
		require.NotContains(t, writeData, ".Errors()")
		require.Contains(t, writeData, "return nil")
	})
}

// TestKinesisPutRecordsPerRecordFailuresReturnNil_KI is a static tripwire for
// the Kinesis successful-response/per-record-failure path. The production pump
// stores the concrete AWS client, so this pins the current branch without
// adding a product-only mock seam.
// Verifies: SW-REQ-056
// Verifies: KI:kinesis-putrecords-per-record-failures-return-nil
// Reproduces: kinesis-putrecords-per-record-failures-return-nil
func TestKinesisPutRecordsPerRecordFailuresReturnNil_KI(t *testing.T) {
	source := readPumpSource(t, "kinesis.go")
	start := strings.Index(source, "func (p *KinesisPump) WriteData(ctx context.Context, records []interface{}) error")
	require.NotEqual(t, -1, start, "Kinesis WriteData function not found")
	end := strings.Index(source[start:], "// splitIntoBatches splits")
	require.NotEqual(t, -1, end, "Kinesis WriteData end marker not found")
	writeData := source[start : start+end]

	require.Contains(t, writeData, "output, err := p.client.PutRecords(ctx, input)")
	require.Contains(t, writeData, "if record.ErrorCode != nil")
	require.Contains(t, writeData, `p.log.Debugf("Failed to put record: %s - %s"`)
	require.Contains(t, writeData, "return nil")
	require.NotContains(t, writeData, "FailedRecordCount")
	require.NotContains(t, writeData, "return err")
	require.NotContains(t, writeData, "return fmt.Errorf")
	require.NotContains(t, writeData, "return errors.New")
	require.Less(t,
		strings.Index(writeData, "if record.ErrorCode != nil"),
		strings.LastIndex(writeData, "return nil"),
		"per-record failure branch should precede final nil return",
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

// TestSQLFamilyBatchSizeNonPositive_KI is a static tripwire for the SQL-family
// batch-size KnownIssue. Executing a zero/negative stride can hang or panic, so
// the test pins the current source shape: Init defaulting only handles == 0,
// while write loops advance by BatchSize without <= 0 validation.
// Verifies: SW-REQ-040
// Verifies: SW-REQ-041
// Verifies: SW-REQ-042
// Verifies: SW-REQ-043
// Verifies: SW-REQ-044
// Verifies: SW-REQ-045
// Verifies: KI:sql-batch-size-zero-infinite-loop
// Reproduces: sql-batch-size-zero-infinite-loop
func TestSQLFamilyBatchSizeNonPositive_KI(t *testing.T) {
	cases := []struct {
		path      string
		batchExpr string
		loopExpr  string
	}{
		{path: "sql.go", batchExpr: `c\.SQLConf\.BatchSize`, loopExpr: `i \+= c\.SQLConf\.BatchSize`},
		{path: "sql_aggregate.go", batchExpr: `c\.SQLConf\.BatchSize`, loopExpr: `i \+= c\.SQLConf\.BatchSize`},
		{path: "graph_sql.go", batchExpr: `g\.Conf\.BatchSize`, loopExpr: `ri \+= g\.Conf\.BatchSize`},
		{path: "graph_sql_aggregate.go", batchExpr: `s\.SQLConf\.BatchSize`, loopExpr: `i \+= s\.SQLConf\.BatchSize`},
		{path: "mcp_sql.go", batchExpr: `g\.Conf\.BatchSize`, loopExpr: `ri \+= g\.Conf\.BatchSize`},
		{path: "mcp_sql_aggregate.go", batchExpr: `s\.SQLConf\.BatchSize`, loopExpr: `i \+= s\.SQLConf\.BatchSize`},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			source := readPumpSource(t, tc.path)

			require.Regexp(t, regexp.MustCompile(tc.batchExpr+`\s*==\s*0`), source)
			require.Regexp(t, regexp.MustCompile(tc.loopExpr), source)
			require.NotRegexp(t, regexp.MustCompile(tc.batchExpr+`\s*(<=\s*0|<\s*1)`), source)
		})
	}
}

// TestSQLMySQLCreateIndexIfNotExists_KI is a static tripwire for the SQL-family
// MySQL index DDL KnownIssues. It pins the current source shape where only
// CONCURRENTLY is Postgres-gated and IF NOT EXISTS remains in the DDL template
// used for non-Postgres paths.
// Verifies: SW-REQ-040
// Verifies: SW-REQ-041
// Verifies: SW-REQ-045
// Verifies: SW-REQ-066
// Verifies: INT-REQ-007
// Verifies: KI:sql-standard-mysql-create-index-if-not-exists-unsupported
// Verifies: KI:sql-aggregate-mysql-create-index-if-not-exists-unsupported
// Verifies: KI:mcp-sql-aggregate-mysql-create-index-syntax-broken
// Reproduces: sql-standard-mysql-create-index-if-not-exists-unsupported
// Reproduces: sql-aggregate-mysql-create-index-if-not-exists-unsupported
// Reproduces: mcp-sql-aggregate-mysql-create-index-syntax-broken
func TestSQLMySQLCreateIndexIfNotExists_KI(t *testing.T) {
	cases := []struct {
		path      string
		receiver  string
		ddlRegexp string
	}{
		{
			path:      "sql.go",
			receiver:  `c\.dbType == "postgres"`,
			ddlRegexp: `CREATE INDEX %s IF NOT EXISTS %s ON %s \(%s\)`,
		},
		{
			path:      "sql_aggregate.go",
			receiver:  `c\.dbType == "postgres"`,
			ddlRegexp: `CREATE INDEX %s IF NOT EXISTS %s ON %s \(dimension, timestamp, org_id, dimension_value\)`,
		},
		{
			path:      "mcp_sql_aggregate.go",
			receiver:  `s\.dbType == "postgres"`,
			ddlRegexp: `CREATE INDEX %s IF NOT EXISTS %s ON %s \(dimension, timestamp, org_id, dimension_value\)`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			source := readPumpSource(t, tc.path)

			require.Regexp(t, regexp.MustCompile(tc.receiver), source)
			require.Regexp(t, regexp.MustCompile(tc.ddlRegexp), source)
			require.NotRegexp(t,
				regexp.MustCompile(`dbType\s*==\s*"mysql"|Dialector\(\)\.Name\(\)\s*==\s*"mysql"`),
				source,
			)
		})
	}
}

// TestSQLAggregateMySQLExcludedKeyword_KI is a static tripwire for the
// aggregate-family MySQL upsert KnownIssue. It pins the shared helper's
// temp-table qualification and the three aggregate callers still passing the
// Postgres-specific "excluded" qualifier.
// Verifies: SW-REQ-041
// Verifies: SW-REQ-043
// Verifies: SW-REQ-045
// Verifies: KI:sql-aggregate-mysql-excluded-keyword-broken
// Reproduces: sql-aggregate-mysql-excluded-keyword-broken
func TestSQLAggregateMySQLExcludedKeyword_KI(t *testing.T) {
	helper := readPumpSource(t, "../analytics/aggregate.go")
	require.Regexp(t,
		regexp.MustCompile(`(?s)func OnConflictAssignments\(tableName, tempTable string\).*tempTable \+ "\." \+ colName`),
		helper,
	)

	for _, path := range []string{"sql_aggregate.go", "graph_sql_aggregate.go", "mcp_sql_aggregate.go"} {
		t.Run(path, func(t *testing.T) {
			source := readPumpSource(t, path)

			require.Contains(t, source, "clause.OnConflict")
			require.Contains(t, source, `analytics.OnConflictAssignments(table, "excluded")`)
			require.NotRegexp(t, regexp.MustCompile(`dbType\s*==\s*"mysql"|VALUES\(|clause\.Expr`), source)
		})
	}
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
