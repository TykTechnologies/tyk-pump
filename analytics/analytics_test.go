package analytics

import (
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

	t.Run("should return true when tags contain the graph analytics tag", func(t *testing.T) {
		record := AnalyticsRecord{
			Tags: []string{"tag_1", "tag_2", PredefinedTagGraphAnalytics, "tag_4", "tag_5"},
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

	assert.Equal(t, 39, len(fields))

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

	assert.Equal(t, 39, len(fields))

	for _, field := range structs.Fields(rec) {
		if field.IsExported() && !field.IsZero() {
			assert.Contains(t, fields, fmt.Sprint(field.Value()))
		}
	}
}
