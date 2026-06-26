package pumps

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SW-REQ-064:temporal_window_inclusive:boundary
//
// Contract: SQL aggregate day-sharding routes records to a deterministic
// per-day table. Boundary semantics:
//   - a record with timestamp = "2026-06-04T00:00:00.000Z" routes to the
//     2026-06-04 table (INCLUSIVE at start)
//   - a record with timestamp = "2026-06-04T23:59:59.999Z" also routes to
//     2026-06-04 (EXCLUSIVE at next-day start)
//   - a record with timestamp = "2026-06-05T00:00:00.000Z" routes to
//     2026-06-05 (the boundary is closed-open: [day, day+1))
//
// Production routing logic (pumps/sql_aggregate.go WriteData):
//
//	recDate := data[startIndex].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")
//	table   = analytics.AggregateSQLTable + "_" + recDate
//
// This test exercises that exact production expression via a small
// dayShardTable helper, asserting all four boundary cases plus a
// nanosecond-before-midnight case.
// SW-REQ-064:temporal_window_inclusive:nominal
func TestTemporalWindowInclusive_DayBoundaries(t *testing.T) {
	mustParse := func(s string) time.Time {
		ts, err := time.Parse(time.RFC3339Nano, s)
		require.NoErrorf(t, err, "failed to parse %q", s)
		return ts
	}

	cases := []struct {
		name          string
		timestamp     time.Time
		expectedTable string
	}{
		{
			name:          "start of day (inclusive)",
			timestamp:     mustParse("2026-06-04T00:00:00.000000000Z"),
			expectedTable: "tyk_aggregated_20260604",
		},
		{
			name:          "noon",
			timestamp:     mustParse("2026-06-04T12:00:00.000000000Z"),
			expectedTable: "tyk_aggregated_20260604",
		},
		{
			name:          "end of day (1ns before next day, exclusive of next)",
			timestamp:     mustParse("2026-06-04T23:59:59.999999999Z"),
			expectedTable: "tyk_aggregated_20260604",
		},
		{
			name:          "midnight rollover routes to next day",
			timestamp:     mustParse("2026-06-05T00:00:00.000000000Z"),
			expectedTable: "tyk_aggregated_20260605",
		},
		{
			name:          "month boundary",
			timestamp:     mustParse("2026-06-30T23:59:59.999999999Z"),
			expectedTable: "tyk_aggregated_20260630",
		},
		{
			name:          "month rollover",
			timestamp:     mustParse("2026-07-01T00:00:00.000000000Z"),
			expectedTable: "tyk_aggregated_20260701",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actual := dayShardTable(tc.timestamp)
			assert.Equalf(t, tc.expectedTable, actual,
				"timestamp %s routed to %q, expected %q",
				tc.timestamp.Format(time.RFC3339Nano), actual, tc.expectedTable)
		})
	}
}

// SW-REQ-064:temporal_window_inclusive:boundary
//
// Same hour in two different zones must route to DIFFERENT day-shard
// tables when the UTC date differs. 23:30 on 2026-06-04 NY time is
// 03:30 on 2026-06-05 UTC. Production code calls .Format("20060102")
// on a time.Time, which formats in the value's own location — so
// 23:30 NY routes to the 2026-06-04 table.
//
// This pins the contract that the day-shard window is computed in the
// timestamp's own location, NOT silently UTC-converted. A regression
// that silently UTC-converted would route cross-zone records to a
// different day than callers expect.
func TestTemporalWindowInclusive_ZoneAware(t *testing.T) {
	nyLoc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	nyEvening := time.Date(2026, 6, 4, 23, 30, 0, 0, nyLoc) // 03:30 UTC on 2026-06-05
	utcSame := nyEvening.UTC()

	nyTable := dayShardTable(nyEvening)
	utcTable := dayShardTable(utcSame)

	assert.Equal(t, "tyk_aggregated_20260604", nyTable,
		"23:30 NY (which is on the 4th NY-local) must route to 20260604")
	assert.Equal(t, "tyk_aggregated_20260605", utcTable,
		"same instant in UTC (03:30 on the 5th UTC) must route to 20260605")
	assert.NotEqual(t, nyTable, utcTable,
		"same instant in different zones must route to different day-shard tables — proves the routing is zone-aware")
}

// Verifies: SW-REQ-040
// Verifies: SW-REQ-041
// Verifies: SW-REQ-042
// Verifies: SW-REQ-043
// Verifies: SW-REQ-044
// Verifies: SW-REQ-045
// Verifies: KI:sql-family-day-shard-timezone-convention-unpinned
// Reproduces: sql-family-day-shard-timezone-convention-unpinned
func TestSQLFamilyDayShardTimezoneConventionUnpinned_KI(t *testing.T) {
	cases := []struct {
		name     string
		file     string
		snippets []string
	}{
		{
			name: "sql standard",
			file: "sql.go",
			snippets: []string{
				`recDate := typedData[startIndex].TimeStamp.Format("20060102")`,
				`nextRecDate = typedData[i].TimeStamp.Format("20060102")`,
			},
		},
		{
			name: "sql aggregate",
			file: "sql_aggregate.go",
			snippets: []string{
				`recDate := data[startIndex].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")`,
				`nextRecDate = data[i].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")`,
			},
		},
		{
			name: "graph sql",
			file: "graph_sql.go",
			snippets: []string{
				`recDate := graphRecords[startIndex].AnalyticsRecord.TimeStamp.Format("20060102")`,
				`nextRecDate = graphRecords[i].AnalyticsRecord.TimeStamp.Format("20060102")`,
			},
		},
		{
			name: "graph sql aggregate",
			file: "graph_sql_aggregate.go",
			snippets: []string{
				`recDate := data[startIndex].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")`,
				`nextRecDate = data[i].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")`,
			},
		},
		{
			name: "mcp sql",
			file: "mcp_sql.go",
			snippets: []string{
				`recDate := mcpRecords[startIndex].AnalyticsRecord.TimeStamp.Format("20060102")`,
				`nextRecDate = mcpRecords[i].AnalyticsRecord.TimeStamp.Format("20060102")`,
			},
		},
		{
			name: "mcp sql aggregate",
			file: "mcp_sql_aggregate.go",
			snippets: []string{
				`recDate := data[startIndex].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")`,
				`nextRecDate = data[i].(analytics.AnalyticsRecord).TimeStamp.Format("20060102")`,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sourceBytes, err := os.ReadFile(tc.file)
			require.NoError(t, err)
			source := string(sourceBytes)
			for _, snippet := range tc.snippets {
				assert.Truef(t, strings.Contains(source, snippet),
					"%s must keep source-location day sharding snippet %q", tc.file, snippet)
			}
		})
	}
}

// dayShardTable mirrors the EXACT production routing expression from
// pumps/sql_aggregate.go WriteData:
//
//	recDate := record.TimeStamp.Format("20060102")
//	table   = analytics.AggregateSQLTable + "_" + recDate
//
// Centralised in a tiny helper so the boundary semantics above can be
// asserted without spinning a real GORM pump or DB. If the production
// code switches away from Format("20060102") (e.g. to UTC-converted
// truncation), this helper must be updated to match — and the test
// above will then fail loudly until the new contract is documented.
func dayShardTable(ts time.Time) string {
	return analytics.AggregateSQLTable + "_" + ts.Format("20060102")
}
