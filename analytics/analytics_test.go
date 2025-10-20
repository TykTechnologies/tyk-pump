package analytics

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/fatih/structs"
	"github.com/stretchr/testify/assert"
)

func TestAnalyticsRecord_IsGraphRecord(t *testing.T) {
	t.Run("should return false when no tags are available", func(t *testing.T) {
		record := AnalyticsRecord{}
		assert.False(t, record.IsGraphRecord())
	})

	t.Run("should return false when tags do not contain the graph analytics tag", func(t *testing.T) {
		record := AnalyticsRecord{
			Tags: []string{"tag_1", "tag_2", "tag_3"},
		}
		assert.False(t, record.IsGraphRecord())
	})

	t.Run("should return true with graph stats", func(t *testing.T) {
		record := AnalyticsRecord{
			GraphQLStats: GraphQLStats{
				IsGraphQL: true,
			},
		}
		assert.True(t, record.IsGraphRecord())
	})
}

func TestAnalyticsRecord_RemoveIgnoredFields(t *testing.T) {
	defaultRecord := AnalyticsRecord{
		APIID:      "api123",
		APIKey:     "api_key_123",
		OrgID:      "org_123",
		APIName:    "api_name_123",
		APIVersion: "v1",
	}

	recordWithoutAPIID := defaultRecord
	recordWithoutAPIID.APIID = ""

	recordWithoutAPIKeyAndAPIID := defaultRecord
	recordWithoutAPIKeyAndAPIID.APIKey = ""
	recordWithoutAPIKeyAndAPIID.APIID = ""

	type args struct {
		ignoreFields []string
	}
	tests := []struct {
		name           string
		record         AnalyticsRecord
		expectedRecord AnalyticsRecord
		args           args
	}{
		{
			name:           "should remove ignored APIID field",
			record:         defaultRecord,
			expectedRecord: recordWithoutAPIID,
			args: args{
				ignoreFields: []string{"api_id"},
			},
		},
		{
			name:           "should remove ignored APIID and APIKey fields",
			record:         defaultRecord,
			expectedRecord: recordWithoutAPIKeyAndAPIID,
			args: args{
				ignoreFields: []string{"api_id", "api_key"},
			},
		},
		{
			name:           "should remove valid fields and ignore invalid fields",
			record:         defaultRecord,
			expectedRecord: recordWithoutAPIKeyAndAPIID,
			args: args{
				ignoreFields: []string{"api_id", "api_key", "invalid_field"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.record.RemoveIgnoredFields(tt.args.ignoreFields)

			assert.Equal(t, tt.expectedRecord, tt.record)
		})
	}
}

func TestAnalyticsRecord_Base(t *testing.T) {
	rec := &AnalyticsRecord{}

	assert.Equal(t, SQLTable, rec.TableName())

	newID := model.NewObjectID()
	rec.SetObjectID(newID)
	assert.Equal(t, newID, rec.GetObjectID())
}

func TestAnalyticsRecord_GetFieldNames(t *testing.T) {
	rec := &AnalyticsRecord{}

	fields := rec.GetFieldNames()

	assert.Equal(t, 40, len(fields))

	expectedFields := []string{
		"Method",
		"Host",
		"Path",
		"RawPath",
		"ContentLength",
		"UserAgent",
		"Day",
		"Month",
		"Year",
		"Hour",
		"ResponseCode",
		"APIKey",
		"TimeStamp",
		"APIVersion",
		"APIName",
		"APIID",
		"OrgID",
		"OauthID",
		"RequestTime",
		"RawRequest",
		"RawResponse",
		"IPAddress",
		"Tags", "Alias", "TrackPath", "ExpireAt", "ApiSchema",
		"GeoData.Country.ISOCode",
		"GeoData.City.GeoNameID",
		"GeoData.City.Names",
		"GeoData.Location.Latitude",
		"GeoData.Location.Longitude",
		"GeoData.Location.TimeZone",
		"Latency.Total",
		"Latency.Upstream",
		"Latency.Gateway",
		"NetworkStats.OpenConnections",
		"NetworkStats.ClosedConnection",
		"NetworkStats.BytesIn",
		"NetworkStats.BytesOut",
	}

	for _, expected := range expectedFields {
		assert.Contains(t, fields, expected)
	}
}

func TestAnalyticsRecord_GetLineValues(t *testing.T) {
	rec := &AnalyticsRecord{
		APIID:      "api123",
		OrgID:      "org123",
		APIKey:     "key123",
		Path:       "/path",
		RawPath:    "/rawpath",
		APIVersion: "v1",
		APIName:    "api_name",
		TimeStamp:  time.Now(),
		ApiSchema:  "http",
	}

	fields := rec.GetLineValues()

	assert.Equal(t, 40, len(fields))

	for _, field := range structs.Fields(rec) {
		if field.IsExported() && !field.IsZero() {
			assert.Contains(t, fields, fmt.Sprint(field.Value()))
		}
	}
}

func TestLatency_GetFieldNames(t *testing.T) {
	latency := &Latency{}
	fieldNames := latency.GetFieldNames()

	expectedFields := []string{
		"Latency.Total",
		"Latency.Upstream",
		"Latency.Gateway",
	}

	assert.Equal(t, expectedFields, fieldNames)
	assert.Len(t, fieldNames, 3)
}

func TestLatency_GetLineValues(t *testing.T) {
	tcs := []struct {
		testName     string
		latency      Latency
		expectedVals []string
	}{
		{
			testName: "all zero values",
			latency: Latency{
				Total:    0,
				Upstream: 0,
				Gateway:  0,
			},
			expectedVals: []string{"0", "0", "0"},
		},
		{
			testName: "all positive values",
			latency: Latency{
				Total:    100,
				Upstream: 80,
				Gateway:  20,
			},
			expectedVals: []string{"100", "80", "20"},
		},
		{
			testName: "mixed values",
			latency: Latency{
				Total:    150,
				Upstream: 120,
				Gateway:  30,
			},
			expectedVals: []string{"150", "120", "30"},
		},
		{
			testName: "large values",
			latency: Latency{
				Total:    999999,
				Upstream: 888888,
				Gateway:  111111,
			},
			expectedVals: []string{"999999", "888888", "111111"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			lineValues := tc.latency.GetLineValues()
			assert.Equal(t, tc.expectedVals, lineValues)
			assert.Len(t, lineValues, 3)
		})
	}
}

func TestLatency_Struct(t *testing.T) {
	// Test that the Latency struct has the Gateway field
	latency := Latency{
		Total:    100,
		Upstream: 80,
		Gateway:  20,
	}

	assert.Equal(t, int64(100), latency.Total)
	assert.Equal(t, int64(80), latency.Upstream)
	assert.Equal(t, int64(20), latency.Gateway)
}

func TestLatency_JSONSerialization(t *testing.T) {
	latency := Latency{
		Total:    100,
		Upstream: 80,
		Gateway:  20,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(latency)
	assert.NoError(t, err)

	// Verify the JSON contains the gateway field
	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"total":100`)
	assert.Contains(t, jsonStr, `"upstream":80`)
	assert.Contains(t, jsonStr, `"gateway":20`)

	// Test JSON unmarshaling
	var unmarshaled Latency
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, latency, unmarshaled)
}
