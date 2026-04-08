package analytics

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

// TODO fix test coverage
func TestAnalyticsRecord_ToGraphRecordNew(t *testing.T) {
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

	testCases := []struct {
		name       string
		graphStats GraphQLStats
		expected   GraphRecord
	}{
		{
			name: "should convert to graph record",
			graphStats: GraphQLStats{
				IsGraphQL: true,
				HasErrors: false,
				Types: map[string][]string{
					"Characters": {"info"},
					"Info":       {"firstField", "secondField"},
				},
				RootFields: []string{"characters"},
				Variables:  `{"id":"hello"}`,
			},
			expected: GraphRecord{
				Types: map[string][]string{
					"Characters": {"info"},
					"Info":       {"firstField", "secondField"},
				},
				RootFields: []string{"characters"},
				Variables:  `{"id":"hello"}`,
				HasErrors:  false,
			},
		},
		{
			name: "isn't graphql record",
			graphStats: GraphQLStats{
				IsGraphQL: false,
				HasErrors: false,
				Types: map[string][]string{
					"Characters": {"info"},
					"Info":       {"firstField", "secondField"},
				},
				RootFields: []string{"characters"},
				Variables:  `{"id":"hello"}`,
			},
		},
		{
			name: "has error",
			graphStats: GraphQLStats{
				IsGraphQL: true,
				HasErrors: true,
				Types: map[string][]string{
					"Characters": {"info"},
					"Info":       {"firstField", "secondField"},
				},
				RootFields: []string{"characters"},
				Variables:  `{"id":"hello"}`,
			},
			expected: GraphRecord{
				Types: map[string][]string{
					"Characters": {"info"},
					"Info":       {"firstField", "secondField"},
				},
				RootFields: []string{"characters"},
				Variables:  `{"id":"hello"}`,
				HasErrors:  true,
			},
		},
		{
			name: "has error with error",
			graphStats: GraphQLStats{
				IsGraphQL: true,
				HasErrors: true,
				Errors: []GraphError{
					{
						Message: "sample error",
					},
				},
				Types: map[string][]string{
					"Characters": {"info"},
					"Info":       {"firstField", "secondField"},
				},
				RootFields: []string{"characters"},
				Variables:  `{"id":"hello"}`,
			},
			expected: GraphRecord{
				Types: map[string][]string{
					"Characters": {"info"},
					"Info":       {"firstField", "secondField"},
				},
				RootFields: []string{"characters"},
				Variables:  `{"id":"hello"}`,
				HasErrors:  true,
				Errors: []GraphError{
					{
						Message: "sample error",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			record := recordSample
			record.GraphQLStats = tc.graphStats
			gotten := record.ToGraphRecord()
			if diff := cmp.Diff(tc.expected, gotten, cmpopts.IgnoreFields(GraphRecord{}, "AnalyticsRecord")); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
