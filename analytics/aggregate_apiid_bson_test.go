package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	mgobson "gopkg.in/mgo.v2/bson"
)

type apiIDHolder struct {
	APIID APIIDCounters `bson:"apiid"`
}

// decodeWithBothDrivers decodes doc with the mongo-go and the mgo BSON decoders.
func decodeWithBothDrivers(t *testing.T, doc bson.D) map[string]APIIDCounters {
	t.Helper()
	data, err := bson.Marshal(doc)
	require.NoError(t, err)

	var mongoGo, mgo apiIDHolder
	require.NoError(t, bson.Unmarshal(data, &mongoGo))
	require.NoError(t, mgobson.Unmarshal(data, &mgo))

	return map[string]APIIDCounters{"mongo-go": mongoGo.APIID, "mgo": mgo.APIID}
}

// bsonDoc builds a bson.D from alternating keys and values.
func bsonDoc(kv ...interface{}) bson.D {
	doc := bson.D{}
	for i := 0; i < len(kv); i += 2 {
		doc = append(doc, bson.E{Key: kv[i].(string), Value: kv[i+1]})
	}
	return doc
}

func counterDoc(identifier string, hits int, kv ...interface{}) bson.D {
	return append(bsonDoc("hits", hits, "identifier", identifier), bsonDoc(kv...)...)
}

func TestAPIIDCounters_DecodeBSON(t *testing.T) {
	tests := []struct {
		name string
		doc  bson.D
		want map[string]int // API ID -> hits
	}{
		{
			name: "API ID without a dot",
			doc:  bsonDoc("apiid", bsonDoc("abc123", counterDoc("abc123", 3))),
			want: map[string]int{"abc123": 3},
		},
		{
			name: "API ID with dots is stored nested",
			doc:  bsonDoc("apiid", bsonDoc("host", bsonDoc("test", bsonDoc("pl", counterDoc("host.test.pl", 7))))),
			want: map[string]int{"host.test.pl": 7},
		},
		{
			name: "API IDs sharing a first segment, and an API named after it",
			doc: bsonDoc("apiid", bsonDoc("bc", counterDoc("bc", 2,
				"example", bsonDoc("com", counterDoc("bc.example.com", 4)),
				"other", bsonDoc("com", counterDoc("bc.other.com", 6)),
			))),
			want: map[string]int{"bc": 2, "bc.example.com": 4, "bc.other.com": 6},
		},
		{
			name: "truncated level written by older Pump versions is not an API",
			doc: bsonDoc("apiid", bsonDoc("host", bsonDoc(
				"errorlist", bson.A{},
				"latency", 0.0,
				"requesttime", 0.0,
				"upstreamlatency", 0.0,
				"test", bsonDoc("pl", counterDoc("host.test.pl", 7)),
			))),
			want: map[string]int{"host.test.pl": 7},
		},
		{
			name: "errormap as a segment of an API ID",
			doc:  bsonDoc("apiid", bsonDoc("host", bsonDoc("errormap", counterDoc("host.errormap", 5)))),
			want: map[string]int{"host.errormap": 5},
		},
		{
			name: "empty apiid",
			doc:  bsonDoc("apiid", bson.D{}),
			want: map[string]int{},
		},
		{
			name: "null apiid",
			doc:  bsonDoc("apiid", nil),
			want: map[string]int{},
		},
		{
			name: "missing apiid",
			doc:  bson.D{},
			want: map[string]int{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for driver, got := range decodeWithBothDrivers(t, tc.doc) {
				hits := map[string]int{}
				for id, c := range got {
					hits[id] = c.Hits
				}
				assert.Equal(t, tc.want, hits, driver)
			}
		})
	}
}

func TestAPIIDCounters_DecodeFullCounter(t *testing.T) {
	want := Counter{
		Hits:             10,
		Success:          8,
		ErrorTotal:       2,
		RequestTime:      12.5,
		TotalRequestTime: 125,
		Identifier:       "host.test.pl",
		HumanIdentifier:  "Test API",
		LastTime:         time.Date(2026, 10, 7, 10, 15, 0, 0, time.UTC),
		BytesIn:          100,
		BytesOut:         200,
		MaxLatency:       30,
		MinLatency:       5,
		TotalLatency:     150,
		Latency:          15,
		ErrorMap:         map[string]int{"404": 1, "500": 1},
	}
	stored, err := bson.Marshal(want)
	require.NoError(t, err)

	doc := bsonDoc("apiid", bsonDoc("host", bsonDoc("test", bsonDoc("pl", bson.Raw(stored)))))
	for driver, got := range decodeWithBothDrivers(t, doc) {
		require.Len(t, got, 1, driver)
		require.Contains(t, got, "host.test.pl", driver)
		assert.Equal(t, want, *got["host.test.pl"], driver)
	}
}

func TestAggregateData_DottedAPIIDUnchanged(t *testing.T) {
	records := []interface{}{
		AnalyticsRecord{APIID: "host.test.pl", OrgID: "org1", ResponseCode: 200, TimeStamp: time.Now()},
	}

	agg := AggregateData(records, false, nil, "", 60)["org1"]
	assert.Contains(t, agg.APIID, "host.test.pl")

	// SQL aggregate pumps and the hybrid pump (MDCB) use Dimensions() values as the API ID.
	values := []string{}
	for _, d := range agg.Dimensions() {
		if d.Name == "apiid" {
			values = append(values, d.Value)
		}
	}
	assert.Equal(t, []string{"host.test.pl"}, values)
}
