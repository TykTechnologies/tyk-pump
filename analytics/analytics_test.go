package analytics

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
}`

func TestAnalyticsRecord_IsGraphRecord(t *testing.T) {
	t.Run("should return false when no tags are available", func(t *testing.T) {
		record := AnalyticsRecord{}
		assert.False(t, record.IsGraphRecord())
	})

	t.Run("should return false when tags do not contain the graph analytics tag", func(t *testing.T) {
		record := AnalyticsRecord{
			Tags: []string{"tag_1", "tag_2", "tag_3"},
		}
		assert.False(t, record.IsGraphRecord())
	})

	t.Run("should return true when tags contain the graph analytics tag", func(t *testing.T) {
		record := AnalyticsRecord{
			Tags: []string{"tag_1", "tag_2", PredefinedTagGraphAnalytics, "tag_4", "tag_5"},
		}
		assert.True(t, record.IsGraphRecord())
	})
}
