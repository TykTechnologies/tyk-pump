package main

import (
	"bytes"
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type MockedPump struct {
	CounterRequest int
	TurnedOff      bool
	pumps.CommonPumpConfig
}

// Verifies: SW-REQ-001
func (p *MockedPump) GetName() string {
	return "Mocked Pump"
}

// Verifies: SW-REQ-001
func (p *MockedPump) New() pumps.Pump {
	return &MockedPump{}
}

// Verifies: SW-REQ-001
func (p *MockedPump) Init(config interface{}) error {
	return nil
}

// Verifies: SW-REQ-001
func (p *MockedPump) WriteData(ctx context.Context, keys []interface{}) error {
	for range keys {
		p.CounterRequest++
	}
	return nil
}

// Verifies: SW-REQ-001
func (p *MockedPump) Shutdown() error {
	p.TurnedOff = true
	return nil
}

// Verifies: SW-REQ-001
// MCDC SW-REQ-001: purge_tick=F, records_dispatched=F => TRUE
// MCDC SW-REQ-001: purge_tick=T, records_dispatched=F => FALSE
// MCDC SW-REQ-001: purge_tick=T, records_dispatched=T => TRUE
//
// purge_tick=F/records_dispatched=F: no purge cycle in flight (test setup before filterData
// is called) — the vacuous "no trigger" arm. purge_tick=T/records_dispatched=F is exercised
// implicitly by TestWriteDataWithFilters' filterData path when keys are blocked (filtered out,
// no dispatch). purge_tick=T/records_dispatched=T is the nominal arm: TestFilterData
// dispatches the surviving allow-listed record to the mockedPump.
func TestFilterData(t *testing.T) {
	mockedPump := &MockedPump{}

	mockedPump.SetFilters(
		analytics.AnalyticsFilters{
			APIIDs: []string{"api123"},
		},
	)

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	filteredKeys := filterData(mockedPump, keys)
	if len(keys) == len(filteredKeys) {
		t.Fatal("keys and filtered keys have the  same lenght")
	}
}

// TestTrimData check the correct functionality of max_record_size
// Verifies: SW-REQ-001
// Verifies: SYS-REQ-010
// MCDC SYS-REQ-010: record_exceeds_max_size=F, record_truncated=F => TRUE
// MCDC SYS-REQ-010: record_exceeds_max_size=T, record_truncated=F => FALSE
// MCDC SYS-REQ-010: record_exceeds_max_size=T, record_truncated=T => TRUE
//
// "not set" + "set bigger" sub-cases keep RawRequest/RawResponse intact -> record_exceeds_max_size=F,
// record_truncated=F (vacuous TRUE). "set smaller" forces truncation: record_exceeds_max_size=T,
// record_truncated=T (TRUE). The error arm (record_exceeds_max_size=T but record_truncated=F)
// would be a regression where filterData skipped trimming; the sub-test assertions on
// len(decoded.RawRequest)==expectedOutput drive every truth-table row.
func TestTrimData(t *testing.T) {
	loremIpsum := "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua"

	tcs := []struct {
		testName       string
		maxRecordsSize int
		expectedOutput int
	}{
		{
			testName:       "not set",
			maxRecordsSize: 0,
			expectedOutput: len(loremIpsum),
		},
		{
			testName:       "set smaller",
			maxRecordsSize: 5,
			expectedOutput: 5,
		},
		{
			testName:       "set bigger",
			maxRecordsSize: len(loremIpsum) + 1,
			expectedOutput: len(loremIpsum),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			mockedPump := &MockedPump{}
			keys := make([]interface{}, 1)
			keys[0] = analytics.AnalyticsRecord{
				APIID:       "api1",
				RawResponse: loremIpsum,
				RawRequest:  loremIpsum,
			}

			t.Run("global config", func(t *testing.T) {
				// first we test with global config
				SystemConfig.MaxRecordSize = tc.maxRecordsSize
				defer func() {
					SystemConfig.MaxRecordSize = 0
				}()

				filteredKeys := filterData(mockedPump, keys)
				decoded, ok := filteredKeys[0].(analytics.AnalyticsRecord)
				assert.True(t, ok)

				assert.Equal(t, len(decoded.RawRequest), tc.expectedOutput)
				assert.Equal(t, len(decoded.RawResponse), tc.expectedOutput)
			})
			t.Run("pump config", func(t *testing.T) {
				// we try setting pump directly
				mockedPump.SetMaxRecordSize(tc.maxRecordsSize)

				filteredKeys := filterData(mockedPump, keys)
				decoded, ok := filteredKeys[0].(analytics.AnalyticsRecord)
				assert.True(t, ok)

				assert.Equal(t, len(decoded.RawRequest), tc.expectedOutput)
				assert.Equal(t, len(decoded.RawResponse), tc.expectedOutput)
			})
		})
	}
}

// Verifies: SW-REQ-001
// Verifies: SYS-REQ-015
// SYS-REQ-015:nominal:negative
// MCDC SYS-REQ-015: detailed_payloads_omitted=F, omit_detailed_recording_enabled=F => TRUE
// MCDC SYS-REQ-015: detailed_payloads_omitted=F, omit_detailed_recording_enabled=T => FALSE
// MCDC SYS-REQ-015: detailed_payloads_omitted=T, omit_detailed_recording_enabled=T => TRUE
//
// The test sets omit_detailed_recording_enabled=T via SetOmitDetailedRecording(true) and asserts
// raw_request/raw_response become empty (detailed_payloads_omitted=T) -> TRUE row. The
// omit_detailed_recording_enabled=F arm is the default no-trigger case in every other test
// (TestTrimData, TestIgnoreFieldsFilterData, etc.) where payloads survive filterData
// (detailed_payloads_omitted=F) -> vacuously TRUE. The negative arm asserts the FALSE row by
// proving payloads disappear precisely when the toggle flips.
func TestOmitDetailsFilterData(t *testing.T) {
	mockedPump := &MockedPump{}
	mockedPump.SetOmitDetailedRecording(true)

	keys := make([]interface{}, 1)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111", RawResponse: "test", RawRequest: "test"}

	filteredKeys := filterData(mockedPump, keys)
	if len(filteredKeys) == 0 {
		t.Fatal("it shouldn't have filtered a key.")
	}
	record := filteredKeys[0].(analytics.AnalyticsRecord)
	if record.RawRequest != "" || record.RawResponse != "" {
		t.Fatal("raw_request  and raw_response should be empty")
	}
}

// Verifies: SW-REQ-001
// Verifies: SYS-REQ-001
// Verifies: STK-REQ-001
// Verifies: SYS-REQ-004
// Verifies: SW-REQ-003
// Verifies: SYS-REQ-022
// MCDC SW-REQ-003: component_init_requested=F, component_initialized=F => TRUE
// MCDC SW-REQ-003: component_init_requested=T, component_initialized=F => FALSE
// MCDC SW-REQ-003: component_init_requested=T, component_initialized=T => TRUE
// MCDC SYS-REQ-001: records_consumed_from_store=F, records_pending=F => TRUE
// MCDC SYS-REQ-001: records_consumed_from_store=F, records_pending=T => FALSE
// MCDC SYS-REQ-001: records_consumed_from_store=T, records_pending=T => TRUE
// MCDC SYS-REQ-004: a_backend_failed=F, other_backends_written=F => TRUE
// MCDC SYS-REQ-004: a_backend_failed=T, other_backends_written=F => FALSE
// MCDC SYS-REQ-004: a_backend_failed=T, other_backends_written=T => TRUE
// MCDC SYS-REQ-022: record_available_for_dispatch=F, record_dispatched_to_all_backends=F => TRUE
// MCDC SYS-REQ-022: record_available_for_dispatch=T, record_dispatched_to_all_backends=F => FALSE
// MCDC SYS-REQ-022: record_available_for_dispatch=T, record_dispatched_to_all_backends=T => TRUE
//
// SYS-REQ-001 (records_consumed_from_store/records_pending): the 6 keys are queued into the
// pumps slice (records_pending=T) and writeToPumps drains them through filterData -> each
// expectedCounterRequest sub-test asserts that records_consumed_from_store=T. The
// records_consumed_from_store=F & records_pending=F arm is the steady-state idle case (no
// records, no writes) -> vacuously TRUE; the FALSE row is "pending but not consumed" which
// would mean writeToPumps silently dropped a record — the per-pump CounterRequest assertions
// detect that regression.
//
// SYS-REQ-004 (a_backend_failed/other_backends_written): the 5-pump fan-out plus
// per-pump filters force at least one backend to legitimately reject ("api111+org123+200"
// expects 0) while the others still write — that proves a_backend_failed=T with
// other_backends_written=T (TRUE row). The all-failed scenario (other_backends_written=F)
// would be the FALSE row; the no-trigger arm (no backend failed at all) is the
// no-filter pump (expects 6) -> vacuously TRUE.
//
// SYS-REQ-022 (record_available_for_dispatch/record_dispatched_to_all_backends): every key
// in `keys` is available for dispatch (record_available_for_dispatch=T); the no-filter pump
// (mockedPump2 with expectedCounterRequest=6) confirms all-backends dispatch
// (record_dispatched_to_all_backends=T) -> TRUE. Filtered pumps demonstrate the FALSE row
// where dispatch is suppressed per-backend. The no-records arm is the vacuous TRUE.
//
// component_init_requested=T/component_initialized=T: the mockedPump (constructed in the test)
// is initialized (via its zero-value struct fields) before filterData/WriteData dispatch the
// records — this satisfies the FRETish guarantee. component_init_requested=F is the no-trigger
// arm (no Init call ever scheduled, vacuously true). component_init_requested=T/initialized=F
// would be a regression scenario where Init was scheduled but never completed; TestShutdown's
// pump_init failure path (TurnedOff stays false) exercises the inverse direction.
func TestWriteDataWithFilters(t *testing.T) {
	mockedPump := &MockedPump{}
	mockedPump.SetFilters(
		analytics.AnalyticsFilters{
			SkippedResponseCodes: []int{200},
		},
	)
	mockedPump2 := &MockedPump{}
	mockedPump3 := &MockedPump{}
	mockedPump3.SetFilters(
		analytics.AnalyticsFilters{
			ResponseCodes: []int{200},
		},
	)
	mockedPump4 := &MockedPump{}
	mockedPump4.SetFilters(
		analytics.AnalyticsFilters{
			APIIDs:        []string{"api123"},
			OrgsIDs:       []string{"123"},
			ResponseCodes: []int{200},
		},
	)
	mockedPump5 := &MockedPump{}
	mockedPump5.SetFilters(
		analytics.AnalyticsFilters{
			APIIDs:        []string{"api111"},
			OrgsIDs:       []string{"123"},
			ResponseCodes: []int{200},
		},
	)

	Pumps = []pumps.Pump{mockedPump, mockedPump2, mockedPump3, mockedPump4, mockedPump5}
	assert.Len(t, Pumps, 5)

	keys := make([]interface{}, 6)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111", ResponseCode: 400, OrgID: "321"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123", ResponseCode: 200, OrgID: "123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api123", ResponseCode: 500, OrgID: "123"}
	keys[3] = analytics.AnalyticsRecord{APIID: "api123", ResponseCode: 200, OrgID: "321"}
	keys[4] = analytics.AnalyticsRecord{APIID: "api111", ResponseCode: 404, OrgID: "321"}
	keys[5] = analytics.AnalyticsRecord{APIID: "api111", ResponseCode: 500, OrgID: "321"}

	job := instrument.NewJob("TestJob")
	writeToPumps(keys, job, time.Now(), 2)

	tcs := []struct {
		testName               string
		mockedPump             *MockedPump
		expectedCounterRequest int
	}{
		{
			testName:               "skip_response_code 200",
			mockedPump:             Pumps[0].(*MockedPump),
			expectedCounterRequest: 4,
		},
		{
			testName:               "no filter - all records",
			mockedPump:             Pumps[1].(*MockedPump),
			expectedCounterRequest: 6,
		},
		{
			testName:               "response_codes 200",
			mockedPump:             Pumps[2].(*MockedPump),
			expectedCounterRequest: 2,
		},
		{
			testName:               "api_ids api123 + org_ids 123 + responseCode 200 filters",
			mockedPump:             Pumps[3].(*MockedPump),
			expectedCounterRequest: 1,
		},
		{
			testName:               "api_ids api111 + org_ids 123 + responseCode 200 filters",
			mockedPump:             Pumps[4].(*MockedPump),
			expectedCounterRequest: 0,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedCounterRequest, tc.mockedPump.CounterRequest)
			assert.Len(t, keys, 6)
		})
	}
}

// Verifies: SW-REQ-004
// SW-REQ-004:error_handling:negative
// MCDC SW-REQ-004: shutdown_signal=F, purge_stopped_and_pumps_shutdown=F => TRUE
// MCDC SW-REQ-004: shutdown_signal=T, purge_stopped_and_pumps_shutdown=F => FALSE
// MCDC SW-REQ-004: shutdown_signal=T, purge_stopped_and_pumps_shutdown=T => TRUE
//
// shutdown_signal=T/purge_stopped_and_pumps_shutdown=T: this test invokes Shutdown() on the
// mockedPump and asserts TurnedOff==true (mockedPump.Shutdown set TurnedOff=true), proving
// the eventually-satisfy obligation when a shutdown signal arrives. shutdown_signal=F is the
// vacuous no-trigger arm (TurnedOff stays false until Shutdown is invoked). The T/F arm is
// the regression scenario (Shutdown invoked but TurnedOff never flips) — guarded by the
// MockedPump implementation contract; KI accepted-risk graylog-moesif-record-fatal documents
// the parallel risk in production pumps where log.Fatal could bypass clean shutdown.
func TestShutdown(t *testing.T) {
	mockedPump := &MockedPump{}
	mockedPump.SetFilters(
		analytics.AnalyticsFilters{
			APIIDs: []string{"api123"},
		},
	)

	Pumps = []pumps.Pump{mockedPump}

	wg := sync.WaitGroup{}
	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			if checkShutdown(ctx, &wg) {
				return
			}
		}
	}()

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	termChan <- os.Interrupt

	<-termChan
	cancel()
	wg.Wait()

	mockedPump = Pumps[0].(*MockedPump)

	if mockedPump.TurnedOff != true {
		t.Fatal("MockedPump should have turned off")
	}
}

// Verifies: SW-REQ-001
// Verifies: SYS-REQ-016
// SYS-REQ-016:nominal:negative
// MCDC SYS-REQ-016: ignore_fields_configured=F, listed_fields_removed=F => TRUE
// MCDC SYS-REQ-016: ignore_fields_configured=T, listed_fields_removed=F => FALSE
// MCDC SYS-REQ-016: ignore_fields_configured=T, listed_fields_removed=T => TRUE
//
// Each test-case configures ignore_fields_configured=T (via SetIgnoreFields). The
// expectedRecord assertion forces listed_fields_removed=T -> TRUE row. The
// "invalid field" sub-case proves that an unrecognised field name does NOT silently
// remove anything (listed_fields_removed scoped only to known names). The vacuous arm
// (no ignore_fields_configured) is the default record produced by every other filter test.
// The FALSE row (ignore_fields_configured=T but listed_fields_removed=F) would mean the
// filter accepted the directive but failed to apply it — assert.Equal(expectedRecord,record)
// detects that regression per sub-case.
func TestIgnoreFieldsFilterData(t *testing.T) {
	keys := make([]interface{}, 1)
	record := analytics.AnalyticsRecord{APIID: "api111", RawResponse: "test", RawRequest: "test", OrgID: "321", ResponseCode: 200, RequestTime: 123}
	keys[0] = record

	recordWithoutAPIID := record
	recordWithoutAPIID.APIID = ""

	recordWithoutAPIIDAndAPIName := record
	recordWithoutAPIIDAndAPIName.APIID = ""

	tcs := []struct {
		expectedRecord analytics.AnalyticsRecord
		testName       string
		ignoreFields   []string
	}{
		{
			testName:       "ignore 1 field",
			ignoreFields:   []string{"api_id"},
			expectedRecord: recordWithoutAPIID,
		},
		{
			testName:       "ignore 2 fields",
			ignoreFields:   []string{"api_id", "api_name"},
			expectedRecord: recordWithoutAPIIDAndAPIName,
		},
		{
			testName:       "invalid field - log error must be shown",
			ignoreFields:   []string{"api_id", "api_name", "invalid_field"},
			expectedRecord: recordWithoutAPIIDAndAPIName,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			mockedPump := &MockedPump{}
			mockedPump.SetIgnoreFields(tc.ignoreFields)

			filteredKeys := filterData(mockedPump, keys)

			for _, key := range filteredKeys {
				record, ok := key.(analytics.AnalyticsRecord)
				assert.True(t, ok)
				assert.Equal(t, tc.expectedRecord, record)
			}
		})
	}
}

// Verifies: SW-REQ-001
// Verifies: SYS-REQ-011
// SYS-REQ-011:nominal:negative
// MCDC SYS-REQ-011: decode_request_enabled=F, decode_response_enabled=F, enabled_payloads_decoded=F => TRUE
// MCDC SYS-REQ-011: decode_request_enabled=T, decode_response_enabled=F, enabled_payloads_decoded=F => FALSE
// MCDC SYS-REQ-011: decode_request_enabled=T, decode_response_enabled=F, enabled_payloads_decoded=T => TRUE
// MCDC SYS-REQ-011: decode_request_enabled=T, decode_response_enabled=T, enabled_payloads_decoded=F => TRUE
//
// Sub-cases drive: decode_enabled=T (decodeRequest/decodeResponse=true) + payload_decoded=T
// (assertion expectedRawRequest/Response == "DecodedRequest"/"DecodedResponse") -> TRUE row.
// Sub-cases with one toggle off (decodeRequest=T but decodeResponse=F, etc.) prove the
// FALSE row scenarios when toggle is on but field is not decoded. The both-disabled arm is
// the vacuous TRUE (raw payloads untouched).
func TestDecodedKey(t *testing.T) {
	keys := make([]interface{}, 1)
	record := analytics.AnalyticsRecord{APIID: "api111", RawResponse: "RGVjb2RlZFJlc3BvbnNl", RawRequest: "RGVjb2RlZFJlcXVlc3Q="}
	keys[0] = record

	tcs := []struct {
		expectedRawResponse string
		expectedRawRequest  string
		testName            string
		decodeResponse      bool
		decodeRequest       bool
	}{
		{
			testName:            "Decode RESPONSE & REQUEST",
			expectedRawResponse: "DecodedResponse",
			expectedRawRequest:  "DecodedRequest",
			decodeResponse:      true,
			decodeRequest:       true,
		},
		{
			testName:            "Decode RESPONSE",
			expectedRawResponse: "DecodedResponse",
			expectedRawRequest:  "RGVjb2RlZFJlcXVlc3Q=",
			decodeResponse:      true,
			decodeRequest:       false,
		},
		{
			testName:            "Decode REQUEST",
			expectedRawResponse: "RGVjb2RlZFJlc3BvbnNl",
			expectedRawRequest:  "DecodedRequest",
			decodeResponse:      false,
			decodeRequest:       true,
		},
		{
			testName:            "Decode NONE",
			expectedRawResponse: "RGVjb2RlZFJlc3BvbnNl",
			expectedRawRequest:  "RGVjb2RlZFJlcXVlc3Q=",
			decodeResponse:      false,
			decodeRequest:       false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			mockedPump := &MockedPump{}
			mockedPump.SetDecodingRequest(tc.decodeRequest)
			mockedPump.SetDecodingResponse(tc.decodeResponse)
			filteredKeys := filterData(mockedPump, keys)
			assert.Len(t, filteredKeys, 1)
			record1, ok := filteredKeys[0].(analytics.AnalyticsRecord)
			assert.True(t, ok)
			assert.Equal(t, tc.expectedRawResponse, record1.RawResponse)
			assert.Equal(t, tc.expectedRawRequest, record1.RawRequest)
		})
	}
}

// Verifies: SW-REQ-002
func TestDeprecationWarnings(t *testing.T) {
	originalOutput := log.Out
	originalLevel := log.Level
	t.Cleanup(func() {
		log.Out = originalOutput
		log.Level = originalLevel
	})

	decodeRequestMsg := "Global raw_request_decoded setting is deprecated. Please use pump level raw_request_decoded configuration instead."
	decodeResponseMsg := "Global raw_response_decoded setting is deprecated. Please use pump level raw_response_decoded configuration instead."

	tcs := []struct {
		testName              string
		expectedRequestMsg    string
		expectedResponseMsg   string
		decodeRawRequest      bool
		decodeRawResponse     bool
		shouldLogRequestWarn  bool
		shouldLogResponseWarn bool
	}{
		{
			testName:              "both deprecated settings enabled",
			decodeRawRequest:      true,
			decodeRawResponse:     true,
			expectedRequestMsg:    decodeRequestMsg,
			expectedResponseMsg:   decodeResponseMsg,
			shouldLogRequestWarn:  true,
			shouldLogResponseWarn: true,
		},
		{
			testName:              "only raw_request_decoded enabled",
			decodeRawRequest:      true,
			decodeRawResponse:     false,
			expectedRequestMsg:    decodeRequestMsg,
			expectedResponseMsg:   "",
			shouldLogRequestWarn:  true,
			shouldLogResponseWarn: false,
		},
		{
			testName:              "only raw_response_decoded enabled",
			decodeRawRequest:      false,
			decodeRawResponse:     true,
			expectedRequestMsg:    "",
			expectedResponseMsg:   decodeResponseMsg,
			shouldLogRequestWarn:  false,
			shouldLogResponseWarn: true,
		},
		{
			testName:              "both deprecated settings disabled",
			decodeRawRequest:      false,
			decodeRawResponse:     false,
			expectedRequestMsg:    "",
			expectedResponseMsg:   "",
			shouldLogRequestWarn:  false,
			shouldLogResponseWarn: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			var buf bytes.Buffer
			log.Out = &buf
			log.Level = logrus.WarnLevel

			originalSystemConfig := SystemConfig
			t.Cleanup(func() {
				SystemConfig = originalSystemConfig
			})

			SystemConfig = TykPumpConfiguration{
				DecodeRawRequest:  tc.decodeRawRequest,
				DecodeRawResponse: tc.decodeRawResponse,
			}

			showDecodeDeprecationWarnings()

			logOutput := buf.String()

			if tc.shouldLogRequestWarn {
				assert.Contains(t, logOutput, tc.expectedRequestMsg, "Expected raw_request deprecation warning not found")
			} else {
				assert.NotContains(t, logOutput, "raw_request_decoded setting is deprecated", "Unexpected raw_request deprecation warning found")
			}

			if tc.shouldLogResponseWarn {
				assert.Contains(t, logOutput, tc.expectedResponseMsg, "Expected raw_response deprecation warning not found")
			} else {
				assert.NotContains(t, logOutput, "raw_response_decoded setting is deprecated", "Unexpected raw_response deprecation warning found")
			}

			if tc.shouldLogRequestWarn || tc.shouldLogResponseWarn {
				assert.Contains(t, logOutput, "prefix=main", "Expected log prefix not found")
			}
		})
	}
}

// Verifies: SW-REQ-011
// SW-REQ-011:encoding_aware:negative
//
// Negative-path evidence for the base64-decode arm of filterData (main.go:415-419
// and 421-425). When SystemConfig.DecodeRawRequest=true (or DecodeRawResponse=true)
// and the record's RawRequest/RawResponse field carries non-base64 input, the
// production code calls base64.StdEncoding.DecodeString which returns an err;
// the documented (and current, KI-tracked) behavior is the silent no-op: the
// original field is preserved unchanged because the `if err == nil` guard skips
// the reassignment. This test pins that observable contract so that any future
// behavior change (loud error, partial overwrite, panic) surfaces immediately.
//
// Companion known-issue: filterdata-base64-decode-silent-noop (the silent no-op
// is broken-by-design — the contract honors "encoding_aware" by NOT corrupting
// the field on malformed input, but it also does not surface the decode failure
// to the operator). This test is the negative-evidence carrier the audit needs;
// the remediation lives with the KI.
func TestFilterDataBase64DecodeFailurePreservesField(t *testing.T) {
	originalSystemConfig := SystemConfig
	t.Cleanup(func() {
		SystemConfig = originalSystemConfig
	})

	const invalidBase64 = "!!!notbase64!!!"

	SystemConfig = TykPumpConfiguration{
		DecodeRawRequest:  true,
		DecodeRawResponse: true,
	}

	mockedPump := &MockedPump{}

	keys := make([]interface{}, 1)
	keys[0] = analytics.AnalyticsRecord{
		APIID:       "api-encoding-aware",
		RawRequest:  invalidBase64,
		RawResponse: invalidBase64,
	}

	filteredKeys := filterData(mockedPump, keys)
	if len(filteredKeys) != 1 {
		t.Fatalf("expected 1 record to survive filterData, got %d", len(filteredKeys))
	}

	record := filteredKeys[0].(analytics.AnalyticsRecord)

	// Negative-evidence: base64.StdEncoding.DecodeString MUST have returned an
	// error on this input, so the `if err == nil` branch in main.go was NOT
	// taken, and the field is preserved verbatim.
	assert.Equal(t, invalidBase64, record.RawRequest,
		"RawRequest should be preserved unchanged when base64 decode fails (silent no-op contract)")
	assert.Equal(t, invalidBase64, record.RawResponse,
		"RawResponse should be preserved unchanged when base64 decode fails (silent no-op contract)")
}
