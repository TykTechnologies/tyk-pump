package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
)

type MockedPump struct {
	CounterRequest int
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

func TestDontObfuscateKeysAndFilterData(t *testing.T) {
	SystemConfig = TykPumpConfiguration{}

	//Since it's false no need to set it, added just to explain
	//ObfuscateKeys = false //Log the actual keys, dont obfuscate them

	mockedPump := &MockedPump{}

	mockedPump.SetFilters(
		analytics.AnalyticsFilters{
			SkippedAPIIDs: []string{"api789"},
		},
	)

	expectedKeys := make([]string, 4)
	expectedKeys[0] = "1234"        // len(key) <= 4
	expectedKeys[1] = "12345678910" // key = 12345678910
	expectedKeys[2] = "my-secret"   // key = my-secret
	expectedKeys[3] = ""            // empty key

	keys := make([]interface{}, 5)
	keys[0] = analytics.AnalyticsRecord{APIID: "api123", APIKey: expectedKeys[0]}
	keys[1] = analytics.AnalyticsRecord{APIID: "api456", APIKey: expectedKeys[1]}
	keys[2] = analytics.AnalyticsRecord{APIID: "api123", APIKey: expectedKeys[2]}
	keys[3] = analytics.AnalyticsRecord{APIID: "api321", APIKey: expectedKeys[3]}
	keys[4] = analytics.AnalyticsRecord{APIID: "api789", APIKey: "blabla"} //should be filtered out anyway

	filteredKeys := filterData(mockedPump, keys)
	if len(keys) == len(filteredKeys) {
		t.Fatal("keys and filtered keys should not the same length")
	}

	if len(expectedKeys) != len(filteredKeys) {
		t.Fatal("expected keys and filtered keys must have the  same length")
	}
	for i := 0; i < len(filteredKeys); i++ {
		actual := filteredKeys[i].(analytics.AnalyticsRecord).APIKey
		if actual != expectedKeys[i] {
			t.Errorf("Record #%d Expected %s, actual %s", i, expectedKeys[i], actual)
		}
	}
}

func TestDontObfuscateKeysAndDontFilterData(t *testing.T) {

	SystemConfig = TykPumpConfiguration{}

	//Since it's false no need to set it, added just to explain
	//SystemConfig.ObfuscateKeys = false //Log the actual keys, dont obfuscate them

	mockedPump := &MockedPump{}

	expectedKeys := make([]string, 5)
	expectedKeys[0] = "1234"        // len(key) <= 4
	expectedKeys[1] = "12345678910" // key = 12345678910
	expectedKeys[2] = "my-secret"   // key = my-secret
	expectedKeys[3] = ""            // empty key
	expectedKeys[4] = "1234"        // 1234 key

	keys := make([]interface{}, 5)
	keys[0] = analytics.AnalyticsRecord{APIID: "api123", APIKey: expectedKeys[0]}
	keys[1] = analytics.AnalyticsRecord{APIID: "api456", APIKey: expectedKeys[1]}
	keys[2] = analytics.AnalyticsRecord{APIID: "api123", APIKey: expectedKeys[2]}
	keys[3] = analytics.AnalyticsRecord{APIID: "api321", APIKey: expectedKeys[3]}
	keys[4] = analytics.AnalyticsRecord{APIID: "api789", APIKey: expectedKeys[4]}

	filteredKeys := filterData(mockedPump, keys)
	expectedLen := len(keys)
	actualLen := len(filteredKeys)
	if expectedLen != actualLen {
		t.Fatalf("keys and filtered keys should have the same length. Expected: %d actual %d", expectedLen, actualLen)
	}

	if len(expectedKeys) != len(filteredKeys) {
		t.Fatal("expected keys and filtered keys must have the  same length")
	}
	for i := 0; i < len(filteredKeys); i++ {
		actual := filteredKeys[i].(analytics.AnalyticsRecord).APIKey
		if actual != expectedKeys[i] {
			t.Errorf("Record #%d Expected %s, actual %s", i, expectedKeys[i], actual)
		}
	}
}

func TestObfuscateKeysAndDontFilterData(t *testing.T) {
	SystemConfig = TykPumpConfiguration{}
	SystemConfig.ObfuscateKeys = true

	mockedPump := &MockedPump{}

	expectedKeys := make([]string, 4)
	expectedKeys[0] = "----"     // len(key) <= 4
	expectedKeys[1] = "****8910" // key = 12345678910
	expectedKeys[2] = "****cret" // key = my-secret
	expectedKeys[3] = ""         // empty key

	keys := make([]interface{}, 4)
	keys[0] = analytics.AnalyticsRecord{APIID: "api123", APIKey: "1234"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api456", APIKey: "12345678910"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api123", APIKey: "my-secret"}
	keys[3] = analytics.AnalyticsRecord{APIID: "api321", APIKey: ""}

	filteredKeys := filterData(mockedPump, keys)
	if len(keys) != len(filteredKeys) {
		t.Fatal("keys and filtered keys should have the same length")
	}

	if len(expectedKeys) != len(filteredKeys) {
		t.Fatal("expected keys and filtered keys must have the  same length")
	}
	for i := 0; i < len(filteredKeys); i++ {
		actual := filteredKeys[i].(analytics.AnalyticsRecord).APIKey
		if actual != expectedKeys[i] {
			t.Errorf("Record #%d Expected %s, actual %s", i, expectedKeys[i], actual)
		}
	}
}

func TestObfuscateKeysAndFilterData(t *testing.T) {
	SystemConfig = TykPumpConfiguration{}
	SystemConfig.ObfuscateKeys = true

	mockedPump := &MockedPump{}

	mockedPump.SetFilters(
		analytics.AnalyticsFilters{
			SkippedAPIIDs: []string{"api789"},
		},
	)

	expectedKeys := make([]string, 5)
	expectedKeys[0] = "----"     // len(key) <= 4
	expectedKeys[1] = "****8910" // key = 12345678910
	expectedKeys[2] = "****cret" // key = my-secret
	expectedKeys[3] = ""         // empty key
	expectedKeys[4] = "****2345" // 5 chars key

	keys := make([]interface{}, 6)
	keys[0] = analytics.AnalyticsRecord{APIID: "api123", APIKey: "1234"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api456", APIKey: "12345678910"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api123", APIKey: "my-secret"}
	keys[3] = analytics.AnalyticsRecord{APIID: "api321", APIKey: ""}
	keys[4] = analytics.AnalyticsRecord{APIID: "api321", APIKey: "12345"}
	keys[5] = analytics.AnalyticsRecord{APIID: "api789", APIKey: "blabla"}

	filteredKeys := filterData(mockedPump, keys)
	if len(keys) == len(filteredKeys) {
		t.Fatal("keys and filtered keys should not have the same length")
	}

	if len(expectedKeys) != len(filteredKeys) {
		t.Fatal("expected keys and filtered keys must have the  same length")
	}
	for i := 0; i < len(filteredKeys); i++ {
		actual := filteredKeys[i].(analytics.AnalyticsRecord).APIKey
		if actual != expectedKeys[i] {
			t.Errorf("Record #%d Expected %s, actual %s", i, expectedKeys[i], actual)
		}
	}
}

func TestWriteData(t *testing.T) {
	mockedPump := &MockedPump{}
	Pumps = []pumps.Pump{mockedPump}

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	job := instrument.NewJob("TestJob")

	writeToPumps(keys, job, time.Now(), 2)

	mockedPump = Pumps[0].(*MockedPump)

	if mockedPump.CounterRequest != 3 {
		t.Fatal("MockedPump should have 3 requests")
	}

}

func TestWriteDataWithFilters(t *testing.T) {
	mockedPump := &MockedPump{}
	mockedPump.SetFilters(
		analytics.AnalyticsFilters{
			APIIDs: []string{"api123"},
		},
	)

	Pumps = []pumps.Pump{mockedPump}

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	job := instrument.NewJob("TestJob")

	writeToPumps(keys, job, time.Now(), 2)

	mockedPump = Pumps[0].(*MockedPump)

	if mockedPump.CounterRequest != 1 {
		fmt.Println(mockedPump.CounterRequest)
		t.Fatal("MockedPump with filter should have 3 requests")
	}
}
