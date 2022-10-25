package analytics

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/TykTechnologies/graphql-go-tools/pkg/ast"
	"github.com/TykTechnologies/graphql-go-tools/pkg/astparser"
	gql "github.com/TykTechnologies/graphql-go-tools/pkg/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
)

const (
	requestTemplate  = "POST / HTTP/1.1\r\nHost: localhost:8281\r\nUser-Agent: test-agent\r\nContent-Length: %d\r\n\r\n%s"
	responseTemplate = "HTTP/0.0 200 OK\r\nContent-Length: %d\r\nConnection: close\r\nContent-Type: application/json\r\n\r\n%s"
)

const sampleSchema = `
type Query {
  characters(filter: FilterCharacter, page: Int): Characters
  listCharacters(): [Characters]!
}

type Mutation {
  changeCharacter(): String
}

type Subscription {
  listenCharacter(): Characters
}
input FilterCharacter {
  name: String
  status: String
  species: String
  type: String
  gender: String! = "M"
}
type Characters {
  info: Info
  secondInfo: String
  results: [Character]
}
type Info {
  count: Int
  next: Int
  pages: Int
  prev: Int
}
type Character {
  gender: String
  id: ID
  name: String
}

type EmptyType{
}`

func getSampleSchema() (*ast.Document, error) {
	schema, err := gql.NewSchemaFromString(string(sampleSchema))
	if err != nil {
		return nil, err
	}
	schemaDoc, operationReport := astparser.ParseGraphqlDocumentBytes(schema.Document())
	if operationReport.HasErrors() {
		return nil, operationReport
	}
	return &schemaDoc, nil
}

func TestAnalyticsRecord_ToGraphRecord(t *testing.T) {
	recordSample := AnalyticsRecord{
		TimeStamp:    time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		Method:       "POST",
		Host:         "localhost:8281",
		Path:         "/",
		RawPath:      "/",
		APIName:      "test-api",
		APIID:        "test-api",
		ApiSchema:    base64.StdEncoding.EncodeToString([]byte(sampleSchema)),
		ResponseCode: 200,
		Day:          1,
		Month:        1,
		Year:         2022,
		Hour:         0,
	}
	graphRecordSample := GraphRecord{
		AnalyticsRecord: recordSample,
		Types:           make(map[string][]string),
	}

	testCases := []struct {
		expected     func(string, string) GraphRecord
		modifyRecord func(a AnalyticsRecord) AnalyticsRecord
		title        string
		request      string
		response     string
		expectedErr  string
	}{
		{
			title:    "no error",
			request:  `{"query":"query{\n  characters(filter: {\n    \n  }){\n    info{\n      count\n    }\n  }\n}"}`,
			response: `{"data":{"characters":{"info":{"count":758}}}}`,
			expected: func(request, response string) GraphRecord {
				g := graphRecordSample
				g.HasErrors = false
				g.Types = map[string][]string{
					"Characters": {"info"},
					"Info":       {"count"},
				}
				g.OperationType = "Query"
				return g
			},
		},
		{
			title:    "no error mutation",
			request:  `{"query":"mutation{\n  changeCharacter()\n}"}`,
			response: `{"data":{"characters":{"info":{"count":758}}}}`,
			expected: func(request, response string) GraphRecord {
				g := graphRecordSample
				g.HasErrors = false
				g.OperationType = "Mutation"
				return g
			},
		},
		{
			title:    "no error subscription",
			request:  `{"query":"subscription{\n  listenCharacter(){\n    info{\n      count\n    }\n  }\n}"}`,
			response: `{"data":{"characters":{"info":{"count":758}}}}`,
			expected: func(request, response string) GraphRecord {
				g := graphRecordSample
				g.HasErrors = false
				g.Types = map[string][]string{
					"Characters": {"info"},
					"Info":       {"count"},
				}
				g.OperationType = "Subscription"
				return g
			},
		},
		{
			title:    "bad document",
			request:  `{"query":"subscriptiona{\n  listenCharacter(){\n    info{\n      count\n    }\n  }\n}"}`,
			response: `{"data":{"characters":{"info":{"count":758}}}}`,
			expected: func(request, response string) GraphRecord {
				return GraphRecord{}
			},
			expectedErr: "error generating documents",
		},
		{
			title:    "no error list operation",
			request:  `{"query":"query{\n  listCharacters(){\n    info{\n      count\n    }\n  }\n}"}`,
			response: `{"data":{"characters":{"info":{"count":758}}}}`,
			expected: func(request, response string) GraphRecord {
				g := graphRecordSample
				g.HasErrors = false
				g.Types = map[string][]string{
					"Characters": {"info"},
					"Info":       {"count"},
				}
				g.OperationType = "Query"
				return g
			},
		},
		{
			title:    "has variables",
			request:  `{"query":"query{\n  characters(filter: {\n    \n  }){\n    info{\n      count\n    }\n  }\n}","variables":{"a":"test"}}`,
			response: `{"data":{"characters":{"info":{"count":758}}}}`,
			expected: func(request, response string) GraphRecord {
				g := graphRecordSample
				g.HasErrors = false
				g.Types = map[string][]string{
					"Characters": {"info"},
					"Info":       {"count"},
				}
				g.OperationType = "Query"
				g.Variables = base64.StdEncoding.EncodeToString([]byte(`{"a":"test"}`))
				return g
			},
		},
		{
			title:    "no operation",
			request:  `{"query":"query main {\ncharacters {\ninfo\n}\n}\n\nquery second {\nlistCharacters{\ninfo\n}\n}","variables":null,"operationName":""}`,
			response: `{"data":{"characters":{"info":{"count":758}}}}`,
			expected: func(request, response string) GraphRecord {
				return GraphRecord{}
			},
			expectedErr: "no operation name specified",
		},
		{
			title:    "operation name specified",
			request:  `{"query":"query main {\ncharacters {\ninfo\n}\n}\n\nquery second {\nlistCharacters{\ninfo\n secondInfo}\n}","variables":null,"operationName":"second"}`,
			response: `{"data":{"characters":{"info":{"count":758}}}}`,
			expected: func(request, response string) GraphRecord {
				g := graphRecordSample
				g.HasErrors = false
				g.Types = map[string][]string{
					"Characters": {"info", "secondInfo"},
				}
				g.OperationType = "Query"
				return g
			},
			expectedErr: "",
		},
		{
			title:   "has errors",
			request: `{"query":"query{\n  characters(filter: {\n    \n  }){\n    info{\n      count\n    }\n  }\n}"}`,
			response: `{
  "errors": [
    {
      "message": "Name for character with ID 1002 could not be fetched.",
      "locations": [{ "line": 6, "column": 7 }],
      "path": ["hero", "heroFriends", 1, "name"]
    }
  ]
}`,
			expected: func(request, response string) GraphRecord {
				g := graphRecordSample
				g.HasErrors = true
				g.Types = map[string][]string{
					"Characters": {"info"},
					"Info":       {"count"},
				}
				g.OperationType = "Query"
				g.Errors = append(g.Errors, GraphError{
					Message: "Name for character with ID 1002 could not be fetched.",
					Path:    []interface{}{"hero", "heroFriends", float64(1), "name"},
				})
				return g
			},
		},
		{
			title: "corrupted raw request should error out",
			modifyRecord: func(a AnalyticsRecord) AnalyticsRecord {
				a.RawRequest = "this isn't a base64 is it?"
				return a
			},
			expectedErr: "error decoding raw request",
			expected: func(s, s2 string) GraphRecord {
				return GraphRecord{}
			},
		},
		{
			title: "corrupted schema should error out",
			modifyRecord: func(a AnalyticsRecord) AnalyticsRecord {
				a.ApiSchema = "this isn't a base64 is it?"
				return a
			},
			expectedErr: "error decoding schema",
			expected: func(s, s2 string) GraphRecord {
				return GraphRecord{}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.title, func(t *testing.T) {
			a := recordSample
			a.RawRequest = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(
				requestTemplate,
				len(testCase.request),
				testCase.request,
			)))
			a.RawResponse = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(
				responseTemplate,
				len(testCase.response),
				testCase.response,
			)))
			if testCase.modifyRecord != nil {
				a = testCase.modifyRecord(a)
			}
			expected := testCase.expected(testCase.request, testCase.response)
			expected.AnalyticsRecord = a
			gotten, err := a.ToGraphRecord()
			if testCase.expectedErr != "" {
				assert.ErrorContains(t, err, testCase.expectedErr)
				return
			}
			assert.NoError(t, err)
			if diff := cmp.Diff(expected, gotten, cmpopts.IgnoreFields(GraphRecord{}, "RawRequest", "RawResponse")); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func Test_getObjectTypeRefWithName(t *testing.T) {
	schema, err := getSampleSchema()
	assert.NoError(t, err)

	testCases := []struct {
		name        string
		typeName    string
		expectedRef int
	}{
		{
			name:        "fail",
			typeName:    "invalidType",
			expectedRef: -1,
		},
		{
			name:        "successful",
			typeName:    "Character",
			expectedRef: 5,
		},
		{
			name:        "invalid because input",
			typeName:    "FilterCharacter",
			expectedRef: -1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ref := getObjectTypeRefWithName(tc.typeName, schema)
			assert.Equal(t, tc.expectedRef, ref)
		})
	}
}

func Test_getObjectFieldRefWithName(t *testing.T) {
	schema, err := getSampleSchema()
	assert.NoError(t, err)

	testCases := []struct {
		name        string
		fieldName   string
		objectName  string
		expectedRef int
	}{
		{
			name:        "successful run",
			fieldName:   "info",
			objectName:  "Characters",
			expectedRef: 8,
		},
		{
			name:        "failed run due to invalid field",
			fieldName:   "infos",
			objectName:  "Characters",
			expectedRef: -1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objRef := getObjectTypeRefWithName(tc.objectName, schema)
			assert.NotEqual(t, -1, objRef)
			ref := getObjectFieldRefWithName(tc.fieldName, objRef, schema)
			assert.Equal(t, tc.expectedRef, ref)
		})
	}
}

func Test_generateNormalizedDocuments(t *testing.T) {
	rQuery := `{"query":"mutation{\n  changeCharacter()\n}"}`
	sampleQuery := []byte(fmt.Sprintf(requestTemplate, len(rQuery), rQuery))

	t.Run("test valid request", func(t *testing.T) {
		_, _, _, err := generateNormalizedDocuments(sampleQuery, []byte(sampleSchema))
		assert.NoError(t, err)
	})
	t.Run("test invalid request", func(t *testing.T) {
		_, _, _, err := generateNormalizedDocuments(sampleQuery[:10], []byte(sampleSchema))
		assert.ErrorContains(t, err, `malformed HTTP version "HTT"`)
	})
	t.Run("invalid schema", func(t *testing.T) {
		_, _, _, err := generateNormalizedDocuments(sampleQuery, []byte(`type Test{`))
		assert.Error(t, err)
	})
	t.Run("invalid request for normalization", func(t *testing.T) {
		query := `{"query":"mutation{\n  changeCharactersss()\n}"}`
		_, _, _, err := generateNormalizedDocuments([]byte(fmt.Sprintf(requestTemplate, len(query), query)), []byte(sampleSchema))
		assert.Error(t, err)
	})
}

func Test_getOperationSelectionFieldDefinition(t *testing.T) {
	schema, err := getSampleSchema()
	assert.NoError(t, err)

	testCases := []struct {
		modifySchema  func(ast.Document) *ast.Document
		name          string
		operationName string
		expectedErr   string
		expectedRef   int
		operationType ast.OperationType
	}{
		{
			name:          "successful query",
			operationType: ast.OperationTypeQuery,
			operationName: "characters",
			expectedRef:   0,
			expectedErr:   "",
		},
		{
			name:          "invalid query",
			operationType: ast.OperationTypeQuery,
			operationName: "invalidQuery",
			expectedRef:   -1,
			expectedErr:   "field not found",
		},
		{
			name:          "invalid query type name",
			operationType: ast.OperationTypeQuery,
			operationName: "testOperation",
			expectedRef:   -1,
			expectedErr:   "missing query type declaration",
			modifySchema: func(document ast.Document) *ast.Document {
				document.Index.QueryTypeName = ast.ByteSlice("Querys")
				return &document
			},
		},
		{
			name:          "invalid mutation type name",
			operationType: ast.OperationTypeMutation,
			operationName: "testOperation",
			expectedRef:   -1,
			expectedErr:   "missing mutation type declaration",
			modifySchema: func(document ast.Document) *ast.Document {
				document.Index.MutationTypeName = ast.ByteSlice("Mutations")
				return &document
			},
		},
		{
			name:          "invalid subscription type name",
			operationType: ast.OperationTypeSubscription,
			operationName: "testOperation",
			expectedRef:   -1,
			expectedErr:   "missing subscription type declaration",
			modifySchema: func(document ast.Document) *ast.Document {
				document.Index.SubscriptionTypeName = ast.ByteSlice("Subscriptions")
				return &document
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var sc *ast.Document
			if tc.modifySchema != nil {
				sc = tc.modifySchema(*schema)
			} else {
				sc = schema
			}
			ref, err := getOperationSelectionFieldDefinition(tc.operationType, tc.operationName, sc)
			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedRef, ref)
		})
	}
}
