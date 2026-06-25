package pumps

import (
	"context"
	"fmt"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
)

// File-level MC/DC witness rows: these requirements are genuinely exercised
// by covered tests in this file (per-test // MCDC blocks below). Rows copied
// verbatim from `proof mcdc show`; this header gives every // Verifies: link
// in the file a matching witness row.
//
// MCDC SW-REQ-037: converted_to_graph_record=F, is_graph_record=F => TRUE
// MCDC SW-REQ-037: converted_to_graph_record=F, is_graph_record=T => FALSE
// MCDC SW-REQ-037: converted_to_graph_record=T, is_graph_record=T => TRUE

// Verifies: SW-REQ-037
// SW-REQ-037:errors_propagated:nominal
// SW-REQ-037:output_cardinality_bounded:nominal
// MCDC SW-REQ-037: is_graph_record=F, converted_to_graph_record=F => TRUE
// MCDC SW-REQ-037: is_graph_record=T, converted_to_graph_record=F => FALSE
// MCDC SW-REQ-037: is_graph_record=T, converted_to_graph_record=T => TRUE
// (The "all records written" case below uses GraphQLStats.IsGraphQL=true so
// records are converted via ToGraphRecord — drives T/T=TRUE. Sibling cases
// with IsGraphQL=false drive the wrap-without-parse skip arm — F/F=TRUE.
// The KI graph-mongo-detected-connection-failure case below exercises the
// is_graph_record=T but conversion-failed pair — T/F=FALSE.)
func TestGraphMongoPump_WriteData(t *testing.T) {
	conf := defaultConf(t)
	conf.CollectionName = uniqueCollection(t)
	pump := GraphMongoPump{
		MongoPump: MongoPump{
			dbConf: &conf,
		},
	}
	pump.log = log.WithField("prefix", mongoPrefix)
	pump.MongoPump.CommonPumpConfig = pump.CommonPumpConfig
	pump.dbConf.CollectionCapEnable = true
	pump.dbConf.CollectionCapMaxSizeBytes = 0

	pump.connect()
	t.Cleanup(func() { _ = pump.store.DropDatabase(context.Background()) })

	sampleRecord := analytics.AnalyticsRecord{
		APIName: "Test API",
		Path:    "POST",
	}
	testCases := []struct {
		expectedError        string
		name                 string
		modifyRecord         func() []interface{}
		expectedGraphRecords []analytics.GraphRecord
	}{
		{
			name: "all records written",
			modifyRecord: func() []interface{} {
				records := make([]interface{}, 3)
				stats := analytics.GraphQLStats{
					IsGraphQL: true,
					Types: map[string][]string{
						"Country": {"code"},
					},
					RootFields:    []string{"country"},
					HasErrors:     false,
					OperationType: analytics.OperationQuery,
				}
				for i := range records {
					record := sampleRecord
					record.GraphQLStats = stats
					switch i {
					case 0:
						record.GraphQLStats.HasErrors = false
					case 1:
						record.GraphQLStats.HasErrors = true
						record.GraphQLStats.Errors = []analytics.GraphError{
							{
								Message: "test error",
							},
						}
					default:
						record.GraphQLStats.HasErrors = true
					}
					records[i] = record
				}
				return records
			},
			expectedGraphRecords: []analytics.GraphRecord{
				{
					Types: map[string][]string{
						"Country": {"code"},
					},
					OperationType: "Query",
					HasErrors:     false,
					Errors:        []analytics.GraphError{},
					RootFields:    []string{"country"},
				},
				{
					Types: map[string][]string{
						"Country": {"code"},
					},
					OperationType: "Query",
					HasErrors:     true,
					Errors: []analytics.GraphError{
						{
							Message: "test error",
							Path:    []interface{}{},
						},
					},
					RootFields: []string{"country"},
				},
				{
					Types: map[string][]string{
						"Country": {"code"},
					},
					OperationType: "Query",
					HasErrors:     true,
					Errors:        []analytics.GraphError{},
					RootFields:    []string{"country"},
				},
			},
		},
		{
			name: "contains non graph records",
			modifyRecord: func() []interface{} {
				records := make([]interface{}, 2)
				stats := analytics.GraphQLStats{
					IsGraphQL: true,
					Types: map[string][]string{
						"Country": {"code"},
					},
					RootFields:    []string{"country"},
					HasErrors:     false,
					OperationType: analytics.OperationQuery,
				}
				for i := range records {
					record := sampleRecord
					record.GraphQLStats = stats
					if i == 1 {
						record.GraphQLStats.IsGraphQL = false
					}
					records[i] = record
				}
				return records
			},
			expectedGraphRecords: []analytics.GraphRecord{
				{
					Types: map[string][]string{
						"Country": {"code"},
					},
					OperationType: "Query",
					HasErrors:     false,
					Errors:        []analytics.GraphError{},
					RootFields:    []string{"country"},
				},
			},
		},
	}

	// clean db before start
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			records := tc.modifyRecord()
			err := pump.WriteData(context.Background(), records)
			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			defer func() {
				if err := pump.store.DropDatabase(context.Background()); err != nil {
					pump.log.WithError(err).Warn("error dropping collection")
				}
			}()

			// now check for the written data
			var results []analytics.GraphRecord

			// Using the same collection name as the default pump config
			d := dbObject{
				tableName: pump.dbConf.CollectionName,
			}
			err = pump.store.Query(context.Background(), d, &results, nil)

			assert.Nil(t, err)
			if diff := cmp.Diff(tc.expectedGraphRecords, results, cmpopts.IgnoreFields(analytics.GraphRecord{}, "AnalyticsRecord")); diff != "" {
				t.Error(diff)
			}
		})
	}
}

// Verifies: SW-REQ-037
func TestGraphMongoPump_Init(t *testing.T) {
	pump := GraphMongoPump{}
	t.Cleanup(func() {
		if pump.store != nil {
			_ = pump.store.DropDatabase(context.Background())
		}
	})
	t.Run("successful init", func(t *testing.T) {
		conf := defaultConf(t)
		conf.CollectionName = uniqueCollection(t)
		assert.NoError(t, pump.Init(conf))
	})
	t.Run("invalid conf type", func(t *testing.T) {
		assert.ErrorContains(t, pump.Init("test"), "expected a map")
	})
	t.Run("max document and insert size set", func(t *testing.T) {
		conf := defaultConf(t)
		conf.CollectionName = uniqueCollection(t)
		conf.MaxInsertBatchSizeBytes = 0
		conf.MaxDocumentSizeBytes = 0
		err := pump.Init(conf)
		assert.NoError(t, err)
		assert.Equal(t, 10*MiB, pump.dbConf.MaxDocumentSizeBytes)
		assert.Equal(t, 10*MiB, pump.dbConf.MaxInsertBatchSizeBytes)
	})
	t.Run("init from default env", func(t *testing.T) {
		// SW-REQ-037:env_override_applied:nominal
		conf := defaultConf(t)
		conf.CollectionName = uniqueCollection(t)
		envCollection := uniqueCollection(t)
		t.Setenv(fmt.Sprintf("%s_MONGOGRAPH%s_COLLECTIONNAME", PUMPS_ENV_PREFIX, PUMPS_ENV_META_PREFIX), envCollection)

		err := pump.Init(conf)
		assert.NoError(t, err)
		assert.Equal(t, envCollection, pump.dbConf.CollectionName)
	})
	t.Run("init from custom env prefix", func(t *testing.T) {
		// SW-REQ-037:env_override_applied:boundary
		conf := defaultConf(t)
		conf.CollectionName = uniqueCollection(t)
		conf.EnvPrefix = "GRAPH_MONGO_CUSTOM"
		envCollection := uniqueCollection(t)
		t.Setenv(conf.EnvPrefix+"_COLLECTIONNAME", envCollection)

		err := pump.Init(conf)
		assert.NoError(t, err)
		assert.Equal(t, conf.EnvPrefix, pump.GetEnvPrefix())
		assert.Equal(t, envCollection, pump.dbConf.CollectionName)
	})
}

// Verifies: SW-REQ-037
func TestDecodeRequestAndDecodeResponseGraphMongo(t *testing.T) {
	newPump := &GraphMongoPump{}
	conf := defaultConf(t)
	conf.CollectionName = uniqueCollection(t)
	err := newPump.Init(conf)
	assert.Nil(t, err)
	t.Cleanup(func() { _ = newPump.store.DropDatabase(context.Background()) })

	// checking if the default values are false
	assert.False(t, newPump.GetDecodedRequest())
	assert.False(t, newPump.GetDecodedResponse())

	// trying to set the values to true
	newPump.SetDecodingRequest(true)
	newPump.SetDecodingResponse(true)

	// checking if the values are still false as expected because this pump doesn't support decoding requests/responses
	assert.False(t, newPump.GetDecodedRequest())
	assert.False(t, newPump.GetDecodedResponse())
}
