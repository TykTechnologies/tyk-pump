package main

import (
	"context"

	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/stretchr/testify/assert"
)

type MockedPump struct {
	CounterRequest int
	TurnedOff      bool
	pumps.CommonPumpConfig
}

func (p *MockedPump) GetName() string {
	return "Mocked Pump"
}

func (p *MockedPump) New() pumps.Pump {
	return &MockedPump{}
}
func (p *MockedPump) Init(config interface{}) error {
	return nil
}
func (p *MockedPump) WriteData(ctx context.Context, keys []interface{}) error {
	for range keys {
		p.CounterRequest++
	}
	return nil
}

func (p *MockedPump) Shutdown() error {
	p.TurnedOff = true
	return nil
}

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
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedCounterRequest, tc.mockedPump.CounterRequest)
			assert.Len(t, keys, 6)

		})
	}
}

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
