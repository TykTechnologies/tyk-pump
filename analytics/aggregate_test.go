package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCode_ProcessStatusCodes(t *testing.T) {
	errorMap := map[string]int{
		"400": 4,
		"481": 3, // not existing error code
		"482": 2, // not existing error code
		"666": 3, // invalid code
	}

	c := Code{}
	c.ProcessStatusCodes(errorMap)

	assert.Equal(t, 4, c.Code400)
	assert.Equal(t, 5, c.Code4x)

}

func TestAggregate_Tags(t *testing.T) {
	recordsEmptyTag := []interface{}{
		AnalyticsRecord{
			OrgID: "ORG123",
			APIID: "123",
			Tags:  []string{"tag1", ""},
		},
		AnalyticsRecord{
			OrgID: "ORG123",
			APIID: "123",
			Tags:  []string{"", "   ", "tag2"},
		},
	}
	recordsDot := []interface{}{
		AnalyticsRecord{
			OrgID: "ORG123",
			APIID: "123",
			Tags:  []string{"tag1", ""},
		},
		AnalyticsRecord{
			OrgID: "ORG123",
			APIID: "123",
			Tags:  []string{"", "...", "tag1"},
		},
		AnalyticsRecord{
			OrgID: "ORG123",
			APIID: "123",
			Tags:  []string{"internal.group1.dc1.", "tag1", ""},
		},
	}
	runTestAggregatedTags(t, "empty tags", recordsEmptyTag)
	runTestAggregatedTags(t, "dot", recordsDot)
}

func runTestAggregatedTags(t *testing.T, name string, records []interface{}) {
	aggregations := AggregateData(records, false, []string{}, 60, false)

	t.Run(name, func(t *testing.T) {
		for _, aggregation := range aggregations {
			assert.Equal(t, 2, len(aggregation.Tags))
		}
	})
}

func TestTrimTag(t *testing.T) {
	assert.Equal(t, "", TrimTag("..."))
	assert.Equal(t, "helloworld", TrimTag("hello.world"))
	assert.Equal(t, "helloworld", TrimTag(".hello.world.."))
	assert.Equal(t, "hello world", TrimTag(" hello world "))
}

func TestAggregateData_SkipGraphRecords(t *testing.T) {
	run := func(records []AnalyticsRecord, expectedAggregatedRecordCount int, expectedExistingOrgKeys []string, expectedNonExistingOrgKeys []string) func(t *testing.T) {
		return func(t *testing.T) {
			data := make([]interface{}, len(records))
			for i := range records {
				data[i] = records[i]
			}
			aggregatedData := AggregateData(data, true, nil, 1, true)
			assert.Equal(t, expectedAggregatedRecordCount, len(aggregatedData))
			for _, expectedExistingOrgKey := range expectedExistingOrgKeys {
				_, exists := aggregatedData[expectedExistingOrgKey]
				assert.True(t, exists)
			}
			for _, expectedNonExistingOrgKey := range expectedNonExistingOrgKeys {
				_, exists := aggregatedData[expectedNonExistingOrgKey]
				assert.False(t, exists)
			}
		}
	}

	t.Run("should not skip records if no graph analytics record is present", run(
		[]AnalyticsRecord{
			{
				OrgID: "123",
				Tags:  []string{"tag_1", "tag_2"},
			},
			{
				OrgID: "987",
			},
		},
		2,
		[]string{"123", "987"},
		nil,
	))

	t.Run("should skip graph analytics records", run([]AnalyticsRecord{
		{
			OrgID: "123",
			Tags:  []string{"tag_1", "tag_2"},
		},
		{
			OrgID: "777-graph",
			Tags:  []string{"tag_1", "tag_2", PredefinedTagGraphAnalytics},
		},
		{
			OrgID: "987",
		},
		{
			OrgID: "555-graph",
			Tags:  []string{PredefinedTagGraphAnalytics},
		},
	},
		2,
		[]string{"123", "987"},
		[]string{"777-graph", "555-graph"},
	))
}
