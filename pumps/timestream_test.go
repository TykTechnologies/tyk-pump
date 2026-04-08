package pumps

import (
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

func TestChunkString(t *testing.T) {
	tests := []struct {
		testName    string
		inputString string
		inputSize   int
		expected    []string
	}{
		{
			testName:    "SizeZero",
			inputString: "aaa",
			inputSize:   0,
			expected:    []string{"aaa"},
		},
		{
			testName:    "SizeLargerThanInputLenght",
			inputString: "aaa",
			inputSize:   10,
			expected:    []string{"aaa"},
		},
		{
			testName:    "Size1",
			inputString: "aaa",
			inputSize:   1,
			expected:    []string{"a", "a", "a"},
		},
		{
			testName:    "Size2_Odd",
			inputString: "aaa",
			inputSize:   2,
			expected:    []string{"aa", "a"},
		},
		{
			testName:    "Size2_Even",
			inputString: "aaaa",
			inputSize:   2,
			expected:    []string{"aa", "aa"},
		},
		{
			testName:    "SizeEqualsInput",
			inputString: "aaa",
			inputSize:   3,
			expected:    []string{"aaa"},
		},
	}
	for _, v := range tests {

		actual := chunkString(v.inputString, v.inputSize)

		if len(actual) != len(v.expected) {
			t.Fatalf("%v: Expected len %d, got %d", v.testName, len(v.expected), len(actual))
		}
		for i := range v.expected {
			if v.expected[i] != actual[i] {
				t.Fatalf("%v: Expected value %v, in position %d and got %v", v.testName, v.expected[i], i, actual[i])
			}
		}
	}
}

func TestGetAnalyticsRecordMeasuresAndDimensions(t *testing.T) {
	pump := TimestreamPump{}
	cfg := make(map[string]interface{})
	cfg["dimensions"] = []string{"Host", "Source", "Promess", "Something Else"}
	cfg["measures"] = []string{"UserAgent", "Field not in Record"}

	err := pump.Init(cfg)
	if err != nil {
		t.Fatal("Timestream pump couldn't be initialized with err: ", err)
	}

	decoded := analytics.AnalyticsRecord{
		Method:    "GET",
		Host:      "127.0.0.1",
		UserAgent: "Firefox1.0",
	}

	dimensionsActual := pump.GetAnalyticsRecordDimensions(&decoded)
	if len(dimensionsActual) != 1 {
		t.Fatal("Should have 1 dimension in list")
	}

	measuresActual := pump.GetAnalyticsRecordMeasures(&decoded)
	if len(measuresActual) != 1 {
		t.Fatal("Should have 1 measure in list")
	}
}

func TestGetAnalyticsRecordMeasureWithRawResponse(t *testing.T) {
	pump := TimestreamPump{}
	cfg := make(map[string]interface{})
	cfg["dimensions"] = []string{"Host"}
	cfg["measures"] = []string{"RawResponse"}

	err := pump.Init(cfg)
	if err != nil {
		t.Fatal("Timestream pump couldn't be initialized with err: ", err)
	}

	decoded := analytics.AnalyticsRecord{
		Method:      "GET",
		Host:        "127.0.0.1",
		RawResponse: "{abc:Firefox1.0,ppp:WEWEWEW}",
	}

	measuresActual := pump.GetAnalyticsRecordMeasures(&decoded)
	if len(measuresActual) != 2 {
		t.Fatal("Should have 2 measure in list")
	}
}
