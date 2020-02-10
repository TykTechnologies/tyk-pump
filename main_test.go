package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/pumps"
)

type MockedPump struct {
	CounterRequest int
	filters        analytics.AnalyticsFilters
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
func (p *MockedPump) WriteData(keys []interface{}) error {
	for range keys {
		p.CounterRequest++
	}
	return nil
}
func (p *MockedPump) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *MockedPump) GetFilters() analytics.AnalyticsFilters {
	return p.filters
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

func TestWriteData(t *testing.T) {
	mockedPump := &MockedPump{}
	Pumps = []pumps.Pump{mockedPump}

	keys := make([]interface{}, 3)
	keys[0] = analytics.AnalyticsRecord{APIID: "api111"}
	keys[1] = analytics.AnalyticsRecord{APIID: "api123"}
	keys[2] = analytics.AnalyticsRecord{APIID: "api321"}

	job := instrument.NewJob("TestJob")

	writeToPumps(keys, job, time.Now())

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

	writeToPumps(keys, job, time.Now())

	mockedPump = Pumps[0].(*MockedPump)

	if mockedPump.CounterRequest != 1 {
		fmt.Println(mockedPump.CounterRequest)
		t.Fatal("MockedPump with filter should have 3 requests")
	}
}
