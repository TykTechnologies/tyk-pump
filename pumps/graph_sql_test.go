package pumps

import (
	"context"
	"encoding/base64"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"testing"
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
			pump.db.Migrator().DropTable("rand-table")
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
	conf := SQLConf{
		Type:             "sqlite",
		ConnectionString: "",
		TableName:        "test-table",
	}
	pump := &GraphSQLPump{}
	assert.NoError(t, pump.Init(conf))

	type customRecord struct {
		isHttp       bool
		tags         []string
		response     string
		responseCode int
	}
	type expectedResponse struct {
		types         map[string][]string
		operationType string
	}

	testCases := []struct {
		name      string
		records   []customRecord
		responses []expectedResponse
		hasError  bool
	}{
		{
			name: "default case",
			records: []customRecord{
				{
					isHttp:       false,
					tags:         []string{analytics.PredefinedTagGraphAnalytics},
					responseCode: 200,
					response:     rawGQLResponse,
				},
			},
			responses: []expectedResponse{
				{
					types: map[string][]string{
						"Country": {"code"},
					},
					operationType: "Query",
				},
			},
			hasError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			records := make([]interface{}, 0)
			expectedResponses := make([]analytics.GraphRecord, 0)
			for _, item := range tc.records {
				r := analytics.AnalyticsRecord{
					APIName: "Test API",
					Path:    "POST",
					Tags:    item.tags,
				}
				if !item.isHttp {
					r.RawRequest = convToBase64(rawGQLRequest)
					r.ApiSchema = convToBase64(schema)
				} else {
					r.RawRequest = convToBase64(rawHTTPReq)
					r.RawResponse = convToBase64(rawHTTPResponse)
				}
				r.RawResponse = item.response
				if item.responseCode != 0 {
					r.ResponseCode = item.responseCode
				}
				records = append(records, r)
			}

			for _, item := range tc.responses {
				r := analytics.GraphRecord{
					Types:         item.types,
					OperationType: item.operationType,
					Errors:        []analytics.GraphError{},
				}
				expectedResponses = append(expectedResponses, r)
			}

			err := pump.WriteData(context.Background(), records)
			if !tc.hasError {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			var resultRecords []analytics.GraphRecord
			pump.db.Table(conf.TableName).Find(&resultRecords)
			assert.Equal(t, len(tc.responses), len(resultRecords), "responses count do no match")
			if diff := cmp.Diff(expectedResponses, resultRecords, cmpopts.IgnoreFields(analytics.GraphRecord{}, "AnalyticsRecord")); diff != "" {
				t.Error(diff)
			}
		})
	}
}
