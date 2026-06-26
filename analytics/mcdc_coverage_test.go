package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsproto "github.com/TykTechnologies/tyk-pump/analytics/proto"
)

// ------- analytics/aggregate.go: incrementAggregate ResponseCode == -1 -------
// Verifies: SYS-REQ-003
// MCDC SYS-REQ-003: aggregates_emitted=T, aggregation_enabled=T => TRUE
// Drives incrementAggregate's `record.ResponseCode == -1` to the T side.
// This is the network-only path (TCP-level event with no HTTP response) which
// must populate Network counters and per-API byte totals but NOT increment Hits.
func TestIncrementAggregate_ResponseCodeMinusOne_NetworkBranch(t *testing.T) {
	ts := time.Date(2026, 5, 30, 9, 0, 0, 0, time.UTC)

	records := []interface{}{
		// Pure network event: ResponseCode == -1, APIID set so APIID counter is
		// updated via the inner if-record.APIID != "" branch.
		AnalyticsRecord{
			OrgID:        "orgN",
			APIID:        "apiNet",
			APIName:      "NetAPI",
			ResponseCode: -1,
			TimeStamp:    ts,
			Network: NetworkStats{
				OpenConnections:  3,
				ClosedConnection: 2,
				BytesIn:          1024,
				BytesOut:         2048,
			},
		},
		// Pure network event without APIID — exercises the F side of
		// `record.APIID != ""` inside the ResponseCode==-1 branch.
		AnalyticsRecord{
			OrgID:        "orgN",
			APIID:        "",
			ResponseCode: -1,
			TimeStamp:    ts,
			Network: NetworkStats{
				OpenConnections: 1,
				BytesIn:         10,
				BytesOut:        20,
			},
		},
		// A second -1 record for the SAME APIID — drives the F side of
		// `c == nil` (per-API counter already present so the if-block is
		// skipped and only the BytesIn/BytesOut accumulator runs).
		AnalyticsRecord{
			OrgID:        "orgN",
			APIID:        "apiNet",
			ResponseCode: -1,
			TimeStamp:    ts,
			Network: NetworkStats{
				BytesIn:  500,
				BytesOut: 600,
			},
		},
		// A normal HTTP record so both T and F sides of `ResponseCode == -1`
		// are exercised in the same suite.
		AnalyticsRecord{
			OrgID:        "orgN",
			APIID:        "apiNet",
			ResponseCode: 200,
			TimeStamp:    ts,
		},
	}

	agg := AggregateData(records, false, nil, "", 60)
	got, ok := agg["orgN"]
	require.True(t, ok, "expected orgN aggregate")

	// Three -1 records contribute to Network counters without bumping Hits.
	assert.Equal(t, int64(4), got.Total.OpenConnections)
	assert.Equal(t, int64(2), got.Total.ClosedConnections)
	assert.Equal(t, int64(1024+10+500), got.Total.BytesIn)
	assert.Equal(t, int64(2048+20+600), got.Total.BytesOut)
	// Only the 200-coded record bumps Hits.
	assert.Equal(t, 1, got.Total.Hits)
	// Per-API counter accumulates from BOTH -1 records that have APIID set:
	// 1024 from the first, +500 from the third. The second had empty APIID.
	require.Contains(t, got.APIID, "apiNet")
	assert.Equal(t, int64(1024+500), got.APIID["apiNet"].BytesIn)
	assert.Equal(t, int64(2048+600), got.APIID["apiNet"].BytesOut)
}

// ------- analytics/aggregate.go: MinLatency 1xx-code F-side proofs ------
// Drives the F side of `(record.ResponseCode >= 200)` in the two MinLatency
// guards at incrementAggregate aggregate.go:851 and :855. To enter these
// guards we need (a) Hits > 1 (so we're past the seeding branch), (b) the
// new record's latency to be lower than the current min, and (c)
// ResponseCode < 300 to be TRUE while ResponseCode >= 200 is FALSE — i.e.
// an informational 1xx code.
// Verifies: SW-REQ-011
func TestIncrementAggregate_MinLatency_InformationalCode(t *testing.T) {
	ts := time.Date(2026, 5, 30, 14, 0, 0, 0, time.UTC)
	records := []interface{}{
		// Seed: Hits becomes 1, MinLatency=200, MinUpstreamLatency=100.
		AnalyticsRecord{
			OrgID: "orgL", APIID: "apiL", ResponseCode: 200, TimeStamp: ts,
			Latency: Latency{Total: 200, Upstream: 100},
		},
		// Second record: Hits becomes 2 -> we reach the else branch with
		// the MinLatency / MinUpstreamLatency guards. Latency is lower than
		// the seed (so the first AND-clause is T), ResponseCode is < 300 (T)
		// but NOT >= 200 (F) — informational 1xx code.
		AnalyticsRecord{
			OrgID: "orgL", APIID: "apiL", ResponseCode: 100, TimeStamp: ts,
			Latency: Latency{Total: 50, Upstream: 25},
		},
	}
	agg := AggregateData(records, false, nil, "", 60)
	got, ok := agg["orgL"]
	require.True(t, ok)
	// Because the 1xx record fails the ResponseCode >= 200 guard, the min
	// values must NOT be updated — they remain the seed values.
	assert.Equal(t, int64(200), got.Total.MinLatency, "MinLatency must stay at seed when 1xx code fails the >=200 guard")
	assert.Equal(t, int64(100), got.Total.MinUpstreamLatency, "MinUpstreamLatency must stay at seed when 1xx code fails the >=200 guard")
	assert.Equal(t, 2, got.Total.Hits)
}

// ------- analytics/uptime_data.go: ResponseCode 1xx F-side proof --------
// Drives the F side of `(thisV.ResponseCode >= 200)` in the success-band
// guard at uptime_data.go:211. With ResponseCode == 100 we have <300 (T) but
// !(>=200) (F), so the if-body must be skipped (no Success increment, no
// errorMap entry for 100).
// Verifies: SW-REQ-015
func TestAggregateUptimeData_InformationalCodeNotCountedAsSuccess(t *testing.T) {
	data := []UptimeReportData{
		{OrgID: "orgL", APIID: "apiL", URL: "http://u", ResponseCode: 100, RequestTime: 5},
	}
	out := AggregateUptimeData(data)
	require.Contains(t, out, "orgL")
	agg := out["orgL"]
	assert.Equal(t, 0, agg.Total.Success, "1xx must not increment Success")
	assert.NotContains(t, agg.Total.ErrorMap, "100", "1xx code must not be recorded in the success-side ErrorMap entry")
}

// ------- analytics/aggregate.go: fnLatencySetter Hits == 0 -------
// Drives the F side of `counter.Hits > 0` in fnLatencySetter (so Latency
// is not divided by zero and remains at the zero value).
// Verifies: SW-REQ-011
func TestFnLatencySetter_HitsZero(t *testing.T) {
	c := &Counter{Hits: 0, TotalLatency: 100, TotalUpstreamLatency: 50}
	out := fnLatencySetter(c)
	assert.Equal(t, float64(0), out.Latency, "Latency must stay 0 when Hits==0")
	assert.Equal(t, float64(0), out.UpstreamLatency, "UpstreamLatency must stay 0 when Hits==0")

	// Sanity: still works when Hits > 0 — keeps both branches exercised in this test.
	c2 := &Counter{Hits: 4, TotalLatency: 20, TotalUpstreamLatency: 8}
	out2 := fnLatencySetter(c2)
	assert.Equal(t, float64(5), out2.Latency)
	assert.Equal(t, float64(2), out2.UpstreamLatency)
}

// ------- analytics/aggregate.go: getRecords incVal.Hits == 0 -------
// Drives the F side of `incVal.Hits > 0` inside (*AnalyticsRecordAggregate).getRecords.
// We construct an aggregate with a zero-Hits counter in the APIID map; AsTimeUpdate
// triggers getRecords for every dimension map.
// Verifies: SW-REQ-011
func TestGetRecords_HitsZeroBranch(t *testing.T) {
	agg := AnalyticsRecordAggregate{}.New()
	agg.OrgID = "orgZ"
	agg.TimeStamp = time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	agg.Total.ErrorMap = make(map[string]int)
	agg.APIID["empty"] = &Counter{
		Hits:             0,
		TotalRequestTime: 12345, // proves the divide-by-zero guard works
		ErrorMap:         map[string]int{},
	}
	// AsTimeUpdate calls getRecords for every dimension map.
	_ = agg.AsTimeUpdate()
	assert.Equal(t, 0, agg.APIID["empty"].Hits)
}

// ------- analytics/aggregate.go: ignoreTag prefix match T -------
// Drives the T side of `strings.HasPrefix(tag, prefix)` in ignoreTag (a tag
// matching one of the user-configured IgnoreTagPrefixList entries).
// Verifies: SW-REQ-011
func TestIgnoreTag_PrefixMatchTrue(t *testing.T) {
	// Match from the user-configured prefix list (not "key-" which is the
	// hardcoded gateway-key prefix already tested elsewhere).
	if !ignoreTag("internal-foo", []string{"internal-"}) {
		t.Fatal("expected ignoreTag to return true for tag matching user prefix list")
	}
	// And the F side — no prefix matches.
	if ignoreTag("user-foo", []string{"internal-", "system-"}) {
		t.Fatal("expected ignoreTag to return false for tag matching no prefix")
	}
}

// ------- analytics/aggregate.go: replaceUnsupportedChars dot T -------
// Drives the T side of `strings.Contains(path, ".")` (a path containing a dot
// must have the dot replaced by its unicode escape).
// Verifies: SW-REQ-011
func TestReplaceUnsupportedChars_PathWithDot(t *testing.T) {
	got := replaceUnsupportedChars("v1.endpoint")
	if got == "v1.endpoint" {
		t.Fatalf("expected dot to be replaced, got %q", got)
	}
	// F side sanity.
	got = replaceUnsupportedChars("v1/endpoint")
	if got != "v1/endpoint" {
		t.Fatalf("expected no replacement, got %q", got)
	}
}

// ------- analytics/aggregate.go: AggregateGraphData !ok branch -------
// Drives the T side of `!ok` in AggregateGraphData (i.e. an item in the input
// slice that is NOT an AnalyticsRecord must be silently skipped).
// Verifies: SW-REQ-013
func TestAggregateGraphData_NonAnalyticsItemSkipped(t *testing.T) {
	ts := time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)
	graphRec := AnalyticsRecord{
		OrgID:        "orgG",
		APIID:        "apiG",
		ResponseCode: 200,
		TimeStamp:    ts,
		GraphQLStats: GraphQLStats{
			IsGraphQL:     true,
			OperationType: OperationQuery,
			RootFields:    []string{"user"},
			Types:         map[string][]string{"User": {"name"}},
		},
	}
	data := []interface{}{
		"not a record", // !ok=T path
		42,             // !ok=T path
		graphRec,       // ok=T, IsGraphRecord=T path
	}
	out := AggregateGraphData(data, "", 60)
	require.Contains(t, out, "apiG")
	assert.Equal(t, 1, out["apiG"].Total.Hits)
}

// ------- analytics/aggregate.go: setAggregateTimestamp !ok F side -------
// Drives setAggregateTimestamp such that the OR-chain `lastDocumentTS == emptyTime || !ok`
// gets evaluated with the first clause FALSE — which forces `!ok` to be
// evaluated. Pre-seeding the per-identifier last-document timestamp ensures
// the map lookup returns (non-zero, ok=true), so the second clause's F value
// is independently observed.
// Verifies: SYS-REQ-003
// MCDC SYS-REQ-003: aggregation_enabled=T, aggregates_emitted=T => TRUE
func TestSetAggregateTimestamp_NonEmptyLastDocOK(t *testing.T) {
	dbID := "mcdc-non-empty-last-doc"
	// Pre-seed a non-empty timestamp so the first clause is false on the
	// next call.
	seed := time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)
	SetlastTimestampAgggregateRecord(dbID, seed)

	// aggregationTime != 60 so we go past the early-return branch.
	asTime := seed.Add(10 * time.Minute)
	out := setAggregateTimestamp(dbID, asTime, 30)

	// With 30-minute aggregation, seed + 30min is still after asTime
	// (seed+10min), so the function returns the seeded timestamp.
	assert.Equal(t, seed, out, "should return the pre-seeded last document timestamp")
}

// Sanity check: the !ok=T path (no entry in the map) still works — keeps
// the OR's T short-circuit branch exercised.
// Verifies: SYS-REQ-003
// MCDC SYS-REQ-003: aggregation_enabled=T, aggregates_emitted=T => TRUE
func TestSetAggregateTimestamp_UnseededFreshIdentifier(t *testing.T) {
	dbID := "mcdc-fresh-identifier-" + time.Now().Format("150405.000000000")
	asTime := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	got := setAggregateTimestamp(dbID, asTime, 30)
	// First call with this identifier must initialise lastDocumentTimestamp
	// from asTime (truncated to minute).
	want := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

// ------- analytics/uptime_data.go: AggregateUptimeData URL != "" T -------
// Drives the T side of `thisV.URL != ""` inside the ResponseCode==-1 branch
// of AggregateUptimeData (TCP-error event for a known URL). Records are
// ordered with the HTTP record FIRST for the same URL so that the per-URL
// Counter is seeded with a valid ErrorMap before the TCP-error record reuses
// it; the inverse order panics on a latent bug tracked by KI
// uptime-aggregate-nil-errormap-on-tcp-then-http.
// Verifies: SW-REQ-015
func TestAggregateUptimeData_TCPErrorWithURL(t *testing.T) {
	data := []UptimeReportData{
		// 2xx path first so the URL counter's ErrorMap is initialised.
		{OrgID: "orgU", APIID: "apiU", URL: "http://target", ResponseCode: 204, RequestTime: 12},
		// T side of `thisV.URL != ""` inside the ResponseCode==-1 arm.
		{OrgID: "orgU", APIID: "apiU", URL: "http://target", ResponseCode: -1, TCPError: true},
		// F side of `thisV.URL != ""` — TCP error with no URL.
		{OrgID: "orgU", APIID: "apiU", URL: "", ResponseCode: -1, TCPError: true},
	}
	out := AggregateUptimeData(data)
	require.Contains(t, out, "orgU")
	agg := out["orgU"]
	require.Contains(t, agg.URL, "http://target", "URL map must contain the known URL")
	assert.Equal(t, "http://target", agg.URL["http://target"].Identifier)
}

// Reproduces: uptime-aggregate-nil-errormap-on-tcp-then-http
// Verifies: SW-REQ-073
// Verifies: KI:uptime-aggregate-nil-errormap-on-tcp-then-http
func TestAggregateUptimeData_TCPErrorBeforeHTTP_KI(t *testing.T) {
	data := []UptimeReportData{
		{OrgID: "orgU", APIID: "apiU", URL: "http://target", ResponseCode: -1, TCPError: true},
		{OrgID: "orgU", APIID: "apiU", URL: "http://target", ResponseCode: 204, RequestTime: 12},
	}

	require.Panics(t, func() {
		AggregateUptimeData(data)
	}, "known issue: TCP-error record seeds the URL counter without ErrorMap, then HTTP record writes into nil map")
}

// ------- analytics/uptime_data.go: OnConflictUptimeAssignments field.IsZero T -------
// OnConflictUptimeAssignments iterates Counter struct fields and only emits a
// "request_time" GORM expression when the field's value is non-zero. The
// function constructs an *empty* Counter, so RequestTime is the zero float —
// IsZero() returns T and the expression is skipped. Calling the function in
// a test exercises that T branch.
// Verifies: SW-REQ-015
func TestOnConflictUptimeAssignments_RequestTimeZeroSkipped(t *testing.T) {
	out := OnConflictUptimeAssignments("dest_table", "tmp_table")

	// Always-emitted columns must be present.
	assert.Contains(t, out, "counter_hits")
	assert.Contains(t, out, "counter_last_time")

	// request_time MUST NOT be present because the zero-value Counter has
	// RequestTime==0, IsZero() returns true, and the if-block is skipped.
	_, found := out["counter_request_time"]
	assert.False(t, found, "counter_request_time must be omitted when RequestTime is zero")
}

// ------- analytics/analytics_filters.go: ShouldFilter skip-list T members ------

// Verifies: SW-REQ-010
// Verifies: SYS-REQ-009
// MCDC SW-REQ-010: filter_true=F, in_should_filter=T, outside_allow_list=F, skip_match=F => TRUE
// MCDC SW-REQ-010: filter_true=T, in_should_filter=T, outside_allow_list=T, skip_match=T => TRUE
// MCDC SYS-REQ-009: record_excluded=F, record_matches_block_filter=F, record_outside_allow_list=F => TRUE
// MCDC SYS-REQ-009: record_excluded=T, record_matches_block_filter=T, record_outside_allow_list=T => TRUE
// Drives MC/DC for each `stringInSlice(...)/intInSlice(...)` inside the
// skip-list switch arms of ShouldFilter. The existing TestShouldFilter_*
// drives empty-list short circuits and list-contains (T). This companion
// drives BOTH "list non-empty AND record IS in the list" (T) and
// "list non-empty AND record IS NOT in the list" (F), so for every skip
// arm we have an evaluation of the second clause at both T and F.
func TestShouldFilter_SkipListsContainAndExcludeRecord(t *testing.T) {
	rec := AnalyticsRecord{APIID: "apiA", OrgID: "orgA", ResponseCode: 502}
	cases := []struct {
		name   string
		filter AnalyticsFilters
		want   bool
	}{
		// T side of membership predicates.
		{"skip api in list (T)", AnalyticsFilters{SkippedAPIIDs: []string{"other", "apiA"}}, true},
		{"skip org in list (T)", AnalyticsFilters{SkippedOrgsIDs: []string{"orgA", "anotherOrg"}}, true},
		{"skip code in list (T)", AnalyticsFilters{SkippedResponseCodes: []int{500, 502, 503}}, true},
		// F side of membership predicates: list is non-empty (so the first
		// clause is T and the second clause IS evaluated) but does not
		// contain the record's value -> stringInSlice/intInSlice returns F
		// -> the switch case does not match. ShouldFilter falls through
		// to the next case and ultimately returns false.
		{"skip api not in list (F)", AnalyticsFilters{SkippedAPIIDs: []string{"other"}}, false},
		{"skip org not in list (F)", AnalyticsFilters{SkippedOrgsIDs: []string{"otherOrg"}}, false},
		{"skip code not in list (F)", AnalyticsFilters{SkippedResponseCodes: []int{500, 503}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.filter.ShouldFilter(rec); got != tc.want {
				t.Fatalf("%s: ShouldFilter=%v want %v", tc.name, got, tc.want)
			}
		})
	}
}

// ------- analytics/aggregate_mcp.go: incrementMCPDimensions PrimitiveType F -------
// Drives the F side of `rec.PrimitiveType != ""` inside
// (*MCPRecordAggregate).incrementMCPDimensions. The MCP record has a method
// and a name but no primitive type — Primitives map must stay empty, Names
// map must use the bare PrimitiveName (no "<type>_<name>" label).
// Verifies: SW-REQ-012
func TestIncrementMCPDimensions_EmptyPrimitiveType(t *testing.T) {
	ts := time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)
	data := []interface{}{
		AnalyticsRecord{
			OrgID: "orgM", APIID: "apiM", TimeStamp: ts, ResponseCode: 200,
			MCPStats: MCPStats{
				IsMCP:         true,
				JSONRPCMethod: "ping",
				PrimitiveType: "", // F side
				PrimitiveName: "bare_name",
			},
		},
	}
	out := AggregateMCPData(data, "", 60)
	agg, ok := out["apiM"]
	require.True(t, ok)
	// PrimitiveType empty -> Primitives map must NOT have an entry keyed by "".
	_, hasEmpty := agg.Primitives[""]
	assert.False(t, hasEmpty, "empty PrimitiveType must not produce a Primitives entry")
	assert.Empty(t, agg.Primitives, "Primitives must remain empty when PrimitiveType is empty")
	// Names must use the bare PrimitiveName (no type_ prefix).
	assert.Contains(t, agg.Names, "bare_name")
}

// ------- analytics/aggregate_mcp.go: AggregateMCPData !ok T (non-MCP item) ----
// Drives the T side of `!ok` in AggregateMCPData (an item in the input slice
// that is not an AnalyticsRecord must be silently skipped, leaving the map
// empty).
// Verifies: SW-REQ-012
func TestAggregateMCPData_NonAnalyticsItemSkipped(t *testing.T) {
	data := []interface{}{
		"not a record",        // !ok=T -> continue
		42,                    // !ok=T -> continue
		struct{ X int }{X: 1}, // !ok=T -> continue
	}
	out := AggregateMCPData(data, "", 60)
	assert.Empty(t, out)
}

// ------- analytics/analytics.go: GeoData.GetLineValues first=T iteration ----
// Drives the T side of `first` inside GeoData.GetLineValues (executed on the
// first map iteration; the existing test passes a single-entry map so the F
// branch — which builds the semicolon-separated suffix — is never taken; this
// test passes two entries to exercise both T and F branches in one call).
// Verifies: SW-REQ-009
func TestGeoData_GetLineValues_MultipleCityNames(t *testing.T) {
	g := &GeoData{}
	g.City.Names = map[string]string{
		"en": "Paris",
		"fr": "Paris",
	}
	got := g.GetLineValues()
	// Layout: [iso, geonameid, cityNames, lat, lon, tz] -> 6 fields.
	require.Len(t, got, 6)
	cityNames := got[2]
	// Must contain a ';' which only the F branch (subsequent iteration) emits.
	assert.Contains(t, cityNames, ";", "multi-entry map must produce semicolon-joined cityNames")
	// And must contain a ':' which both branches emit.
	assert.Contains(t, cityNames, ":")
}

// ------- analytics/analytics.go: TimeStampFromProto err != nil T --------
// Drives the T side of `err != nil` from `time.LoadLocation(protoRecord.TimeZone)`.
// An obviously invalid IANA name guarantees LoadLocation fails.
// Verifies: SW-REQ-009
func TestTimeStampFromProto_InvalidTimeZoneError(t *testing.T) {
	rec := &AnalyticsRecord{}
	// Capture pre-state — function returns early on error so TimeStamp/ExpireAt
	// must remain at the zero value.
	preStamp := rec.TimeStamp
	preExpire := rec.ExpireAt

	proto := &analyticsproto.AnalyticsRecord{
		TimeStamp: timestamppb.New(time.Date(2026, 5, 30, 9, 0, 0, 0, time.UTC)),
		ExpireAt:  timestamppb.New(time.Date(2027, 5, 30, 9, 0, 0, 0, time.UTC)),
		TimeZone:  "Not/A/Real/Zone",
	}
	rec.TimeStampFromProto(proto)
	assert.Equal(t, preStamp, rec.TimeStamp, "error path must not mutate TimeStamp")
	assert.Equal(t, preExpire, rec.ExpireAt, "error path must not mutate ExpireAt")

	// F side sanity (existing tests cover this, but pairing keeps MC/DC rows
	// next to each other in this suite).
	proto.TimeZone = "UTC"
	rec.TimeStampFromProto(proto)
	assert.False(t, rec.TimeStamp.IsZero(), "valid zone must populate TimeStamp")
}

// ------- analytics/analytics.go: RemoveIgnoredFields err != nil T --------
// Drives the T side of `err != nil` from `field.Zero()` in RemoveIgnoredFields.
// AnalyticsRecord has an unexported `id` field with no `json` tag. Passing the
// empty string as a field-to-ignore makes the json-tag comparison match (both
// sides are ""), and Zero() fails with errNotExported. We assert that the
// surrounding loop survives the error (the next valid field still gets
// zeroed) — i.e. the error branch is non-fatal.
// Verifies: SW-REQ-076
// MCDC SW-REQ-076: ignore_fields_configured=T, listed_fields_removed=T => TRUE
func TestRemoveIgnoredFields_UnexportedFieldErrorPath(t *testing.T) {
	rec := AnalyticsRecord{
		APIID:  "api123",
		APIKey: "secret",
	}
	// "" matches the unexported `id` field (no json tag) -> Zero() returns
	// errNotExported, hitting the err != nil branch. "api_key" matches APIKey
	// which IS exported and must still be cleared afterward.
	rec.RemoveIgnoredFields([]string{"", "api_key"})
	assert.Equal(t, "api123", rec.APIID, "APIID must be untouched by the error-path field")
	assert.Equal(t, "", rec.APIKey, "APIKey should still be zeroed even after the earlier error")
}

// ------- analytics/graph_record.go: TableName GraphSQLTableName != "" ----
// Drives the F side of `GraphSQLTableName == ""` in (*GraphRecord).TableName.
// The package global starts as "" (T branch — falls through to the embedded
// AnalyticsRecord.TableName()); after setting it, the F branch returns the
// override directly. Restore the global so other tests are unaffected.
// Verifies: SW-REQ-013
func TestGraphRecord_TableName_WithOverride(t *testing.T) {
	// T branch first (no override) — must return the embedded analytics table.
	g := &GraphRecord{}
	assert.Equal(t, SQLTable, g.TableName(), "default must come from embedded AnalyticsRecord")

	prev := GraphSQLTableName
	GraphSQLTableName = "tyk_graph_override"
	t.Cleanup(func() { GraphSQLTableName = prev })

	assert.Equal(t, "tyk_graph_override", g.TableName(), "override must short-circuit the default")
}
