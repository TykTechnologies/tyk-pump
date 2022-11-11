package analytics

import (
	"testing"

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

			assert.Equal(t, tt.record, tt.expectedRecord)
		})
	}
}
