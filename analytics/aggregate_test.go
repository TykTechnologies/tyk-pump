package analytics

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

const graphErrorResponse = `{
  "errors": [
    {
      "message": "Name for character with ID 1002 could not be fetched.",
      "locations": [{ "line": 6, "column": 7 }],
      "path": ["hero", "heroFriends", 1, "name"]
    }
  ]
}`

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
	aggregations := AggregateData(records, false, []string{}, "", 60)

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

func TestAggregateGraphData(t *testing.T) {
	sampleRecord := AnalyticsRecord{
		TimeStamp:    time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		Method:       "POST",
		Host:         "localhost:8281",
		Path:         "/",
		RawPath:      "/",
		APIName:      "test-api",
		APIID:        "test-api",
		ApiSchema:    base64.StdEncoding.EncodeToString([]byte(sampleSchema)),
		Tags:         []string{PredefinedTagGraphAnalytics},
		ResponseCode: 200,
		Day:          1,
		Month:        1,
		Year:         2022,
		Hour:         0,
		OrgID:        "test-org",
		APIKey:       "test-key",
		TrackPath:    true,
		OauthID:      "test-id",
	}

	compareFields := func(r *require.Assertions, expected, actual map[string]*Counter) {
		r.Equal(len(expected), len(actual), "field map not equal")
		for k, expectedVal := range expected {
			actualVal, ok := actual[k]
			r.True(ok)
			r.Equal(expectedVal.Hits, actualVal.Hits, "hits not matching for %s", k)
			r.Equal(expectedVal.ErrorTotal, actualVal.ErrorTotal, "error total not matching for %s", k)
			r.Equal(expectedVal.Success, actualVal.Success, "success not matching for %s", k)
		}
	}

	testCases := []struct {
		expectedAggregate map[string]GraphRecordAggregate
		recordGenerator   func() []interface{}
		name              string
	}{
		{
			name: "default",
			recordGenerator: func() []interface{} {
				records := make([]interface{}, 3)
				for i := range records {
					record := sampleRecord
					query := `{"query":"query{\n  characters(filter: {\n    \n  }){\n    info{\n      count\n    }\n  }\n}"}`
					response := `{"data":{"characters":{"info":{"count":758}}}}`
					record.RawRequest = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(requestTemplate, len(query), query)))
					record.RawResponse = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(responseTemplate, len(response), response)))
					records[i] = record
				}
				return records
			},
			expectedAggregate: map[string]GraphRecordAggregate{
				"test-org": {
					Types: map[string]*Counter{
						"Characters": {Hits: 3, ErrorTotal: 0, Success: 3},
						"Info":       {Hits: 3, ErrorTotal: 0, Success: 3},
					},
					Fields: map[string]*Counter{
						"Characters_info": {Hits: 3, ErrorTotal: 0, Success: 3},
						"Info_count":      {Hits: 3, ErrorTotal: 0, Success: 3},
					},
					RootFields: map[string]*Counter{
						"characters": {Hits: 3, ErrorTotal: 0, Success: 3},
					},
				},
			},
		},
		{
			name: "skip non graph records",
			recordGenerator: func() []interface{} {
				records := make([]interface{}, 3)
				for i := range records {
					record := sampleRecord
					query := `{"query":"query{\n  characters(filter: {\n    \n  }){\n    info{\n      count\n    }\n  }\n}"}`
					response := `{"data":{"characters":{"info":{"count":758}}}}`
					record.RawRequest = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(requestTemplate, len(query), query)))
					record.RawResponse = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(responseTemplate, len(response), response)))
					if i == 1 {
						record.Tags = []string{}
					}
					records[i] = record
				}
				return records
			},
			expectedAggregate: map[string]GraphRecordAggregate{
				"test-org": {
					Types: map[string]*Counter{
						"Characters": {Hits: 2, ErrorTotal: 0, Success: 2},
						"Info":       {Hits: 2, ErrorTotal: 0, Success: 2},
					},
					Fields: map[string]*Counter{
						"Characters_info": {Hits: 2, ErrorTotal: 0, Success: 2},
						"Info_count":      {Hits: 2, ErrorTotal: 0, Success: 2},
					},
					RootFields: map[string]*Counter{
						"characters": {Hits: 2, ErrorTotal: 0, Success: 2},
					},
				},
			},
		},
		{
			name: "has errors",
			recordGenerator: func() []interface{} {
				records := make([]interface{}, 3)
				for i := range records {
					record := sampleRecord
					query := `{"query":"query{\n  characters(filter: {\n    \n  }){\n    info{\n      count\n    }\n  }\n}"}`
					response := `{"data":{"characters":{"info":{"count":758}}}}`
					if i == 1 {
						response = graphErrorResponse
					}
					record.RawRequest = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(requestTemplate, len(query), query)))
					record.RawResponse = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(responseTemplate, len(response), response)))
					records[i] = record
				}
				return records
			},
			expectedAggregate: map[string]GraphRecordAggregate{
				"test-org": {
					Types: map[string]*Counter{
						"Characters": {Hits: 3, ErrorTotal: 1, Success: 2},
						"Info":       {Hits: 3, ErrorTotal: 1, Success: 2},
					},
					Fields: map[string]*Counter{
						"Characters_info": {Hits: 3, ErrorTotal: 1, Success: 2},
						"Info_count":      {Hits: 3, ErrorTotal: 1, Success: 2},
					},
					RootFields: map[string]*Counter{
						"characters": {Hits: 3, ErrorTotal: 1, Success: 2},
					},
				},
			},
		},
		{
			name: "error response code",
			recordGenerator: func() []interface{} {
				records := make([]interface{}, 5)
				for i := range records {
					record := sampleRecord
					query := `{"query":"query{\n  characters(filter: {\n    \n  }){\n    info{\n      count\n    }\n  }\n}"}`
					response := `{"data":{"characters":{"info":{"count":758}}}}`
					record.RawRequest = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(requestTemplate, len(query), query)))
					record.RawResponse = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(responseTemplate, len(response), response)))
					if i == 2 || i == 4 {
						record.ResponseCode = 500
					}
					records[i] = record
				}
				return records
			},
			expectedAggregate: map[string]GraphRecordAggregate{
				"test-org": {
					Types: map[string]*Counter{
						"Characters": {Hits: 5, ErrorTotal: 2, Success: 3},
						"Info":       {Hits: 5, ErrorTotal: 2, Success: 3},
					},
					Fields: map[string]*Counter{
						"Characters_info": {Hits: 5, ErrorTotal: 2, Success: 3},
						"Info_count":      {Hits: 5, ErrorTotal: 2, Success: 3},
					},
					RootFields: map[string]*Counter{
						"characters": {Hits: 5, ErrorTotal: 2, Success: 3},
					},
				},
			},
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			records := c.recordGenerator()
			aggregated := AggregateGraphData(records, "", 0)
			r.Len(aggregated, len(c.expectedAggregate))
			for key, expectedAggregate := range c.expectedAggregate {
				actualAggregate, ok := aggregated[key]
				r.True(ok)
				// check types and fields
				compareFields(r, expectedAggregate.Types, actualAggregate.Types)
				compareFields(r, expectedAggregate.Fields, actualAggregate.Fields)
				compareFields(r, expectedAggregate.RootFields, actualAggregate.RootFields)
			}
		})
	}
}

func TestAggregateGraphData_Dimension(t *testing.T) {
	sampleRecord := AnalyticsRecord{
		TimeStamp:    time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		Method:       "POST",
		Host:         "localhost:8281",
		Path:         "/",
		RawPath:      "/",
		APIName:      "test-api",
		APIID:        "test-api",
		ApiSchema:    base64.StdEncoding.EncodeToString([]byte(sampleSchema)),
		Tags:         []string{PredefinedTagGraphAnalytics},
		ResponseCode: 200,
		Day:          1,
		Month:        1,
		Year:         2022,
		Hour:         0,
		OrgID:        "test-org",
	}

	records := make([]interface{}, 3)
	for i := range records {
		record := sampleRecord
		query := `{"query":"query{\n  characters(filter: {\n    \n  }){\n    info{\n      count\n    }\n  }\n}"}`
		response := `{"data":{"characters":{"info":{"count":758}}}}`
		record.RawRequest = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(requestTemplate, len(query), query)))
		record.RawResponse = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(responseTemplate, len(response), response)))
		records[i] = record
	}

	responsesCheck := map[string][]string{
		"types": {
			"Characters",
			"Info",
		},
		"fields": {
			"Characters_info",
			"Info_count",
		},
		"operation": {
			"Query",
		},
		"rootfields": {
			"characters",
		},
	}

	r := require.New(t)
	aggregated := AggregateGraphData(records, "", 1)
	r.Len(aggregated, 1)
	aggre := aggregated["test-org"]
	dimensions := aggre.Dimensions()
	fmt.Println(dimensions)
	for d, values := range responsesCheck {
		for _, v := range values {
			found := false
			for _, dimension := range dimensions {
				if dimension.Name == d && dimension.Value == v && dimension.Counter.Hits == 3 {
					found = true
				}
			}
			if !found {
				t.Errorf("item missing from dimensions: NameL %s, Value: %s, Hits:3", d, v)
			}
		}
	}
}

func TestAggregateData_SkipGraphRecords(t *testing.T) {
	run := func(records []AnalyticsRecord, expectedAggregatedRecordCount int, expectedExistingOrgKeys, expectedNonExistingOrgKeys []string) func(t *testing.T) {
		return func(t *testing.T) {
			data := make([]interface{}, len(records))
			for i := range records {
				data[i] = records[i]
			}
			aggregatedData := AggregateData(data, true, nil, "", 1)
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

func TestSetAggregateTimestamp(t *testing.T) {
	asTime := time.Now()

	tests := []struct {
		ExpectedTime    time.Time
		testName        string
		DBIdentifier    string
		AggregationTime int
	}{
		{
			testName:        "AggregationTime is 60",
			AggregationTime: 60,
			DBIdentifier:    "testing-mongo",
			ExpectedTime:    time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), 0, 0, 0, asTime.Location()),
		},
		{
			testName:        "AggregationTime is 1",
			AggregationTime: 1,
			DBIdentifier:    "testing-mongo",
			ExpectedTime:    time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location()),
		},
		{
			testName:        "AggregationTime is 40",
			AggregationTime: 40,
			DBIdentifier:    "testing-mongo",
			ExpectedTime:    time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location()),
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			ts := setAggregateTimestamp(test.DBIdentifier, asTime, test.AggregationTime)
			assert.Equal(t, test.ExpectedTime, ts)
		})
	}

	SetlastTimestampAgggregateRecord("testing-setLastTimestamp", time.Now().Add(-time.Minute*10))
	ts := setAggregateTimestamp("testing-setLastTimestamp", asTime, 7)
	assert.Equal(t, time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location()), ts)
}
