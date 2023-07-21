package pumps

import (
	"context"
	"encoding/base64"
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

const rawGQLResponseWithError = `HTTP/0.0 200 OK
Content-Length: 61
Connection: close
Content-Type: application/json
X-Ratelimit-Limit: 0
X-Ratelimit-Remaining: 0
X-Ratelimit-Reset: 0

{"data":{"country":null},"errors":[{"message":"test error"}]}`

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

const rawHTTPReq = `GET /get HTTP/1.1
Host: localhost:8181
User-Agent: PostmanRuntime/7.29.2
Accept: */*
Accept-Encoding: gzip, deflate, br
Postman-Token: a67c3054-aa1a-47f3-9bca-5dbde04c8565
`

const rawHTTPResponse = `
HTTP/1.1 200 OK
Content-Length: 376
Access-Control-Allow-Credentials: true
Access-Control-Allow-Origin: *
Connection: close
Content-Type: application/json
Date: Tue, 04 Oct 2022 06:33:23 GMT
Server: gunicorn/19.9.0
X-Ratelimit-Limit: 0
X-Ratelimit-Remaining: 0
X-Ratelimit-Reset: 0

{
  "args": {}, 
  "headers": {
    "Accept": "*/*", 
    "Accept-Encoding": "gzip, deflate, br", 
    "Host": "httpbin.org", 
    "Postman-Token": "a67c3054-aa1a-47f3-9bca-5dbde04c8565", 
    "User-Agent": "PostmanRuntime/7.29.2", 
    "X-Amzn-Trace-Id": "Root=1-633bd3b3-6345504724f3295b68d7dcd3"
  }, 
  "origin": "::1, 102.89.45.253", 
  "url": "http://httpbin.org/get"
}

`

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

	type customRecord struct {
		rawRequest   string
		rawResponse  string
		schema       string
		tags         []string
		responseCode int
	}

	testCases := []struct {
		expectedError        string
		name                 string
		expectedGraphRecords []analytics.GraphRecord
		records              []customRecord
	}{
		{
			name: "all records written",
			records: []customRecord{
				{
					rawRequest:  rawGQLRequest,
					rawResponse: rawGQLResponse,
					schema:      schema,
					tags:        []string{analytics.PredefinedTagGraphAnalytics},
				},
				{
					rawRequest:  rawGQLRequest,
					rawResponse: rawGQLResponseWithError,
					schema:      schema,
					tags:        []string{analytics.PredefinedTagGraphAnalytics},
				},
				{
					rawRequest:   rawGQLRequest,
					rawResponse:  rawGQLResponse,
					schema:       schema,
					tags:         []string{analytics.PredefinedTagGraphAnalytics},
					responseCode: 500,
				},
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
			records: []customRecord{
				{
					rawRequest:  rawGQLRequest,
					rawResponse: rawGQLResponse,
					schema:      schema,
					tags:        []string{analytics.PredefinedTagGraphAnalytics},
				},
				{
					rawRequest:  rawHTTPReq,
					rawResponse: rawHTTPResponse,
				},
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
		{
			name: "should be empty on empty request response",
			records: []customRecord{
				{
					rawRequest:  "",
					rawResponse: rawGQLResponse,
					schema:      schema,
					tags:        []string{analytics.PredefinedTagGraphAnalytics},
				},
				{
					rawResponse: "",
					rawRequest:  rawGQLRequest,
					schema:      schema,
					tags:        []string{analytics.PredefinedTagGraphAnalytics},
				},
				{
					rawRequest:  rawGQLRequest,
					rawResponse: rawGQLResponse,
					tags:        []string{analytics.PredefinedTagGraphAnalytics},
				},
			},
			expectedGraphRecords: []analytics.GraphRecord{
				{
					Types:      map[string][]string{},
					Errors:     []analytics.GraphError{},
					RootFields: []string{},
				},
				{
					Types:      map[string][]string{},
					Errors:     []analytics.GraphError{},
					RootFields: []string{},
				},
				{
					Types:      map[string][]string{},
					Errors:     []analytics.GraphError{},
					RootFields: []string{},
				},
			},
		},
	}

	// clean db before start
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			records := make([]interface{}, 0)
			for _, cr := range tc.records {
				r := analytics.AnalyticsRecord{
					APIName:     "Test API",
					Path:        "POST",
					RawRequest:  base64.StdEncoding.EncodeToString([]byte(cr.rawRequest)),
					RawResponse: base64.StdEncoding.EncodeToString([]byte(cr.rawResponse)),
					ApiSchema:   base64.StdEncoding.EncodeToString([]byte(cr.schema)),
					Tags:        cr.tags,
				}
				if cr.responseCode != 0 {
					r.ResponseCode = cr.responseCode
				}
				records = append(records, r)
			}

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
