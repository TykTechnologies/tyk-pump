package pumps

import (
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

func TestChunkString_SizeZero(t *testing.T) {
	inputString := "aaa"
	inputSize := 0

	actual := chunkString(inputString, inputSize)

	if len(actual) != 1 {
		t.Fatal("Should return a size 1 array with the input")
	}
	if actual[0] != inputString {
		t.Fatal("Should return a size 1 array with the input")
	}
}
func TestChunkString_SizeLargerThanInputLenght(t *testing.T) {
	inputString := "aaa"
	inputSize := 10

	actual := chunkString(inputString, inputSize)
	if len(actual) != 1 {
		t.Fatal("Should return a size 1 array with the input")
	}
	if actual[0] != inputString {
		t.Fatal("Should return a size 1 array with the input")
	}
}
func TestChunkString1(t *testing.T) {
	inputString := "aaa"
	inputSize := 1
	expected := []string{"a", "a", "a"}

	actual := chunkString(inputString, inputSize)

	if len(actual) != len(expected) {
		t.Fatalf("Expected len %d, got %d", len(expected), len(actual))
	}
	for i := range expected {
		if expected[i] != actual[i] {
			t.Fatalf("Expected value %v, in position %d and got %v", expected[i], i, actual[i])
		}
	}
}
func TestChunkString2(t *testing.T) {
	inputString := "aaa"
	inputSize := 2
	expected := []string{"aa", "a"}

	actual := chunkString(inputString, inputSize)

	if len(actual) != len(expected) {
		t.Fatalf("Expected len %d, got %d", len(expected), len(actual))
	}
	for i := range expected {
		if expected[i] != actual[i] {
			t.Fatalf("Expected value %v, in position %d and got %v", expected[i], i, actual[i])
		}
	}
}
func TestChunkString2pair(t *testing.T) {
	inputString := "aaaa"
	inputSize := 2
	expected := []string{"aa", "aa"}

	actual := chunkString(inputString, inputSize)

	if len(actual) != len(expected) {
		t.Fatalf("Expected len %d, got %d", len(expected), len(actual))
	}
	for i := range expected {
		if expected[i] != actual[i] {
			t.Fatalf("Expected value %v, in position %d and got %v", expected[i], i, actual[i])
		}
	}
}
func TestChunkString3(t *testing.T) {
	inputString := "aaa"
	inputSize := 3
	expected := []string{"aaa"}

	actual := chunkString(inputString, inputSize)

	if len(actual) != len(expected) {
		t.Fatalf("Expected len %d, got %d", len(expected), len(actual))
	}
	for i := range expected {
		if expected[i] != actual[i] {
			t.Fatalf("Expected value %v, in position %d and got %v", expected[i], i, actual[i])
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

	dimensionsActual := GetAnalyticsRecordDimensions(&pump, &decoded)
	if len(dimensionsActual) != 1 {
		t.Fatal("Should have 1 dimension in list")
	}

	measuresActual := GetAnalyticsRecordMeasures(&pump, &decoded)
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

	measuresActual := GetAnalyticsRecordMeasures(&pump, &decoded)
	if len(measuresActual) != 2 {
		t.Fatal("Should have 2 measure in list")
	}
	// for _, v := range measuresActual {
	// 	t.Logf("%v", *v.Name)
	// 	t.Logf("%v", *v.Value)
	// }
}
