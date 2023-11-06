package pumps

import (
	"context"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
)

const rawGQLRequest = `POST / HTTP/1.1
Host: localhost:8181
User-Agent: PostmanRuntime/7.29.2
Content-Length: 58
Accept: */*
Accept-Encoding: gzip, deflate, br
Content-Type: application/json
Postman-Token: e6d4bc44-3268-40ae-888b-d84bb5ea07fd

{"query":"{\n  country(code: \"NGN\"){\n    code\n  }\n}"}`

const rawGQLResponse = `HTTP/0.0 200 OK
Content-Length: 25
Connection: close
Content-Type: application/json
X-Ratelimit-Limit: 0
X-Ratelimit-Remaining: 0
X-Ratelimit-Reset: 0

{"data":{"country":null}}`

const schema = `type Query {
  countries(filter: CountryFilterInput): [Country!]!
  country(code: ID!): Country
  continents(filter: ContinentFilterInput): [Continent!]!
  continent(code: ID!): Continent
  languages(filter: LanguageFilterInput): [Language!]!
  language(code: ID!): Language
}

type Country {
  code: ID!
  name: String!
  native: String!
  phone: String!
  continent: Continent!
  capital: String
  currency: String
  languages: [Language!]!
  emoji: String!
  emojiU: String!
  states: [State!]!
}

type Continent {
  code: ID!
  name: String!
  countries: [Country!]!
}

type Language {
  code: ID!
  name: String
  native: String
  rtl: Boolean!
}

type State {
  code: String
  name: String!
  country: Country!
}

input StringQueryOperatorInput {
  eq: String
  ne: String
  in: [String]
  nin: [String]
  regex: String
  glob: String
}

input CountryFilterInput {
  code: StringQueryOperatorInput
  currency: StringQueryOperatorInput
  continent: StringQueryOperatorInput
}

input ContinentFilterInput {
  code: StringQueryOperatorInput
}

input LanguageFilterInput {
  code: StringQueryOperatorInput
}`

func TestGraphMongoPump_WriteData(t *testing.T) {
	conf := defaultConf()
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
				for i, _ := range records {
					record := sampleRecord
					record.GraphQLStats = stats
					if i == 0 {
						record.GraphQLStats.HasErrors = false
					} else if i == 1 {
						record.GraphQLStats.HasErrors = true
						record.GraphQLStats.Errors = []analytics.GraphError{
							{
								Message: "test error",
							},
						}
					} else {
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
				for i, _ := range records {
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

func TestGraphMongoPump_Init(t *testing.T) {
	pump := GraphMongoPump{}
	t.Run("successful init", func(t *testing.T) {
		conf := defaultConf()
		assert.NoError(t, pump.Init(conf))
	})
	t.Run("invalid conf type", func(t *testing.T) {
		assert.ErrorContains(t, pump.Init("test"), "expected a map")
	})
	t.Run("max document and insert size set", func(t *testing.T) {
		conf := defaultConf()
		conf.MaxInsertBatchSizeBytes = 0
		conf.MaxDocumentSizeBytes = 0
		err := pump.Init(conf)
		assert.NoError(t, err)
		assert.Equal(t, 10*MiB, pump.dbConf.MaxDocumentSizeBytes)
		assert.Equal(t, 10*MiB, pump.dbConf.MaxInsertBatchSizeBytes)
	})
}

func TestDecodeRequestAndDecodeResponseGraphMongo(t *testing.T) {
	newPump := &GraphMongoPump{}
	conf := defaultConf()
	err := newPump.Init(conf)
	assert.Nil(t, err)

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
