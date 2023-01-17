package pumps

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphSQLPump_Init(t *testing.T) {
	pump := &GraphSQLPump{}
	t.Run("successful", func(t *testing.T) {
		conf := SQLConf{
			Type:             "sqlite",
			ConnectionString: "",
			TableName:        "rand-table",
		}
		assert.NoError(t, pump.Init(conf))
		t.Cleanup(func() {
			if err := pump.db.Migrator().DropTable("rand-table"); err != nil {
				t.Errorf("error cleaning up table: %v", err)
			}
		})
		assert.True(t, pump.db.Migrator().HasTable(conf.TableName))
	})

	t.Run("invalid connection details", func(t *testing.T) {
		conf := SQLConf{
			Type:             "postgres",
			ConnectionString: "host=localhost user=gorm password=gorm DB.name=gorm port=9920 sslmode=disable",
		}
		assert.Error(t, pump.Init(conf))
	})

	t.Run("should fail", func(t *testing.T) {
		conf := SQLConf{ConnectionString: "random"}
		assert.ErrorContains(t, pump.Init(conf), "Unsupported `config_storage.type` value:")
	})

	t.Run("invalid config", func(t *testing.T) {
		conf := map[string]interface{}{
			"type": 1,
		}
		assert.ErrorContains(t, pump.Init(conf), "error decoding con")
	})

	t.Run("sharded table", func(t *testing.T) {
		conf := SQLConf{
			Type:             "sqlite",
			ConnectionString: "",
			TableName:        "test-table",
			TableSharding:    true,
		}
		assert.NoError(t, pump.Init(conf))
		assert.False(t, pump.db.Migrator().HasTable(conf.TableName))
	})
}

func convToBase64(raw string) string {
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func TestGraphSQLPump_WriteData(t *testing.T) {
	r := require.New(t)
	conf := SQLConf{
		Type:             "sqlite",
		ConnectionString: "",
		TableName:        "test-table",
	}

	type customRecord struct {
		response     string
		tags         []string
		responseCode int
		isHTTP       bool
	}
	type customResponses struct {
		types         map[string][]string
		operationType string
		expectedErr   []analytics.GraphError
	}

	testCases := []struct {
		name      string
		records   []customRecord
		responses []customResponses
		hasError  bool
	}{
		{
			name: "default case",
			records: []customRecord{
				{
					isHTTP:       false,
					tags:         []string{analytics.PredefinedTagGraphAnalytics},
					responseCode: 200,
					response:     rawGQLResponse,
				},
				{
					isHTTP:       false,
					tags:         []string{analytics.PredefinedTagGraphAnalytics},
					responseCode: 200,
					response:     rawGQLResponseWithError,
				},
				{
					isHTTP:       false,
					tags:         []string{analytics.PredefinedTagGraphAnalytics},
					responseCode: 500,
					response:     "",
				},
			},
			responses: []customResponses{
				{
					types: map[string][]string{
						"Country": {"code"},
					},
					operationType: "Query",
				},
				{
					types: map[string][]string{
						"Country": {"code"},
					},
					operationType: "Query",
					expectedErr: []analytics.GraphError{
						{
							Message: "test error",
							Path:    []interface{}{},
						},
					},
				},
				{
					types: map[string][]string{
						"Country": {"code"},
					},
					operationType: "Query",
					expectedErr:   []analytics.GraphError{},
				},
			},
			hasError: false,
		},
		{
			name: "skip record",
			records: []customRecord{
				{
					isHTTP:       false,
					tags:         []string{analytics.PredefinedTagGraphAnalytics},
					responseCode: 200,
					response:     rawGQLResponse,
				},
				{
					isHTTP:       true,
					responseCode: 200,
					response:     rawHTTPResponse,
				},
				{
					isHTTP:       false,
					responseCode: 200,
					response:     rawGQLResponse,
				},
			},
			responses: []customResponses{
				{
					types: map[string][]string{
						"Country": {"code"},
					},
					operationType: "Query",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pump := &GraphSQLPump{}
			assert.NoError(t, pump.Init(conf))

			t.Cleanup(func() {
				if err := pump.db.Migrator().DropTable(conf.TableName); err != nil {
					t.Error(err)
				}
			})
			records := make([]interface{}, 0)
			expectedResponses := make([]analytics.GraphRecord, 0)
			// create the records to passed to the pump
			for _, item := range tc.records {
				r := analytics.AnalyticsRecord{
					APIName: "Test API",
					Path:    "POST",
					Tags:    item.tags,
				}
				if !item.isHTTP {
					r.RawRequest = convToBase64(rawGQLRequest)
					r.ApiSchema = convToBase64(schema)
				} else {
					r.RawRequest = convToBase64(rawHTTPReq)
					r.RawResponse = convToBase64(rawHTTPResponse)
				}
				r.RawResponse = convToBase64(item.response)
				if item.responseCode != 0 {
					r.ResponseCode = item.responseCode
				}
				records = append(records, r)
			}

			// create the responses to be expected from the db
			for _, item := range tc.responses {
				r := analytics.GraphRecord{
					Types:         item.types,
					OperationType: item.operationType,
					Errors:        []analytics.GraphError{},
				}
				if item.expectedErr == nil {
					r.Errors = []analytics.GraphError{}
				} else {
					r.Errors = item.expectedErr
					r.HasErrors = true
				}
				expectedResponses = append(expectedResponses, r)
			}

			err := pump.WriteData(context.Background(), records)
			if !tc.hasError {
				r.NoError(err)
			} else {
				r.Error(err)
			}

			var resultRecords []analytics.GraphRecord
			pump.db.Table(conf.TableName).Find(&resultRecords)
			r.Equalf(len(tc.responses), len(resultRecords), "responses count do no match")
			if diff := cmp.Diff(expectedResponses, resultRecords, cmpopts.IgnoreFields(analytics.GraphRecord{}, "AnalyticsRecord")); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestGraphSQLPump_Sharded(t *testing.T) {
	r := require.New(t)
	conf := SQLConf{
		Type:             "sqlite",
		ConnectionString: "",
		TableName:        "graph-record",
		TableSharding:    true,
	}
	pump := &GraphSQLPump{}
	assert.NoError(t, pump.Init(conf))

	baseRecord := analytics.AnalyticsRecord{
		APIID:        "test-api",
		Path:         "/test-api",
		RawRequest:   convToBase64(rawGQLRequest),
		RawResponse:  convToBase64(rawGQLResponse),
		ApiSchema:    convToBase64(schema),
		Tags:         []string{analytics.PredefinedTagGraphAnalytics},
		APIName:      "test-api",
		ResponseCode: 200,
		Method:       "POST",
	}

	expectedTables := make([]string, 0)
	records := make([]interface{}, 0)
	for i := 1; i <= 3; i++ {
		day := i
		timestamp := time.Date(2023, time.January, day, 0, 1, 0, 0, time.UTC)
		rec := baseRecord
		rec.TimeStamp = timestamp
		rec.Month = timestamp.Month()
		rec.Day = timestamp.Day()
		rec.Year = timestamp.Year()
		records = append(records, rec)
		expectedTables = append(expectedTables, fmt.Sprintf("%s_%s", conf.TableName, timestamp.Format("20060102")))
	}

	r.NoError(pump.WriteData(context.Background(), records))
	// check tables
	for _, item := range expectedTables {
		r.Truef(pump.db.Migrator().HasTable(item), "table %s does not exist", item)
		recs := make([]analytics.GraphRecord, 0)
		pump.db.Table(item).Find(&recs)
		fmt.Printf("got %d records for %s\n", len(recs), item)
		assert.Equalf(t, 1, len(recs), "expected one record for %s table, instead got %d", item, len(recs))
	}
}
