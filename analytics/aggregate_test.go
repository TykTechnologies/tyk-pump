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
			Tags:  []string{"", "...", "tag2"},
		},
		AnalyticsRecord{
			OrgID: "ORG123",
			APIID: "123",
			Tags:  []string{"", " ...", "tag2"},
		},
	}
	runTestAggregatedTags(t, "empty tags", recordsEmptyTag)
	runTestAggregatedTags(t, "dot", recordsDot)
}

func runTestAggregatedTags(t *testing.T, name string, records []interface{}) {
	aggregations := AggregateData(records, false, []string{}, false)

	t.Run(name, func(t *testing.T) {
		for _, aggregation := range aggregations {

			assert.Equal(t, 2, len(aggregation.Tags))
		}
	})
}
