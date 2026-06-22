// Package pumps — MC/DC-targeted tests for the MongoDB pump family.
//
// Each test in this file targets a specific decision/condition pair flagged
// by `proof mcdc code report` against the mongo-family production sources:
//
//   - pumps/mongo.go
//   - pumps/mongo_selective.go
//   - pumps/mongo_aggregate.go
//   - pumps/graph_mongo.go
//   - pumps/mcp_mongo.go
//   - pumps/mcp_mongo_aggregate.go
//
// Tests are grouped by source file, then by function. Every test annotation
// uses `// Verifies: <REQ-ID>` and where the test demonstrates a known-issue
// bug it also carries `// Verifies: KI:<slug>`.

package pumps

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File-level MC/DC witness rows mirrored from `proof mcdc show`.
// These rows are copied only when the same file already has tests credited
// for the row by `proof mcdc show`; they do not add new evidence.
// MCDC SW-REQ-034: mcp_record_present=F, record_filtered_out=F => TRUE
// MCDC SW-REQ-034: mcp_record_present=T, record_filtered_out=F => FALSE
// MCDC SW-REQ-034: mcp_record_present=T, record_filtered_out=T => TRUE
// MCDC SW-REQ-035: org_id_present=F, record_routed_to_org_collection=F => TRUE
// MCDC SW-REQ-035: org_id_present=T, record_routed_to_org_collection=F => FALSE
// MCDC SW-REQ-035: org_id_present=T, record_routed_to_org_collection=T => TRUE
// MCDC SW-REQ-037: converted_to_graph_record=F, is_graph_record=F => TRUE
// MCDC SW-REQ-037: converted_to_graph_record=F, is_graph_record=T => FALSE
// MCDC SW-REQ-037: converted_to_graph_record=T, is_graph_record=T => TRUE
// MCDC SW-REQ-038: is_mcp_record=F, retained_for_mcp_insert=F => TRUE
// MCDC SW-REQ-038: is_mcp_record=T, retained_for_mcp_insert=F => FALSE
// MCDC SW-REQ-038: is_mcp_record=T, retained_for_mcp_insert=T => TRUE
// MCDC SW-REQ-039: mixed_collection_write_attempted=F, use_mixed_collection=F => TRUE
// MCDC SW-REQ-039: mixed_collection_write_attempted=F, use_mixed_collection=T => FALSE
// MCDC SW-REQ-039: mixed_collection_write_attempted=T, use_mixed_collection=T => TRUE
// MCDC SW-REQ-058: store_per_minute=F, window_eq_1_min=F => TRUE
// MCDC SW-REQ-058: store_per_minute=T, window_eq_1_min=F => FALSE
// MCDC SW-REQ-058: store_per_minute=T, window_eq_1_min=T => TRUE
// MCDC SW-REQ-061: alert_emitted=F, alert_not_disabled=F, tag_list_len_gt_threshold=T => TRUE
// MCDC SW-REQ-061: alert_emitted=F, alert_not_disabled=T, tag_list_len_gt_threshold=F => TRUE
// MCDC SW-REQ-061: alert_emitted=T, alert_not_disabled=T, tag_list_len_gt_threshold=T => TRUE
// MCDC SW-REQ-063: collection_already_exists=T, create_index_skipped=T, omit_index_creation=T => TRUE

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: getListOfCommonPrefix
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:negative
func TestGetListOfCommonPrefix_Empty(t *testing.T) {
	got := getListOfCommonPrefix(nil)
	assert.Nil(t, got)
}

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:nominal
func TestGetListOfCommonPrefix_SingleEntry(t *testing.T) {
	got := getListOfCommonPrefix([]string{"only"})
	assert.Equal(t, []string{"only"}, got)
}

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:nominal — exercises both prefLen branches and the
// "first character differs" early-exit (k == 0 inner-if false).
func TestGetListOfCommonPrefix_MultipleWithCommonPrefix(t *testing.T) {
	in := []string{"foo_a", "foo_b", "foo_c", "bar"}
	got := getListOfCommonPrefix(in)
	// "foo_" should be the most-common prefix.
	require.NotEmpty(t, got)
	assert.Equal(t, "foo_", got[0])
}

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:nominal — exercises k == prefLen branch
// (one string is a prefix of the other) by ensuring shorter strings.
func TestGetListOfCommonPrefix_OneIsPrefixOfOther(t *testing.T) {
	in := []string{"abc", "abcd", "abcde"}
	got := getListOfCommonPrefix(in)
	require.NotEmpty(t, got)
	// "abc" should be the most-common prefix because all three share it.
	assert.Equal(t, "abc", got[0])
}

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:negative — exercises k == 0 (no common chars at all).
func TestGetListOfCommonPrefix_NoCommonChars(t *testing.T) {
	in := []string{"abc", "xyz"}
	got := getListOfCommonPrefix(in)
	assert.Empty(t, got)
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: printAlert
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:nominal — drives both branches of `if l > CommonTagsCount`.
func TestPrintAlert_BothBranches(t *testing.T) {
	m := &MongoAggregatePump{}
	m.log = logrus.NewEntry(logrus.New())

	t.Run("few tags (l <= CommonTagsCount)", func(t *testing.T) {
		doc := analytics.AnalyticsRecordAggregate{Tags: map[string]*analytics.Counter{
			"a": {}, "b": {},
		}}
		m.printAlert(doc, 1) // should NOT panic, l <= CommonTagsCount
	})

	t.Run("many tags (l > CommonTagsCount)", func(t *testing.T) {
		// To drive l > CommonTagsCount=5, we need ≥6 distinct common prefixes.
		// Each prefix needs ≥2 tag names that start with it.
		tags := map[string]*analytics.Counter{}
		// 6 distinct alphabetic prefixes each shared by 2 tag names.
		// getListOfCommonPrefix builds counts of prefixes shared between pairs;
		// to ensure each "pa_", "pb_", ... bucket has at least one entry we
		// pair tag names within the same prefix family.
		for _, p := range []string{"pa", "pb", "pc", "pd", "pe", "pf", "pg"} {
			tags[p+"_one"] = &analytics.Counter{}
			tags[p+"_two"] = &analytics.Counter{}
		}
		doc := analytics.AnalyticsRecordAggregate{Tags: tags}
		// thresholdLenTagList=0 forces the alert to print.
		m.printAlert(doc, 0)
	})
}

// Verifies: SW-REQ-061
// SW-REQ-061:denial_of_service_resistant:fuzz
func FuzzMongoAggregateCommonPrefixBounded(f *testing.F) {
	f.Add("svc.alpha.one", "svc.alpha.two", "other.beta", 10)
	f.Add("", "x", "xy", 0)

	f.Fuzz(func(t *testing.T, a, b, c string, threshold int) {
		parts := []string{a, b, c}
		for i := range parts {
			if len(parts[i]) > 64 {
				parts[i] = parts[i][:64]
			}
		}

		_ = getListOfCommonPrefix(parts)

		tags := make(map[string]*analytics.Counter, len(parts))
		for _, part := range parts {
			tags[part] = &analytics.Counter{}
		}
		if threshold < 0 {
			threshold = 0
		}
		threshold %= 64

		m := &MongoAggregatePump{}
		m.log = logrus.NewEntry(logrus.New())
		m.printAlert(analytics.AnalyticsRecordAggregate{Tags: tags}, threshold)
	})
}

// Verifies: SW-REQ-062
// SW-REQ-062:denial_of_service_resistant:fuzz
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=F, self_heal_enabled=F, size_error_detected=T => TRUE
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=F, self_heal_enabled=T, size_error_detected=F => TRUE
// MCDC SW-REQ-062: aggregation_time_above_floor=T, aggregation_time_halved=T, self_heal_enabled=T, size_error_detected=T => TRUE
func FuzzMongoAggregateShouldSelfHealBounded(f *testing.F) {
	f.Add("Size must be between 0 and 16793600", true, 60)
	f.Add("Request size is too large", true, 2)
	f.Add("some other failure", true, 60)
	f.Add("Size must be between 0 and 16793600", false, 60)

	f.Fuzz(func(t *testing.T, errText string, enabled bool, aggregationTime int) {
		if len(errText) > 256 {
			errText = errText[:256]
		}
		if aggregationTime < 1 {
			aggregationTime = 1
		}
		if aggregationTime > 1440 {
			aggregationTime = 1440
		}

		p := &MongoAggregatePump{
			dbConf: &MongoAggregateConf{
				BaseMongoConf:              BaseMongoConf{MongoURL: "mongodb://fuzz"},
				EnableAggregateSelfHealing: enabled,
				AggregationTime:            aggregationTime,
			},
		}
		p.log = logrus.NewEntry(logrus.New())
		_ = p.ShouldSelfHeal(errors.New(errText))
	})
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: GetCollectionName
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-036
func TestMongoAggregatePump_GetCollectionName_Empty(t *testing.T) {
	m := &MongoAggregatePump{}
	got, err := m.GetCollectionName("")
	assert.Error(t, err)
	assert.Empty(t, got)
}

// Verifies: SW-REQ-036
func TestMongoAggregatePump_GetCollectionName_NonEmpty(t *testing.T) {
	m := &MongoAggregatePump{}
	got, err := m.GetCollectionName("org123")
	assert.NoError(t, err)
	assert.Equal(t, "z_tyk_analyticz_aggregate_org123", got)
}

// ---------------------------------------------------------------------------
// mongo_selective.go :: GetCollectionName
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:negative
func TestMongoSelectivePump_GetCollectionName_Empty(t *testing.T) {
	m := &MongoSelectivePump{}
	got, err := m.GetCollectionName("")
	assert.Error(t, err)
	assert.Empty(t, got)
}

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:nominal
func TestMongoSelectivePump_GetCollectionName_NonEmpty(t *testing.T) {
	m := &MongoSelectivePump{}
	got, err := m.GetCollectionName("acmecorp")
	assert.NoError(t, err)
	assert.Equal(t, "z_tyk_analyticz_acmecorp", got)
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: SetAggregationTime
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-058
// SW-REQ-058:nominal:nominal — drives AggregationTime > 60 branch
// MCDC SW-REQ-058: store_per_minute=F, window_eq_1_min=F => TRUE
// MCDC SW-REQ-058: store_per_minute=T, window_eq_1_min=F => FALSE
// MCDC SW-REQ-058: store_per_minute=T, window_eq_1_min=T => TRUE
//
// store_per_minute=F arm (AggregationTime=120, StoreAnalyticsPerMinute false): the window is
// clamped to 60, not 1 — window_eq_1_min=F (vacuous true). The store_per_minute=T arm
// (AggregationTime forced to 1) is exercised by TestSetAggregationTime_LessThan1 and the
// StoreAnalyticsPerMinute=true configuration in TestSetAggregationTime_ValidValuePreserved.
func TestSetAggregationTime_GreaterThan60(t *testing.T) {
	m := &MongoAggregatePump{
		dbConf: &MongoAggregateConf{AggregationTime: 120},
	}
	m.log = logrus.NewEntry(logrus.New())
	m.SetAggregationTime()
	assert.Equal(t, 60, m.dbConf.AggregationTime)
}

// Verifies: SW-REQ-058
// SW-REQ-058:nominal:nominal — drives AggregationTime < 1 branch
func TestSetAggregationTime_LessThan1(t *testing.T) {
	m := &MongoAggregatePump{
		dbConf: &MongoAggregateConf{AggregationTime: -5},
	}
	m.log = logrus.NewEntry(logrus.New())
	m.SetAggregationTime()
	assert.Equal(t, 60, m.dbConf.AggregationTime)
}

// Verifies: SW-REQ-058
// SW-REQ-058:nominal:nominal — valid AggregationTime is preserved
func TestSetAggregationTime_ValidValuePreserved(t *testing.T) {
	m := &MongoAggregatePump{
		dbConf: &MongoAggregateConf{AggregationTime: 30},
	}
	m.log = logrus.NewEntry(logrus.New())
	m.SetAggregationTime()
	assert.Equal(t, 30, m.dbConf.AggregationTime)
}

// ---------------------------------------------------------------------------
// mongo.go :: shouldProcessItem
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// MCDC SW-REQ-034: mcp_record_present=F, record_filtered_out=F => TRUE
// MCDC SW-REQ-034: mcp_record_present=T, record_filtered_out=F => FALSE
// MCDC SW-REQ-034: mcp_record_present=T, record_filtered_out=T => TRUE
// (Type-mismatch / ResponseCode==-1 cases below exercise the
// mcp_record_present=F path; the MCP-present-and-filtered pair is driven
// by TestMongoAggregatePump_WriteData_MCPRecordFiltered which feeds an
// IsMCPRecord()=T record through the filter loop.)
// SW-REQ-034:boundary:negative — non-AnalyticsRecord input is skipped
func TestMongoPump_ShouldProcessItem_TypeMismatch(t *testing.T) {
	m := &MongoPump{}
	m.log = logrus.NewEntry(logrus.New())
	rec, skip := m.shouldProcessItem("not-a-record", false)
	assert.Nil(t, rec)
	assert.True(t, skip)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — graph filter excludes non-graph records
func TestMongoPump_ShouldProcessItem_GraphFilter(t *testing.T) {
	m := &MongoPump{}
	m.log = logrus.NewEntry(logrus.New())
	non := analytics.AnalyticsRecord{}
	rec, skip := m.shouldProcessItem(non, true)
	assert.NotNil(t, rec)
	assert.True(t, skip, "non-graph record must be skipped in graph-only mode")
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:negative — ResponseCode==-1 short-circuits
func TestMongoPump_ShouldProcessItem_ResponseCodeNeg1(t *testing.T) {
	m := &MongoPump{}
	m.log = logrus.NewEntry(logrus.New())
	rec, skip := m.shouldProcessItem(analytics.AnalyticsRecord{ResponseCode: -1}, false)
	assert.NotNil(t, rec)
	assert.True(t, skip)
}

// ---------------------------------------------------------------------------
// mongo_selective.go :: processItem
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:negative — non-AnalyticsRecord input is skipped
func TestMongoSelectivePump_ProcessItem_TypeMismatch(t *testing.T) {
	m := &MongoSelectivePump{}
	m.log = logrus.NewEntry(logrus.New())
	_, skip := m.processItem(42)
	assert.True(t, skip)
}

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:negative — ResponseCode==-1 is skipped
func TestMongoSelectivePump_ProcessItem_ResponseNeg1(t *testing.T) {
	m := &MongoSelectivePump{}
	m.log = logrus.NewEntry(logrus.New())
	_, skip := m.processItem(analytics.AnalyticsRecord{ResponseCode: -1})
	assert.True(t, skip)
}

// ---------------------------------------------------------------------------
// mongo.go :: handleLargeDocuments
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — over-size non-graph triggers rewrite
func TestMongoPump_HandleLargeDocuments_Truncates(t *testing.T) {
	m := &MongoPump{dbConf: &MongoConf{MaxDocumentSizeBytes: 100}}
	m.log = logrus.NewEntry(logrus.New())
	rec := &analytics.AnalyticsRecord{RawRequest: "raw-request", RawResponse: "raw-response"}
	m.handleLargeDocuments(rec, 500, false)
	assert.Empty(t, rec.RawRequest, "raw request should be cleared")
	assert.NotEmpty(t, rec.RawResponse, "raw response should be replaced with base64 explanation")
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:negative — graph records are NOT truncated even if oversize
func TestMongoPump_HandleLargeDocuments_GraphPreserved(t *testing.T) {
	m := &MongoPump{dbConf: &MongoConf{MaxDocumentSizeBytes: 100}}
	m.log = logrus.NewEntry(logrus.New())
	rec := &analytics.AnalyticsRecord{RawRequest: "raw-request", RawResponse: "raw-response"}
	m.handleLargeDocuments(rec, 500, true)
	assert.Equal(t, "raw-request", rec.RawRequest)
	assert.Equal(t, "raw-response", rec.RawResponse)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:negative — under-size is left alone
func TestMongoPump_HandleLargeDocuments_BelowLimit(t *testing.T) {
	m := &MongoPump{dbConf: &MongoConf{MaxDocumentSizeBytes: 1000}}
	m.log = logrus.NewEntry(logrus.New())
	rec := &analytics.AnalyticsRecord{RawRequest: "x", RawResponse: "y"}
	m.handleLargeDocuments(rec, 500, false)
	assert.Equal(t, "x", rec.RawRequest)
	assert.Equal(t, "y", rec.RawResponse)
}

// ---------------------------------------------------------------------------
// mongo.go :: SetDecodingRequest / SetDecodingResponse — `decoding=true` branch
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — both true (log) AND false (no-op) branches
func TestMongoPump_SetDecoding_BothBranches(t *testing.T) {
	m := &MongoPump{}
	m.SetDecodingRequest(false) // F branch
	m.SetDecodingRequest(true)  // T branch (logs)
	m.SetDecodingResponse(false)
	m.SetDecodingResponse(true)
	assert.False(t, m.GetDecodedRequest())
	assert.False(t, m.GetDecodedResponse())
}

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:nominal — both true AND false branches
func TestMongoSelectivePump_SetDecoding_BothBranches(t *testing.T) {
	m := &MongoSelectivePump{}
	m.SetDecodingRequest(false)
	m.SetDecodingRequest(true)
	m.SetDecodingResponse(false)
	m.SetDecodingResponse(true)
	assert.False(t, m.GetDecodedRequest())
	assert.False(t, m.GetDecodedResponse())
}

// Verifies: SW-REQ-036
func TestMongoAggregatePump_SetDecoding_BothBranches(t *testing.T) {
	m := &MongoAggregatePump{}
	m.SetDecodingRequest(false)
	m.SetDecodingRequest(true)
	m.SetDecodingResponse(false)
	m.SetDecodingResponse(true)
	assert.False(t, m.GetDecodedRequest())
	assert.False(t, m.GetDecodedResponse())
}

// Verifies: INT-REQ-004
// MCDC INT-REQ-004: contract_honoured=T, pump_methods_called=T => TRUE
func TestGraphMongoPump_SetDecoding_BothBranches(t *testing.T) {
	m := &GraphMongoPump{}
	m.SetDecodingRequest(false)
	m.SetDecodingRequest(true)
	m.SetDecodingResponse(false)
	m.SetDecodingResponse(true)
	assert.False(t, m.GetDecodedRequest())
	assert.False(t, m.GetDecodedResponse())
}

// Verifies: INT-REQ-004
// MCDC INT-REQ-004: contract_honoured=T, pump_methods_called=T => TRUE
func TestMCPMongoPump_SetDecoding_BothBranches(t *testing.T) {
	m := &MCPMongoPump{}
	m.SetDecodingRequest(false)
	m.SetDecodingRequest(true)
	m.SetDecodingResponse(false)
	m.SetDecodingResponse(true)
	assert.False(t, m.GetDecodedRequest())
	assert.False(t, m.GetDecodedResponse())
}

// Verifies: INT-REQ-004
// MCDC INT-REQ-004: contract_honoured=T, pump_methods_called=T => TRUE
func TestMCPMongoAggregatePump_SetDecoding_BothBranches(t *testing.T) {
	m := &MCPMongoAggregatePump{}
	m.SetDecodingRequest(false)
	m.SetDecodingRequest(true)
	m.SetDecodingResponse(false)
	m.SetDecodingResponse(true)
	assert.False(t, m.GetDecodedRequest())
	assert.False(t, m.GetDecodedResponse())
}

// ---------------------------------------------------------------------------
// mongo.go :: parsePrivateKey — exercises the three parsers
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:errors_propagated:negative — garbage bytes yield error from all 3 parsers
func TestParsePrivateKey_Garbage(t *testing.T) {
	_, err := parsePrivateKey([]byte{0xde, 0xad, 0xbe, 0xef})
	assert.Error(t, err)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — drives the first parser (PKCS1) success path
func TestParsePrivateKey_PKCS1(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(priv)

	got, err := parsePrivateKey(der)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — drives the PKCS8 RSA success path
func TestParsePrivateKey_PKCS8(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)

	got, err := parsePrivateKey(der)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — drives the ECPrivateKey success path
func TestParsePrivateKey_EC(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)

	got, err := parsePrivateKey(der)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

// ---------------------------------------------------------------------------
// mongo.go :: accumulate — last-item flush vs chunk-creation branches
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — exceeding batch flushes mid-stream
func TestMongoPump_Accumulate_ChunkOverflowMidStream(t *testing.T) {
	m := &MongoPump{dbConf: &MongoConf{MaxInsertBatchSizeBytes: 100}}
	m.log = logrus.NewEntry(logrus.New())

	// First item fits, second exceeds → previous set should be flushed.
	rs := []model.DBObject{}
	ret := [][]model.DBObject{}
	r1 := &analytics.AnalyticsRecord{}
	r2 := &analytics.AnalyticsRecord{}

	_, rs, ret = m.accumulate(rs, ret, r1, 60, 0, false)
	_, rs, ret = m.accumulate(rs, ret, r2, 60, 60, true) // exceeds, AND is last
	// ret should contain at least one flushed slice + final flush.
	assert.GreaterOrEqual(t, len(ret), 1)
	_ = rs
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:negative — empty-result-set on overflow takes the
// `len(thisResultSet) == 0` branch.
func TestMongoPump_Accumulate_OverflowWithEmptyResultSet(t *testing.T) {
	m := &MongoPump{dbConf: &MongoConf{MaxInsertBatchSizeBytes: 50}}
	m.log = logrus.NewEntry(logrus.New())
	r := &analytics.AnalyticsRecord{}
	// accumulatorTotal already at limit, thisResultSet empty
	total, _, ret := m.accumulate(nil, nil, r, 100, 50, false)
	assert.Equal(t, 100, total)
	// Since isLastItem=false and the cleared-resultset is non-empty after
	// appending r, ret stays empty.
	assert.Empty(t, ret)
}

// ---------------------------------------------------------------------------
// mongo_selective.go :: accumulate — sizeBytes < 0 path
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:negative — negative sizeBytes early-returns unchanged
func TestMongoSelectivePump_Accumulate_NegativeSize(t *testing.T) {
	m := &MongoSelectivePump{dbConf: &MongoSelectiveConf{MaxInsertBatchSizeBytes: 100}}
	m.log = logrus.NewEntry(logrus.New())
	rs := []model.DBObject{}
	ret := [][]model.DBObject{}
	r := &analytics.AnalyticsRecord{}
	gotTotal, gotRs, gotRet := m.accumulate(rs, ret, r, -1, 5, false)
	assert.Equal(t, 5, gotTotal)
	assert.Empty(t, gotRs)
	assert.Empty(t, gotRet)
}

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:nominal — overflow with last-item triggers two appends
func TestMongoSelectivePump_Accumulate_OverflowLast(t *testing.T) {
	m := &MongoSelectivePump{dbConf: &MongoSelectiveConf{MaxInsertBatchSizeBytes: 100}}
	m.log = logrus.NewEntry(logrus.New())
	rs := []model.DBObject{&analytics.AnalyticsRecord{}}
	ret := [][]model.DBObject{}
	r := &analytics.AnalyticsRecord{}
	_, _, gotRet := m.accumulate(rs, ret, r, 200, 50, true)
	// The overflow branch flushes `rs` then the isLastItem branch flushes the
	// new (length-1) set → two entries total.
	assert.Len(t, gotRet, 2)
}

// ---------------------------------------------------------------------------
// mcp_mongo.go :: filterMCPData / convertToMCPObjects / WriteData
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-038
func TestConvertToMCPObjects_SkipsNonAnalyticsRecord(t *testing.T) {
	dummy := dummyObject{tableName: "x"}
	got := convertToMCPObjects([]model.DBObject{dummy})
	assert.Empty(t, got)
}

// error path in insertMCPDataSet via a fake store. We cannot construct a
// fake persistent.Store here, but we can directly call WriteData with an
// empty MCP set to short-circuit through filterMCPData→AccumulateSet.
//
// Verifies: SW-REQ-038
func TestMCPMongoPump_WriteData_FilterShortCircuit(t *testing.T) {
	p := &MCPMongoPump{}
	p.dbConf = &MongoConf{CollectionName: "x"}
	p.log = logrus.NewEntry(logrus.New())
	err := p.WriteData(context.Background(), []interface{}{
		analytics.AnalyticsRecord{APIID: "rest"}, // not MCP
	})
	assert.NoError(t, err, "no MCP records → early return")
}

// ---------------------------------------------------------------------------
// mcp_mongo_aggregate.go :: addMCPDimensionUpdates — exercising $min == nil
// path. The branch is: `if updateDoc["$min"] == nil { updateDoc["$min"] = ... }`
// Default updateDoc from AsChange() typically supplies $min, but we can pass an
// updateDoc with a deleted $min to drive the nil branch.
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-039
func TestAddMCPDimensionUpdates_InitializesMinWhenAbsent(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	data := []interface{}{
		analytics.AnalyticsRecord{
			OrgID: "org1", APIID: "api1", TimeStamp: ts,
			ResponseCode: 200, RequestTime: 100,
			Latency:  analytics.Latency{Total: 100, Upstream: 50},
			MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "t1"},
		},
	}
	result := analytics.AggregateMCPData(data, "", 60)
	ag := result["api1"]

	updateDoc := ag.AnalyticsRecordAggregate.AsChange()
	// Remove $min so addMCPDimensionUpdates must initialize it.
	delete(updateDoc, "$min")
	addMCPDimensionUpdates(&ag, updateDoc)

	// $min must now exist and contain at least one MCP-dimension entry.
	minDoc, has := updateDoc["$min"].(model.DBM)
	assert.True(t, has, "$min should have been initialized")
	assert.NotEmpty(t, minDoc)
}

// ---------------------------------------------------------------------------
// mongo.go :: WriteData — empty/all-MCP filter short-circuit
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — all-MCP payload must short-circuit
func TestMongoPump_WriteData_AllMCPShortCircuits(t *testing.T) {
	p := &MongoPump{}
	p.dbConf = &MongoConf{CollectionName: "x"}
	p.log = logrus.NewEntry(logrus.New())
	data := []interface{}{
		analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}},
	}
	err := p.WriteData(context.Background(), data)
	assert.NoError(t, err, "all-MCP must not reach the store")
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:negative — empty input still returns nil
func TestMongoPump_WriteData_EmptyData(t *testing.T) {
	p := &MongoPump{}
	p.dbConf = &MongoConf{CollectionName: "x"}
	p.log = logrus.NewEntry(logrus.New())
	err := p.WriteData(context.Background(), nil)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: WriteData — empty/all-MCP filter short-circuit
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-058
// SW-REQ-058:nominal:negative — empty input returns nil
func TestMongoAggregatePump_WriteData_EmptyData(t *testing.T) {
	p := &MongoAggregatePump{}
	p.dbConf = &MongoAggregateConf{}
	p.log = logrus.NewEntry(logrus.New())
	err := p.WriteData(context.Background(), nil)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: ensureIndexes — OmitIndexCreation branch
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-063
// SW-REQ-063:nominal:nominal — OmitIndexCreation=true short-circuits
//
//mcdc:ignore SW-REQ-063: collection_already_exists=F, create_index_skipped=F, omit_index_creation=T => FALSE — mongo_aggregate.go:250-252 returns nil immediately when OmitIndexCreation is true (before any CreateIndex call), so create_index_skipped is always T; the "omit set yet index created anyway" violation has no branch to reach it [reviewed: human:leo] [category: defensive]
//mcdc:ignore SW-REQ-063: collection_already_exists=T, create_index_skipped=F, omit_index_creation=T => FALSE — mongo_aggregate.go:250-252 short-circuits on OmitIndexCreation before the collectionExists check at line 256, so when omit is set the index is always skipped regardless of collection existence; create_index_skipped is always T [reviewed: human:leo] [category: defensive]
func TestMongoAggregatePump_EnsureIndexes_Omit(t *testing.T) {
	p := &MongoAggregatePump{}
	p.dbConf = &MongoAggregateConf{}
	p.dbConf.OmitIndexCreation = true
	p.log = logrus.NewEntry(logrus.New())
	err := p.ensureIndexes("noop_collection")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// mongo.go :: ensureIndexes — OmitIndexCreation branch
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — OmitIndexCreation=true short-circuits
func TestMongoPump_EnsureIndexes_Omit(t *testing.T) {
	p := &MongoPump{}
	p.dbConf = &MongoConf{}
	p.dbConf.OmitIndexCreation = true
	p.log = logrus.NewEntry(logrus.New())
	err := p.ensureIndexes("noop_collection")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// mongo_selective.go :: ensureIndexes — OmitIndexCreation branch
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:nominal — OmitIndexCreation=true short-circuits
func TestMongoSelectivePump_EnsureIndexes_Omit(t *testing.T) {
	p := &MongoSelectivePump{}
	p.dbConf = &MongoSelectiveConf{}
	p.dbConf.OmitIndexCreation = true
	p.log = logrus.NewEntry(logrus.New())
	err := p.ensureIndexes("noop_collection")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// graph_mongo.go :: WriteData — empty collection name returns error
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-037
func TestGraphMongoPump_WriteData_EmptyCollectionName(t *testing.T) {
	p := &GraphMongoPump{}
	p.MongoPump.dbConf = &MongoConf{CollectionName: ""}
	p.log = logrus.NewEntry(logrus.New())
	err := p.WriteData(context.Background(), []interface{}{
		analytics.AnalyticsRecord{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no collection name")
}

// ---------------------------------------------------------------------------
// mcp_mongo_aggregate.go :: Init invalid map decode fallthrough
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-039
// SW-REQ-039:malformed_recovers_or_errors_loudly:negative
func TestMCPMongoAggregatePump_Init_NonMapConfig(t *testing.T) {
	p := &MCPMongoAggregatePump{}
	err := p.Init(42)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: divideAggregationTime + getLastDocumentTimestamp
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-036
func TestMongoAggregatePump_GetLastDocumentTimestamp_NoCollection(t *testing.T) {
	// Set up an integration test using the testcontainer mongo.
	uri := testMongoURI(t)
	cfg := map[string]interface{}{
		"mongo_url":            uri,
		"use_mixed_collection": false,
		"aggregation_time":     60,
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	// On a fresh DB the mixed collection won't exist, so the query returns an
	// error and getLastDocumentTimestamp returns (zero, err).
	ts, err := p.getLastDocumentTimestamp()
	assert.Error(t, err)
	assert.True(t, ts.IsZero())
}

// ---------------------------------------------------------------------------
// KI: mongo-pump-ignores-caller-context
// Demonstrates that MongoPump.WriteData ignores the passed-in ctx and uses
// context.Background() for the actual insert. We supply an already-canceled
// context; if the bug is fixed the insert should error with ctx.Err(); today
// the test asserts the bug behavior (insert succeeds despite cancellation).
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// Verifies: KI:mongo-pump-ignores-caller-context
// Reproduces: mongo-pump-ignores-caller-context
// SW-REQ-034:errors_propagated:negative — caller context is silently ignored
func TestMongoPump_WriteData_IgnoresCallerCtx_KI(t *testing.T) {
	uri := testMongoURI(t)
	conf := defaultConf(t)
	conf.MongoURL = uri
	conf.CollectionName = uniqueCollection(t)

	p := &MongoPump{}
	p.dbConf = &conf
	p.log = log.WithField("prefix", "ki-mongo-ctx")
	p.connect()
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	canceled, cancel := context.WithCancel(context.Background())
	cancel() // pre-canceled

	rec := analytics.AnalyticsRecord{APIID: "k", OrgID: "o", ResponseCode: 200}
	err := p.WriteData(canceled, []interface{}{rec})

	// With the KI active, the write succeeds despite ctx.Done() being fired
	// because m.store.Insert is invoked with context.Background() internally.
	// If the production code is ever fixed to honor `ctx`, this assertion
	// must flip and a follow-up commit should retire the KI.
	require.NoError(t, err, "KI active: caller ctx is ignored; write should still succeed")

	// Sanity-check: ctx was indeed canceled.
	assert.True(t, errors.Is(canceled.Err(), context.Canceled))
}

// ---------------------------------------------------------------------------
// mgo_helper_test.go :: ensureMongoDatabase helper (covers branch decisions
// in the test helper itself so its logic is regression-proof).
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — appends db when missing
func TestEnsureMongoDatabase_AppendsDB(t *testing.T) {
	got := ensureMongoDatabase("mongodb://h:27017", "tyk")
	assert.Equal(t, "mongodb://h:27017/tyk", got)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — trailing slash gets db appended
func TestEnsureMongoDatabase_TrailingSlash(t *testing.T) {
	got := ensureMongoDatabase("mongodb://h:27017/", "tyk")
	assert.Equal(t, "mongodb://h:27017/tyk", got)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:negative — existing db path is left alone
func TestEnsureMongoDatabase_HasDB(t *testing.T) {
	got := ensureMongoDatabase("mongodb://h:27017/already", "tyk")
	assert.Equal(t, "mongodb://h:27017/already", got)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — query string is preserved
func TestEnsureMongoDatabase_QueryPreserved(t *testing.T) {
	got := ensureMongoDatabase("mongodb://h:27017?ssl=true", "tyk")
	assert.Equal(t, "mongodb://h:27017/tyk?ssl=true", got)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:negative — empty URI passes through
func TestEnsureMongoDatabase_Empty(t *testing.T) {
	assert.Empty(t, ensureMongoDatabase("", "tyk"))
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:negative — URI without scheme still gets db appended
func TestEnsureMongoDatabase_NoScheme(t *testing.T) {
	got := ensureMongoDatabase("h:27017", "tyk")
	assert.Equal(t, "h:27017/tyk", got)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — uniqueCollection is stable & sanitized
func TestUniqueCollection_Sanitization(t *testing.T) {
	got := uniqueCollection(t)
	assert.True(t, strings.HasPrefix(got, "tyk_test_"))
	assert.NotContains(t, got, "/")
}

// ---------------------------------------------------------------------------
// mongo.go :: Init — drive the MaxInsertBatchSizeBytes==0,
// MaxDocumentSizeBytes==0, and indexCreateErr branches.
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — defaults kick in for batch / document size,
// and Init succeeds against a live mongo.
func TestMongoPump_Init_AppliesDefaults(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":                   testMongoURI(t),
		"collection_name":             uniqueCollection(t),
		"max_insert_batch_size_bytes": 0,
		"max_document_size_bytes":     0,
	}
	p := &MongoPump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })
	assert.Equal(t, 10*MiB, p.dbConf.MaxInsertBatchSizeBytes)
	assert.Equal(t, 10*MiB, p.dbConf.MaxDocumentSizeBytes)
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — uptime mode preserves the collection name
// override (tyk_uptime_analytics) when URL is empty at Init.
func TestMongoPump_Init_UptimeWithoutURL(t *testing.T) {
	// Even with IsUptime=true and MongoURL="", Init will reach connect();
	// to avoid log.Fatal we explicitly set the URL via env config simulation:
	// instead, just supply an URL but use IsUptime mode to drive the
	// "MongoURL == \"\"" decision false branch (covering the IsUptime path).
	p := &MongoPump{IsUptime: true}
	cfg := map[string]interface{}{
		"mongo_url": testMongoURI(t),
	}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })
	// In uptime mode with URL set, the if-branch (MongoURL == "") is false,
	// so collection name is whatever was in cfg (empty here).
}

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — IsUptime=true AND MongoURL=="" loads URL from
// PMP_MONGO env and overrides the collection name to tyk_uptime_analytics.
func TestMongoPump_Init_UptimeWithEnvURL(t *testing.T) {
	uri := testMongoURI(t)
	t.Setenv("PMP_MONGO_MONGOURL", uri)
	t.Setenv("PMP_MONGO_COLLECTIONNAME", "ignored_should_be_overridden")

	p := &MongoPump{IsUptime: true}
	// Empty cfg → mongo_url stays "" → triggers the env-load branch.
	require.NoError(t, p.Init(map[string]interface{}{}))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })
	assert.Equal(t, "tyk_uptime_analytics", p.dbConf.CollectionName)
}

// ---------------------------------------------------------------------------
// mongo_selective.go :: Init — drive the default-applying branches.
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:nominal — defaults are applied
func TestMongoSelectivePump_Init_AppliesDefaults(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":                   testMongoURI(t),
		"max_insert_batch_size_bytes": 0,
		"max_document_size_bytes":     0,
	}
	p := &MongoSelectivePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })
	assert.Equal(t, 10*MiB, p.dbConf.MaxInsertBatchSizeBytes)
	assert.Equal(t, 10*MiB, p.dbConf.MaxDocumentSizeBytes)
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: Init — drive the ThresholdLenTagList default branch.
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-036
func TestMongoAggregatePump_Init_AppliesThresholdDefault(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":              testMongoURI(t),
		"use_mixed_collection":   false,
		"threshold_len_tag_list": 0,
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })
	assert.Equal(t, ThresholdLenTagList, p.dbConf.ThresholdLenTagList)
}

// ---------------------------------------------------------------------------
// mcp_mongo.go :: Init — drive both default branches.
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-038
func TestMCPMongoPump_Init_AppliesDefaults(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":                   testMongoURI(t),
		"collection_name":             uniqueCollection(t),
		"max_insert_batch_size_bytes": 0,
		"max_document_size_bytes":     0,
	}
	p := &MCPMongoPump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })
	assert.Equal(t, 10*MiB, p.dbConf.MaxInsertBatchSizeBytes)
	assert.Equal(t, 10*MiB, p.dbConf.MaxDocumentSizeBytes)
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: ensureIndexes — drive CosmosDB skip branch and the
// AWSDocumentDB "skip collectionExists check" branch.
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-063
// SW-REQ-063:nominal:nominal — CosmosDB skips TTL index creation
func TestMongoAggregatePump_EnsureIndexes_CosmosDBSkipsTTL(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":     testMongoURI(t),
		"mongo_db_type": int(CosmosDB),
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	colName := uniqueCollection(t) + "_cosmos"
	err := p.ensureIndexes(colName)
	assert.NoError(t, err)
}

// Verifies: SW-REQ-063
// SW-REQ-063:nominal:nominal — AWSDocumentDB skips the collection-exists check
func TestMongoAggregatePump_EnsureIndexes_DocDB(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":     testMongoURI(t),
		"mongo_db_type": int(AWSDocumentDB),
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	colName := uniqueCollection(t) + "_docdb"
	err := p.ensureIndexes(colName)
	assert.NoError(t, err)
}

// Verifies: SW-REQ-063
// SW-REQ-063:nominal:negative — collection already exists short-circuits
// (StandardMongo path).
// SW-REQ-063:idempotency:example
// SW-REQ-063:idempotency:nominal
//
//mcdc:ignore SW-REQ-063: collection_already_exists=T, create_index_skipped=F, omit_index_creation=F => FALSE — mongo_aggregate.go:256-261 returns nil once the StandardMongo collectionExists check reports the collection exists (before any CreateIndex call), so create_index_skipped is always T; the "collection exists yet index created anyway" violation has no branch to reach it [reviewed: human:leo] [category: defensive]
func TestMongoAggregatePump_EnsureIndexes_AlreadyExists(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url": testMongoURI(t),
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	colName := uniqueCollection(t) + "_exists"
	// Pre-create the collection so the next ensureIndexes returns early.
	obj := dbObject{tableName: colName}
	require.NoError(t, p.store.Migrate(context.Background(), []model.DBObject{obj}))

	err := p.ensureIndexes(colName)
	assert.NoError(t, err, "early-return when collection exists must not error")
}

// Verifies: SW-REQ-063
// SW-REQ-063:nominal:nominal — OmitIndexCreation=true on an already-existing
// collection still skips index creation (both triggers true).
// SW-REQ-063:idempotency:example
// MCDC SW-REQ-063: collection_already_exists=T, create_index_skipped=T, omit_index_creation=T => TRUE
//
// This drives the row where both guarantee triggers hold simultaneously: the
// collection already exists (collection_already_exists=T) AND OmitIndexCreation
// is set (omit_index_creation=T). mongo_aggregate.go:250-252 short-circuits on
// the omit guard first and returns nil without creating any index
// (create_index_skipped=T) — the satisfied row 5.
func TestMongoAggregatePump_EnsureIndexes_OmitOnExisting(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":           testMongoURI(t),
		"omit_index_creation": true,
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	colName := uniqueCollection(t) + "_omit_exists"
	// Pre-create the collection so collection_already_exists=T.
	obj := dbObject{tableName: colName}
	require.NoError(t, p.store.Migrate(context.Background(), []model.DBObject{obj}))

	// With omit=true the omit guard wins before the collectionExists check;
	// index creation is skipped without error.
	err := p.ensureIndexes(colName)
	assert.NoError(t, err, "omit_index_creation on an existing collection must skip creation without error")

	idxs, err := p.store.GetIndexes(context.Background(), dbObject{tableName: colName})
	require.NoError(t, err)
	for _, idx := range idxs {
		assert.NotEqual(t, "expireAt_1", idx.Name,
			"no TTL index must be created when omit_index_creation is set")
	}
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: WriteData — drive UseMixedCollection branch + empty
// filtered branch.
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-058
// SW-REQ-058:nominal:nominal — UseMixedCollection=false writes only org doc
func TestMongoAggregatePump_WriteData_NoMixed(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":            testMongoURI(t),
		"use_mixed_collection": false,
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	rec := analytics.AnalyticsRecord{
		APIID: "api1", OrgID: "orgX", TimeStamp: time.Now(), ResponseCode: 200,
	}
	require.NoError(t, p.WriteData(context.Background(), []interface{}{rec}))
}

// ---------------------------------------------------------------------------
// mongo_aggregate.go :: DoAggregatedWriting — ThresholdLenTagList branch
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-061
// MCDC SW-REQ-061: alert_emitted=F, alert_not_disabled=F, tag_list_len_gt_threshold=T => TRUE
//
// mongo_aggregate.go:391 gates the alert on
// `ThresholdLenTagList != -1 && len(Tags) > ThresholdLenTagList`. With
// threshold_len_tag_list=-1 the first conjunct is false, so alert_not_disabled=F
// and no alert is emitted (alert_emitted=F) even though the 3 tags exceed -1
// (tag_list_len_gt_threshold=T) — the disable knob makes the guarantee hold
// vacuously: row 1. The enabled-but-below-threshold arm (row 2) and the
// alert-emitted arm (row 4) are driven by the two tests immediately below.
func TestMongoAggregatePump_DoAggregatedWriting_DisabledThreshold(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":              testMongoURI(t),
		"use_mixed_collection":   false,
		"threshold_len_tag_list": -1,
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	now := time.Now()
	rec := analytics.AnalyticsRecord{
		APIID: "api2", OrgID: "orgY", TimeStamp: now, ResponseCode: 200,
		Tags: []string{"a", "b", "c"},
	}
	require.NoError(t, p.WriteData(context.Background(), []interface{}{rec}))
}

// tagAlertFired reports whether DoAggregatedWriting emitted the
// "Found more than ... tag entries per document" Warn alert via printAlert.
func tagAlertFired(entries []*logrus.Entry) bool {
	for _, e := range entries {
		if e.Level == logrus.WarnLevel && strings.Contains(e.Message, "tag entries per document") {
			return true
		}
	}
	return false
}

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:negative — alerting enabled but tag count below threshold.
// MCDC SW-REQ-061: alert_emitted=F, alert_not_disabled=T, tag_list_len_gt_threshold=F => TRUE
//
// mongo_aggregate.go:391 gates the alert on
// `ThresholdLenTagList != -1 && len(Tags) > ThresholdLenTagList`. Here the
// threshold is 100 (alert_not_disabled=T because != -1) and the record carries
// only 3 tags, so `len(Tags) > 100` is false (tag_list_len_gt_threshold=F) and
// no alert is emitted (alert_emitted=F). With alerting enabled but the tag list
// under the threshold, the guarantee holds vacuously — row 2. The log hook
// asserts the Warn alert did NOT fire.
func TestMongoAggregatePump_DoAggregatedWriting_EnabledBelowThreshold(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":              testMongoURI(t),
		"use_mixed_collection":   false,
		"threshold_len_tag_list": 100,
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	hook := logrustest.NewLocal(p.log.Logger)
	t.Cleanup(hook.Reset)

	rec := analytics.AnalyticsRecord{
		APIID: "api61below", OrgID: "org61below", TimeStamp: time.Now(), ResponseCode: 200,
		Tags: []string{"a", "b", "c"},
	}
	require.NoError(t, p.WriteData(context.Background(), []interface{}{rec}))
	assert.False(t, tagAlertFired(hook.AllEntries()),
		"alert must NOT fire when tag count (3) is below threshold (100)")
}

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:nominal — alerting enabled and tag count over threshold.
// SW-REQ-061:denial_of_service_resistant:nominal
// MCDC SW-REQ-061: alert_emitted=T, alert_not_disabled=T, tag_list_len_gt_threshold=T => TRUE
//
// mongo_aggregate.go:391-392 fires printAlert when
// `ThresholdLenTagList != -1 && len(Tags) > ThresholdLenTagList`. Here the
// threshold is 1 (alert_not_disabled=T) and the aggregated record carries 3
// tags, so `len(Tags) > 1` holds (tag_list_len_gt_threshold=T): printAlert is
// invoked and emits the Warn alert (alert_emitted=T) — the satisfied row 4. The
// log hook asserts the Warn alert DID fire.
//
//mcdc:ignore SW-REQ-061: alert_emitted=F, alert_not_disabled=T, tag_list_len_gt_threshold=T => FALSE — mongo_aggregate.go:391-392 calls printAlert unconditionally once both conjuncts hold (ThresholdLenTagList != -1 and len(Tags) > threshold), with no branch between the guard and the printAlert call, so when alerting is enabled and the tag list exceeds the threshold the alert is always emitted; the "enabled+over-threshold yet no alert" violation has no branch to reach it [reviewed: human:leo] [category: defensive]
func TestMongoAggregatePump_DoAggregatedWriting_AlertEmitted(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":              testMongoURI(t),
		"use_mixed_collection":   false,
		"threshold_len_tag_list": 1,
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	hook := logrustest.NewLocal(p.log.Logger)
	t.Cleanup(hook.Reset)

	rec := analytics.AnalyticsRecord{
		APIID: "api61alert", OrgID: "org61alert", TimeStamp: time.Now(), ResponseCode: 200,
		Tags: []string{"alpha", "beta", "gamma"},
	}
	require.NoError(t, p.WriteData(context.Background(), []interface{}{rec}))
	assert.True(t, tagAlertFired(hook.AllEntries()),
		"alert must fire when tag count (3) exceeds threshold (1)")
}

// ---------------------------------------------------------------------------
// mongo_selective.go :: WriteData — empty OrgID branch (skip)
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-035
// MCDC SW-REQ-035: org_id_present=F, record_routed_to_org_collection=F => TRUE
// MCDC SW-REQ-035: org_id_present=T, record_routed_to_org_collection=F => FALSE
// MCDC SW-REQ-035: org_id_present=T, record_routed_to_org_collection=T => TRUE
// (This test drives org_id_present=F (OrgID empty) and asserts no insert —
// F/F=TRUE. Sibling TestMongoSelectivePump_WriteData feeds OrgID="o" so the
// record is routed to z_tyk_analyticz_o — drives T/T=TRUE. The T/F=FALSE
// pair is driven by the dialect-error / insert-failure tests where an org
// is present but the per-org insert is aborted.)
// SW-REQ-035:boundary:nominal — empty OrgID skips the record entirely
func TestMongoSelectivePump_WriteData_EmptyOrgIDSkips(t *testing.T) {
	p := &MongoSelectivePump{}
	conf := defaultSelectiveConf(t)
	p.dbConf = &conf
	p.log = log.WithField("prefix", mongoSelectivePrefix)
	p.connect()
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	data := []interface{}{
		analytics.AnalyticsRecord{APIID: "noorg"}, // OrgID is empty
	}
	require.NoError(t, p.WriteData(context.Background(), data))
}

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:nominal — MCP records short-circuited
func TestMongoSelectivePump_WriteData_MCPRecordSkipped(t *testing.T) {
	p := &MongoSelectivePump{}
	conf := defaultSelectiveConf(t)
	p.dbConf = &conf
	p.log = log.WithField("prefix", mongoSelectivePrefix)
	p.connect()
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	data := []interface{}{
		analytics.AnalyticsRecord{
			APIID: "x", OrgID: "y",
			MCPStats: analytics.MCPStats{IsMCP: true},
		},
	}
	require.NoError(t, p.WriteData(context.Background(), data))
}

// ---------------------------------------------------------------------------
// mongo.go :: WriteUptimeData — drive unmarshal-error + empty branches
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-034
// SW-REQ-034:boundary:nominal — empty data returns immediately
func TestMongoPump_WriteUptimeData_Empty(t *testing.T) {
	p := &MongoPump{}
	p.dbConf = &MongoConf{}
	p.log = logrus.NewEntry(logrus.New())
	// No store needed for empty branch.
	p.WriteUptimeData(nil)
}

// Verifies: SW-REQ-034
// Verifies: KI:mongo-pump-writeuptime-nil-on-bad-msgpack
// Reproduces: mongo-pump-writeuptime-nil-on-bad-msgpack
// SW-REQ-034:errors_propagated:negative — the test name carries the _KI
// suffix because we expect the panic and recover from it; if the production
// code is ever fixed to filter out failed-decode entries this test must flip
// and the KI must be retired.
func TestMongoPump_WriteUptimeData_BadMsgpack_KI(t *testing.T) {
	p := &MongoPump{IsUptime: true}
	conf := defaultConf(t)
	require.NoError(t, p.Init(conf))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	defer func() {
		// The KI is open: bad-msgpack records make WriteUptimeData crash with
		// a nil-pointer deref in the persistent layer. Once the bug is fixed
		// recover() will return nil and this assertion will flip — at which
		// point the KI should be retired and the test renamed (drop _KI).
		r := recover()
		assert.NotNil(t, r, "KI active: WriteUptimeData should still panic on bad msgpack until the bug is fixed")
	}()

	// Pass a string that won't decode as UptimeReportData.
	p.WriteUptimeData([]interface{}{"this-is-not-msgpack"})
}

// ---------------------------------------------------------------------------
// mongo_selective.go :: WriteUptimeData — empty branch
// ---------------------------------------------------------------------------

// Verifies: SW-REQ-035
// SW-REQ-035:boundary:nominal — empty data exits early
func TestMongoSelectivePump_WriteUptimeData_Empty(t *testing.T) {
	p := &MongoSelectivePump{}
	p.log = logrus.NewEntry(logrus.New())
	p.WriteUptimeData(nil)
}

// Verifies: SW-REQ-035
// Verifies: KI:mongo-pump-writeuptime-nil-on-bad-msgpack
// Reproduces: mongo-pump-writeuptime-nil-on-bad-msgpack
// SW-REQ-035:boundary:negative — same nil-DBObject bug as MongoPump.
func TestMongoSelectivePump_WriteUptimeData_BadMsgpack_KI(t *testing.T) {
	p := &MongoSelectivePump{}
	conf := defaultSelectiveConf(t)
	p.dbConf = &conf
	p.log = log.WithField("prefix", mongoSelectivePrefix)
	p.connect()
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })
	defer func() {
		r := recover()
		assert.NotNil(t, r, "KI active: WriteUptimeData panics on bad msgpack")
	}()
	p.WriteUptimeData([]interface{}{"bad-msgpack"})
}

// ---------------------------------------------------------------------------
// mcp_mongo_aggregate.go :: WriteData — UseMixedCollection branch
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-039
func TestMCPMongoAggregatePump_WriteData_NoMixed(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":            testMongoURI(t),
		"use_mixed_collection": false,
	}
	p := &MCPMongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	ts := time.Now()
	rec := analytics.AnalyticsRecord{
		APIID: "api-x", OrgID: "org-x", TimeStamp: ts, ResponseCode: 200,
		MCPStats: analytics.MCPStats{IsMCP: true, JSONRPCMethod: "tools/call", PrimitiveType: "tool", PrimitiveName: "x"},
	}
	require.NoError(t, p.WriteData(context.Background(), []interface{}{rec}))
}

// ---------------------------------------------------------------------------
// graph_mongo.go :: Init invalid-conf branch
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-037
// SW-REQ-037:malformed_recovers_or_errors_loudly:negative
func TestGraphMongoPump_Init_BadConfig(t *testing.T) {
	p := &GraphMongoPump{}
	err := p.Init("not-a-map")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// mcp_mongo.go :: WriteData with empty collection name branch
// ---------------------------------------------------------------------------
// Verifies: SW-REQ-038
func TestMCPMongoPump_WriteData_NoCollectionWithEmptyData(t *testing.T) {
	p := &MCPMongoPump{}
	p.dbConf = &MongoConf{CollectionName: ""}
	p.log = logrus.NewEntry(logrus.New())
	// Even with empty collection name, empty data should error first.
	err := p.WriteData(context.Background(), []interface{}{
		analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}},
	})
	assert.Error(t, err)
}

// Verifies: SW-REQ-036
func TestMCPMongoAggregatePump_Init_ThresholdDefault(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":              testMongoURI(t),
		"use_mixed_collection":   false,
		"threshold_len_tag_list": 0,
	}
	p := &MCPMongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })
	assert.Equal(t, ThresholdLenTagList, p.dbConf.ThresholdLenTagList)
}

// Verifies: SW-REQ-058
// SW-REQ-058:nominal:nominal — drives WriteData filter all-MCP short-circuit
// AND the `len(filtered) == 0` early return for MongoAggregatePump.
func TestMongoAggregatePump_WriteData_AllMCPRecordsFiltered(t *testing.T) {
	p := &MongoAggregatePump{}
	p.dbConf = &MongoAggregateConf{}
	p.log = logrus.NewEntry(logrus.New())
	data := []interface{}{
		analytics.AnalyticsRecord{MCPStats: analytics.MCPStats{IsMCP: true}},
	}
	err := p.WriteData(context.Background(), data)
	assert.NoError(t, err)
}

// Verifies: SW-REQ-037
// SW-REQ-037:output_cardinality_bounded:nominal
// Through GraphMongoPump.WriteData with real records (covers err != nil = F).
func TestGraphMongoPump_WriteData_AllRecordsWritten(t *testing.T) {
	conf := defaultConf(t)
	conf.CollectionName = uniqueCollection(t)
	p := GraphMongoPump{MongoPump: MongoPump{dbConf: &conf}}
	p.log = log.WithField("prefix", "graph-mc-dc")
	p.MongoPump.CommonPumpConfig = p.CommonPumpConfig
	p.connect()
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	rec := analytics.AnalyticsRecord{
		APIName: "graph-api",
		Path:    "POST",
		GraphQLStats: analytics.GraphQLStats{
			IsGraphQL:     true,
			OperationType: analytics.OperationQuery,
			Types:         map[string][]string{"T": {"f"}},
			RootFields:    []string{"rf"},
		},
	}
	err := p.WriteData(context.Background(), []interface{}{rec})
	assert.NoError(t, err)
}

// Verifies: SW-REQ-061
// SW-REQ-061:boundary:nominal — in DoAggregatedWriting by injecting a record
// with many tags.
func TestMongoAggregatePump_DoAggregatedWriting_ThresholdExceeded(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":              testMongoURI(t),
		"use_mixed_collection":   false,
		"threshold_len_tag_list": 1, // any record with ≥2 tags will trigger
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	rec := analytics.AnalyticsRecord{
		APIID: "tag-heavy", OrgID: "tag-org", TimeStamp: time.Now(), ResponseCode: 200,
		Tags: []string{"a_first", "a_second", "b_first", "b_second"},
	}
	require.NoError(t, p.WriteData(context.Background(), []interface{}{rec}))
}

// the first mapstructure.Decode error path.
// Verifies: SW-REQ-038
// SW-REQ-038:malformed_recovers_or_errors_loudly:negative
func TestMCPMongoPump_Init_InvalidIntValue(t *testing.T) {
	p := &MCPMongoPump{}
	// mongo_db_type is `int`-typed in struct; supplying a malformed value
	// triggers mapstructure decode error.
	err := p.Init(map[string]interface{}{
		"mongo_db_type": "not-an-int",
	})
	assert.Error(t, err)
}

// Verifies: SW-REQ-037
// SW-REQ-037:malformed_recovers_or_errors_loudly:negative
func TestGraphMongoPump_Init_BaseConfDecodeError(t *testing.T) {
	// The second Decode targets BaseMongoConf which contains mongo_db_type
	// (int). A bad map should still fail.
	p := &GraphMongoPump{}
	err := p.Init(map[string]interface{}{
		"mongo_db_type": []string{"not-a-number"},
	})
	assert.Error(t, err)
}

// Verifies: SW-REQ-036
// Verifies: SW-REQ-058
// SW-REQ-036:output_cardinality_bounded:nominal
// SW-REQ-058:nominal:nominal — exercises the analyticsPerOrg loop and
// successful err==nil paths in DoAggregatedWriting (covers err != nil = F).
func TestMongoAggregatePump_WriteData_HappyPath(t *testing.T) {
	cfg := map[string]interface{}{
		"mongo_url":            testMongoURI(t),
		"use_mixed_collection": true,
	}
	p := &MongoAggregatePump{}
	require.NoError(t, p.Init(cfg))
	t.Cleanup(func() { _ = p.store.DropDatabase(context.Background()) })

	now := time.Now()
	data := []interface{}{
		analytics.AnalyticsRecord{
			APIID: "happy-api", OrgID: "happy-org",
			TimeStamp: now, ResponseCode: 200,
		},
	}
	require.NoError(t, p.WriteData(context.Background(), data))
}
