package analytics

import (
	"testing"
)

func TestGetFieldNames(t *testing.T) {
	record := AnalyticsRecord{}

	totalFields := len(record.GetFieldNames())
	if totalFields != 29 {
		t.Fatal("GetFieldNames should return 29 names")
	}
}

func TestGetLineValues(t *testing.T) {
	record := AnalyticsRecord{Method: "POST"}

	values := record.GetLineValues()

	if len(values) != 1 && values[0] != "POST" {
		t.Fatal("GetLineValues should return 1 string and it should be POST.")
	}
}
