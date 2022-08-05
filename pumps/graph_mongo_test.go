package pumps

import (
	"encoding/base64"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/assert"
	"testing"
)

const rawRequestSample = "POST / HTTP/1.1\r\nHost: localhost:8281\r\nUser-Agent: PostmanRuntime/7.29.2\r\nContent-Length: 94\r\nAccept: */*\r\nAccept-Encoding: gzip, deflate, br\r\nContent-Type: application/json\r\nPostman-Token: 594d66bc-762e-4ff3-8d27-eefc4c8c98f2\r\n\r\n{\"query\":\"query{\\n  characters(filter: {\\n    \\n  }){\\n    info{\\n      count\\n    }\\n  }\\n}\"}"

const sampleSchema = `
type Query {
  characters(filter: FilterCharacter, page: Int): Characters
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
}`

func TestGraphMongoPump_recordToGraphRecord(t *testing.T) {
	pump := GraphMongoPump{
		log: log.WithFields(logrus.Fields{
			"prefix": "test-pump",
		}),
	}

	_, err := pump.recordToGraphRecord(analytics.AnalyticsRecord{
		RawRequest: base64.StdEncoding.EncodeToString([]byte(rawRequestSample)),
		Metadata: map[string]string{
			"graphql-schema": base64.StdEncoding.EncodeToString([]byte(sampleSchema)),
		},
	})
	assert.NoError(t, err)
}
