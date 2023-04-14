package analytics

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/storage/persistent/model"
	"github.com/google/go-cmp/cmp"
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

func TestAggregatedRecord_TableName(t *testing.T) {
	tcs := []struct {
		testName          string
		givenRecord       AnalyticsRecordAggregate
		expectedTableName string
	}{
		{
			testName: "should return table name with org id",
			givenRecord: AnalyticsRecordAggregate{
				OrgID: "123",
				Mixed: true,
			},
			expectedTableName: AgggregateMixedCollectionName,
		},
		{
			testName: "should return table name with org id",
			givenRecord: AnalyticsRecordAggregate{
				OrgID: "123",
				Mixed: false,
			},
			expectedTableName: "z_tyk_analyticz_aggregate_123",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			assert.Equal(t, tc.expectedTableName, tc.givenRecord.TableName())
		})
	}
}

func TestAggregatedRecord_GetObjectID(t *testing.T) {
	t.Run("should return the ID field", func(t *testing.T) {
		id := model.NewObjectID()
		record := AnalyticsRecordAggregate{
			id: id,
		}
		assert.Equal(t, id, record.GetObjectID())
	})
}

func TestAggregatedRecord_SetObjectID(t *testing.T) {
	t.Run("should set the ID field", func(t *testing.T) {
		id := model.NewObjectID()
		record := AnalyticsRecordAggregate{}
		record.SetObjectID(id)
		assert.Equal(t, id, record.id)
	})
}

func TestSQLAnalyticsRecordAggregate_TableName(t *testing.T) {
	t.Run("should return the SQL table name", func(t *testing.T) {
		record := SQLAnalyticsRecordAggregate{}
		assert.Equal(t, AggregateSQLTable, record.TableName())
	})
}

func TestAnalyticsRecordAggregate_generateBSONFromProperty(t *testing.T) {
	currentTime := time.Date(2023, 0o4, 0o4, 10, 0, 0, 0, time.UTC)

	tcs := []struct {
		givenCounter *Counter
		expected     model.DBM

		testName   string
		givenName  string
		givenValue string
	}{
		{
			testName: "success counter",
			givenCounter: &Counter{
				Hits:                 2,
				TotalRequestTime:     100,
				Success:              1,
				ErrorTotal:           0,
				RequestTime:          100,
				TotalUpstreamLatency: 20,
				MaxLatency:           100,
				MaxUpstreamLatency:   110,
				MinUpstreamLatency:   10,
				MinLatency:           20,
				TotalLatency:         150,
				Identifier:           "",
				HumanIdentifier:      "",
				ErrorMap:             map[string]int{"200": 1},
				LastTime:             currentTime,
			},
			givenName:  "test",
			givenValue: "total",
			expected: model.DBM{
				"$set": model.DBM{
					"test.total.bytesin":           int64(0),
					"test.total.bytesout":          int64(0),
					"test.total.humanidentifier":   "",
					"test.total.identifier":        "",
					"test.total.lasttime":          currentTime,
					"test.total.openconnections":   int64(0),
					"test.total.closedconnections": int64(0),
				},
				"$inc": model.DBM{
					"test.total.errormap.200":         int(1),
					"test.total.errortotal":           int(0),
					"test.total.hits":                 int(2),
					"test.total.success":              int(1),
					"test.total.totallatency":         int64(150),
					"test.total.totalrequesttime":     float64(100),
					"test.total.totalupstreamlatency": int64(20),
				},
				"$max": model.DBM{
					"test.total.maxlatency":         int64(100),
					"test.total.maxupstreamlatency": int64(110),
				},
				"$min": model.DBM{
					"test.total.minlatency":         int64(20),
					"test.total.minupstreamlatency": int64(10),
				},
			},
		},
		{
			testName: "error counter",
			givenCounter: &Counter{
				Hits:                 2,
				TotalRequestTime:     100,
				Success:              0,
				ErrorTotal:           2,
				RequestTime:          100,
				TotalUpstreamLatency: 20,
				MaxLatency:           100,
				MaxUpstreamLatency:   110,
				MinUpstreamLatency:   10,
				MinLatency:           20,
				TotalLatency:         150,
				Identifier:           "test",
				HumanIdentifier:      "",
				ErrorMap:             map[string]int{"500": 2},
				LastTime:             currentTime,
			},
			givenName:  "test",
			givenValue: "total",
			expected: model.DBM{
				"$set": model.DBM{
					"test.total.bytesin":           int64(0),
					"test.total.bytesout":          int64(0),
					"test.total.humanidentifier":   "",
					"test.total.identifier":        "test",
					"test.total.lasttime":          currentTime,
					"test.total.openconnections":   int64(0),
					"test.total.closedconnections": int64(0),
				},
				"$inc": model.DBM{
					"test.total.errormap.500":         int(2),
					"test.total.errortotal":           int(2),
					"test.total.hits":                 int(2),
					"test.total.success":              int(0),
					"test.total.totallatency":         int64(150),
					"test.total.totalrequesttime":     float64(100),
					"test.total.totalupstreamlatency": int64(20),
				},
				"$max": model.DBM{
					"test.total.maxlatency":         int64(100),
					"test.total.maxupstreamlatency": int64(110),
				},
				"$min": model.DBM{}, // we don't update mins on case of full error counter
			},
		},

		{
			testName: "without name",
			givenCounter: &Counter{
				Hits:                 2,
				TotalRequestTime:     100,
				Success:              0,
				ErrorTotal:           2,
				RequestTime:          100,
				TotalUpstreamLatency: 20,
				MaxLatency:           100,
				MaxUpstreamLatency:   110,
				MinUpstreamLatency:   10,
				MinLatency:           20,
				TotalLatency:         150,
				Identifier:           "test",
				HumanIdentifier:      "",
				ErrorMap:             map[string]int{"500": 2},
				LastTime:             currentTime,
			},
			givenName:  "",
			givenValue: "noname",
			expected: model.DBM{
				"$set": model.DBM{
					"noname.bytesin":           int64(0),
					"noname.bytesout":          int64(0),
					"noname.humanidentifier":   "",
					"noname.identifier":        "test",
					"noname.lasttime":          currentTime,
					"noname.openconnections":   int64(0),
					"noname.closedconnections": int64(0),
				},
				"$inc": model.DBM{
					"noname.errormap.500":         int(2),
					"noname.errortotal":           int(2),
					"noname.hits":                 int(2),
					"noname.success":              int(0),
					"noname.totallatency":         int64(150),
					"noname.totalrequesttime":     float64(100),
					"noname.totalupstreamlatency": int64(20),
				},
				"$max": model.DBM{
					"noname.maxlatency":         int64(100),
					"noname.maxupstreamlatency": int64(110),
				},
				"$min": model.DBM{}, // we don't update mins on case of full error counter
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			aggregate := &AnalyticsRecordAggregate{}

			baseDBM := model.DBM{
				"$set": model.DBM{},
				"$inc": model.DBM{},
				"$max": model.DBM{},
				"$min": model.DBM{},
			}

			actual := aggregate.generateBSONFromProperty(tc.givenName, tc.givenValue, tc.givenCounter, baseDBM)
			if !cmp.Equal(tc.expected, actual) {
				t.Errorf("AggregateUptimeData() mismatch (-want +got):\n%s", cmp.Diff(tc.expected, actual))
			}
		})
	}
}

func TestAnalyticsRecordAggregate_generateSetterForTime(t *testing.T) {
	tcs := []struct {
		expected model.DBM

		testName         string
		givenName        string
		givenValue       string
		givenRequestTime float64
	}{
		{
			testName:         "with name",
			givenName:        "test",
			givenValue:       "total",
			givenRequestTime: 100,
			expected: model.DBM{
				"$set": model.DBM{
					"test.total.requesttime": float64(100),
				},
			},
		},
		{
			testName:         "without name",
			givenName:        "",
			givenValue:       "noname",
			givenRequestTime: 130,
			expected: model.DBM{
				"$set": model.DBM{
					"noname.requesttime": float64(130),
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			aggregate := &AnalyticsRecordAggregate{}

			baseDBM := model.DBM{
				"$set": model.DBM{},
			}

			actual := aggregate.generateSetterForTime(tc.givenName, tc.givenValue, tc.givenRequestTime, baseDBM)
			if !cmp.Equal(tc.expected, actual) {
				t.Errorf("AggregateUptimeData() mismatch (-want +got):\n%s", cmp.Diff(tc.expected, actual))
			}
		})
	}
}

func TestAnalyticsRecordAggregate_latencySetter(t *testing.T) {
	tcs := []struct {
		givenCounter *Counter
		expected     model.DBM

		testName   string
		givenName  string
		givenValue string
	}{
		{
			testName: "with name and hits",
			givenCounter: &Counter{
				Hits:                 2,
				TotalLatency:         100,
				TotalUpstreamLatency: 200,
			},
			givenName:  "test",
			givenValue: "total",
			expected: model.DBM{
				"$set": model.DBM{
					"test.total.latency":         float64(50),
					"test.total.upstreamlatency": float64(100),
				},
			},
		},
		{
			testName: "without name and with hits",
			givenCounter: &Counter{
				Hits:                 2,
				TotalLatency:         200,
				TotalUpstreamLatency: 400,
			},
			givenName:  "",
			givenValue: "noname",
			expected: model.DBM{
				"$set": model.DBM{
					"noname.latency":         float64(100),
					"noname.upstreamlatency": float64(200),
				},
			},
		},

		{
			testName: "without name and without hits",
			givenCounter: &Counter{
				Hits:                 0,
				TotalLatency:         200,
				TotalUpstreamLatency: 400,
			},
			givenName:  "",
			givenValue: "noname",
			expected: model.DBM{
				"$set": model.DBM{
					"noname.latency":         float64(0),
					"noname.upstreamlatency": float64(0),
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			aggregate := &AnalyticsRecordAggregate{}

			baseDBM := model.DBM{
				"$set": model.DBM{},
			}

			actual := aggregate.latencySetter(tc.givenName, tc.givenValue, baseDBM, tc.givenCounter)
			if !cmp.Equal(tc.expected, actual) {
				t.Errorf("AggregateUptimeData() mismatch (-want +got):\n%s", cmp.Diff(tc.expected, actual))
			}
		})
	}
}

func TestAnalyticsRecordAggregate_AsChange(t *testing.T) {
	currentTime := time.Date(2023, 0o4, 0o4, 10, 0, 0, 0, time.UTC)

	tcs := []struct {
		given    *AnalyticsRecordAggregate
		expected model.DBM
		testName string
	}{
		{
			testName: "aggregate with versions - no errors",
			given: &AnalyticsRecordAggregate{
				OrgID: "testorg",
				TimeID: struct {
					Year  int
					Month int
					Day   int
					Hour  int
				}{
					Year:  currentTime.Year(),
					Month: int(currentTime.Month()),
					Day:   currentTime.Day(),
					Hour:  currentTime.Hour(),
				},
				Versions: map[string]*Counter{
					"v1": {
						Hits:                 1,
						Success:              1,
						TotalLatency:         100,
						TotalUpstreamLatency: 200,
						TotalRequestTime:     200,
						MinUpstreamLatency:   20,
						MinLatency:           10,
						MaxUpstreamLatency:   100,
						MaxLatency:           100,
						LastTime:             currentTime,
					},
					"v2": {
						Hits:                 1,
						Success:              1,
						TotalLatency:         100,
						TotalUpstreamLatency: 200,
						TotalRequestTime:     200,
						MinUpstreamLatency:   20,
						MinLatency:           10,
						MaxUpstreamLatency:   100,
						MaxLatency:           100,
						LastTime:             currentTime,
					},
				},
				Total: Counter{
					Hits:                 2,
					Success:              2,
					TotalLatency:         200,
					TotalRequestTime:     200,
					MaxUpstreamLatency:   100,
					MaxLatency:           100,
					MinUpstreamLatency:   20,
					MinLatency:           10,
					TotalUpstreamLatency: 400,
					LastTime:             currentTime,
				},
				Errors:    map[string]*Counter{},
				LastTime:  currentTime,
				TimeStamp: currentTime,
				ExpireAt:  currentTime,
			},
			expected: model.DBM{
				"$inc": model.DBM{
					"total.hits":                       int(2),
					"total.success":                    int(2),
					"total.errortotal":                 int(0),
					"total.totallatency":               int64(200),
					"total.totalupstreamlatency":       int64(400),
					"total.totalrequesttime":           float64(200),
					"versions.v1.errortotal":           int(0),
					"versions.v1.hits":                 int(1),
					"versions.v1.success":              int(1),
					"versions.v1.totallatency":         int64(100),
					"versions.v1.totalrequesttime":     float64(200),
					"versions.v1.totalupstreamlatency": int64(200),
					"versions.v2.errortotal":           int(0),
					"versions.v2.hits":                 int(1),
					"versions.v2.success":              int(1),
					"versions.v2.totallatency":         int64(100),
					"versions.v2.totalrequesttime":     float64(200),
					"versions.v2.totalupstreamlatency": int64(200),
				},
				"$min": model.DBM{
					"total.minlatency":               int64(10),
					"total.minupstreamlatency":       int64(20),
					"versions.v1.minlatency":         int64(10),
					"versions.v1.minupstreamlatency": int64(20),
					"versions.v2.minlatency":         int64(10),
					"versions.v2.minupstreamlatency": int64(20),
				},
				"$max": model.DBM{
					"total.maxlatency":               int64(100),
					"total.maxupstreamlatency":       int64(100),
					"versions.v1.maxlatency":         int64(100),
					"versions.v1.maxupstreamlatency": int64(100),
					"versions.v2.maxlatency":         int64(100),
					"versions.v2.maxupstreamlatency": int64(100),
				},
				"$set": model.DBM{
					"expireAt":                      currentTime,
					"lasttime":                      currentTime,
					"timestamp":                     currentTime,
					"total.lasttime":                currentTime,
					"timeid.day":                    currentTime.Day(),
					"timeid.hour":                   currentTime.Hour(),
					"timeid.month":                  currentTime.Month(),
					"timeid.year":                   currentTime.Year(),
					"total.bytesin":                 int64(0),
					"total.bytesout":                int64(0),
					"total.closedconnections":       int64(0),
					"total.openconnections":         int64(0),
					"total.humanidentifier":         "",
					"total.identifier":              "",
					"versions.v1.bytesin":           int64(0),
					"versions.v1.bytesout":          int64(0),
					"versions.v1.lasttime":          currentTime,
					"versions.v1.humanidentifier":   "",
					"versions.v1.identifier":        "",
					"versions.v1.closedconnections": int64(0),
					"versions.v1.openconnections":   int64(0),
					"versions.v2.bytesin":           int64(0),
					"versions.v2.bytesout":          int64(0),
					"versions.v2.lasttime":          currentTime,
					"versions.v2.humanidentifier":   "",
					"versions.v2.identifier":        "",
					"versions.v2.closedconnections": int64(0),
					"versions.v2.openconnections":   int64(0),
				},
			},
		},
		{
			testName: "aggregate with apiid - with errors",
			given: &AnalyticsRecordAggregate{
				OrgID: "testorg",
				TimeID: struct {
					Year  int
					Month int
					Day   int
					Hour  int
				}{
					Year:  currentTime.Year(),
					Month: int(currentTime.Month()),
					Day:   currentTime.Day(),
					Hour:  currentTime.Hour(),
				},
				APIID: map[string]*Counter{
					"api1": {
						Hits:                 3,
						Success:              0,
						ErrorTotal:           3,
						TotalLatency:         100,
						TotalUpstreamLatency: 200,
						TotalRequestTime:     200,
						MinUpstreamLatency:   20,
						MinLatency:           10,
						MaxUpstreamLatency:   100,
						MaxLatency:           100,
						ErrorMap:             map[string]int{"404": 1, "500": 2},
						ErrorList:            []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
						LastTime:             currentTime,
					},
					"api2": {
						Hits:                 1,
						Success:              1,
						TotalLatency:         100,
						TotalUpstreamLatency: 200,
						TotalRequestTime:     200,
						MinUpstreamLatency:   20,
						MinLatency:           10,
						MaxUpstreamLatency:   100,
						MaxLatency:           100,
						LastTime:             currentTime,
					},
				},
				Total: Counter{
					Hits:                 4,
					Success:              1,
					ErrorTotal:           3,
					TotalLatency:         200,
					TotalRequestTime:     200,
					MaxUpstreamLatency:   100,
					MaxLatency:           100,
					MinUpstreamLatency:   20,
					MinLatency:           10,
					TotalUpstreamLatency: 400,
					ErrorMap:             map[string]int{"404": 1, "500": 2},
					ErrorList:            []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
					LastTime:             currentTime,
				},
				Errors:    map[string]*Counter{},
				LastTime:  currentTime,
				TimeStamp: currentTime,
				ExpireAt:  currentTime,
			},
			expected: model.DBM{
				"$inc": model.DBM{
					"total.hits":                      int(4),
					"total.success":                   int(1),
					"total.errortotal":                int(3),
					"total.totallatency":              int64(200),
					"total.totalupstreamlatency":      int64(400),
					"total.totalrequesttime":          float64(200),
					"total.errormap.404":              int(1),
					"total.errormap.500":              int(2),
					"apiid.api1.hits":                 int(3),
					"apiid.api1.success":              int(0),
					"apiid.api1.errortotal":           int(3),
					"apiid.api1.totallatency":         int64(100),
					"apiid.api1.totalupstreamlatency": int64(200),
					"apiid.api1.totalrequesttime":     float64(200),
					"apiid.api1.errormap.404":         int(1),
					"apiid.api1.errormap.500":         int(2),
					"apiid.api2.hits":                 int(1),
					"apiid.api2.success":              int(1),
					"apiid.api2.totallatency":         int64(100),
					"apiid.api2.totalupstreamlatency": int64(200),
					"apiid.api2.totalrequesttime":     float64(200),
					"apiid.api2.errortotal":           int(0),
				},
				"$min": model.DBM{
					"total.minlatency":              int64(10),
					"total.minupstreamlatency":      int64(20),
					"apiid.api2.minlatency":         int64(10),
					"apiid.api2.minupstreamlatency": int64(20),
				},
				"$max": model.DBM{
					"total.maxlatency":              int64(100),
					"total.maxupstreamlatency":      int64(100),
					"apiid.api1.maxlatency":         int64(100),
					"apiid.api1.maxupstreamlatency": int64(100),
					"apiid.api2.maxlatency":         int64(100),
					"apiid.api2.maxupstreamlatency": int64(100),
				},
				"$set": model.DBM{
					"expireAt":                     currentTime,
					"lasttime":                     currentTime,
					"timestamp":                    currentTime,
					"total.lasttime":               currentTime,
					"timeid.day":                   currentTime.Day(),
					"timeid.hour":                  currentTime.Hour(),
					"timeid.month":                 currentTime.Month(),
					"timeid.year":                  currentTime.Year(),
					"total.bytesin":                int64(0),
					"total.bytesout":               int64(0),
					"total.closedconnections":      int64(0),
					"total.openconnections":        int64(0),
					"total.humanidentifier":        "",
					"total.identifier":             "",
					"apiid.api1.bytesin":           int64(0),
					"apiid.api1.bytesout":          int64(0),
					"apiid.api1.closedconnections": int64(0),
					"apiid.api1.openconnections":   int64(0),
					"apiid.api1.humanidentifier":   "",
					"apiid.api1.identifier":        "",
					"apiid.api1.lasttime":          currentTime,
					"apiid.api2.bytesin":           int64(0),
					"apiid.api2.bytesout":          int64(0),
					"apiid.api2.closedconnections": int64(0),
					"apiid.api2.openconnections":   int64(0),
					"apiid.api2.humanidentifier":   "",
					"apiid.api2.identifier":        "",
					"apiid.api2.lasttime":          currentTime,
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			actual := tc.given.AsChange()
			if !cmp.Equal(tc.expected, actual) {
				t.Errorf("AggregateUptimeData() mismatch (-want +got):\n%s", cmp.Diff(tc.expected, actual))
			}
		})
	}
}

func TestAnalyticsRecordAggregate_AsTimeUpdate(t *testing.T) {
	currentTime := time.Date(2023, 0o4, 0o4, 10, 0, 0, 0, time.UTC)

	tcs := []struct {
		given    *AnalyticsRecordAggregate
		expected model.DBM
		testName string
	}{
		{
			testName: "oauthendpoint+keyendpoint+apiendpoint+tota",
			given: &AnalyticsRecordAggregate{
				OrgID: "testorg",
				KeyEndpoint: map[string]map[string]*Counter{
					"apikey1": {
						"/get": {
							Hits:                 3,
							Success:              0,
							ErrorTotal:           3,
							TotalLatency:         300,
							TotalUpstreamLatency: 600,
							LastTime:             currentTime,
							ErrorMap:             map[string]int{"404": 1, "500": 2},
						},
					},
				},
				OauthEndpoint: map[string]map[string]*Counter{
					"oauthid1": {
						"/get": {
							Hits:                 3,
							Success:              0,
							ErrorTotal:           3,
							TotalLatency:         300,
							TotalUpstreamLatency: 600,
							LastTime:             currentTime,
							ErrorMap:             map[string]int{"404": 1, "500": 2},
						},
					},
				},
				ApiEndpoint: map[string]*Counter{
					"/get": {
						Hits:                 3,
						Success:              0,
						ErrorTotal:           3,
						TotalLatency:         300,
						TotalUpstreamLatency: 600,
						LastTime:             currentTime,
						ErrorMap:             map[string]int{"404": 1, "500": 2},
					},
				},

				Total: Counter{
					Hits:                 3,
					Success:              0,
					ErrorTotal:           3,
					TotalLatency:         300,
					TotalUpstreamLatency: 600,
					TotalRequestTime:     300,
					ErrorMap:             map[string]int{"404": 1, "500": 2},
					BytesIn:              0,
					BytesOut:             0,
					OpenConnections:      0,
					ClosedConnections:    0,
					HumanIdentifier:      "",
					Identifier:           "",
					LastTime:             currentTime,
					MinLatency:           10,
					MaxLatency:           100,
					MinUpstreamLatency:   20,
					MaxUpstreamLatency:   100,
				},
			},
			expected: model.DBM{
				"$set": model.DBM{
					"apiendpoints./get.errorlist":               []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
					"apiendpoints./get.latency":                 float64(100),
					"apiendpoints./get.requesttime":             float64(0),
					"apiendpoints./get.upstreamlatency":         float64(200),
					"keyendpoints.apikey1./get.errorlist":       []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
					"keyendpoints.apikey1./get.latency":         float64(100),
					"keyendpoints.apikey1./get.requesttime":     float64(0),
					"keyendpoints.apikey1./get.upstreamlatency": float64(200),
					"lists.apiendpoints": []Counter{
						{
							Hits:                 3,
							Success:              0,
							ErrorTotal:           3,
							TotalLatency:         300,
							TotalUpstreamLatency: 600,
							UpstreamLatency:      200,
							Latency:              100,
							LastTime:             currentTime,
							ErrorMap:             map[string]int{"404": 1, "500": 2},
							ErrorList:            []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
						},
					},
					"lists.apiid":     []Counter{},
					"lists.apikeys":   []Counter{},
					"lists.endpoints": []Counter{},
					"lists.errors":    []Counter{},
					"lists.geo":       []Counter{},
					"lists.oauthids":  []Counter{},
					"lists.tags":      []Counter{},
					"lists.versions":  []Counter{},
					"lists.keyendpoints.apikey1": []Counter{
						{
							Hits:                 3,
							Success:              0,
							ErrorTotal:           3,
							TotalLatency:         300,
							TotalUpstreamLatency: 600,
							UpstreamLatency:      200,
							Latency:              100,
							LastTime:             currentTime,
							ErrorMap:             map[string]int{"404": 1, "500": 2},
							ErrorList:            []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
						},
					},
					"lists.oauthendpoints.oauthid1": []Counter{
						{
							Hits:                 3,
							Success:              0,
							ErrorTotal:           3,
							TotalLatency:         300,
							TotalUpstreamLatency: 600,
							UpstreamLatency:      200,
							Latency:              100,
							LastTime:             currentTime,
							ErrorMap:             map[string]int{"404": 1, "500": 2},
							ErrorList:            []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
						},
					},
					"oauthendpoints.oauthid1./get.errorlist":       []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
					"oauthendpoints.oauthid1./get.latency":         float64(100),
					"oauthendpoints.oauthid1./get.requesttime":     float64(0),
					"oauthendpoints.oauthid1./get.upstreamlatency": float64(200),
					"total.errorlist":                              []ErrorData{{Code: "404", Count: 1}, {Code: "500", Count: 2}},
					"total.latency":                                float64(100),
					"total.requesttime":                            float64(100),
					"total.upstreamlatency":                        float64(200),
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			actual := tc.given.AsTimeUpdate()
			if !cmp.Equal(tc.expected, actual) {
				t.Errorf("AggregateUptimeData() mismatch (-want +got):\n%s", cmp.Diff(tc.expected, actual))
			}
		})
	}
}
