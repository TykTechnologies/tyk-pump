package analytics

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	mgobson "gopkg.in/mgo.v2/bson"
)

// APIIDCounters holds the per-API counters of an aggregate, keyed by API ID.
//
// MongoDB treats "." in a field path as nesting, so the counters of an API ID
// such as "host.example.com" are stored at apiid.host.example.com rather than
// under a single key. Decoding from BSON walks the nested documents and keys
// each counter by its full dotted path, so the read-back aggregate sees the
// real API ID instead of a truncated "host" key with an empty counter.
type APIIDCounters map[string]*Counter

// UnmarshalBSON is used by the official mongo driver.
func (m *APIIDCounters) UnmarshalBSON(data []byte) error {
	out := APIIDCounters{}
	if err := collectAPIIDCounters(bson.Raw(data), "", out); err != nil {
		return err
	}
	*m = out
	return nil
}

// SetBSON is used by the mgo driver. The raw document bytes are standard BSON.
func (m *APIIDCounters) SetBSON(raw mgobson.Raw) error {
	if raw.Kind != byte(bsontype.EmbeddedDocument) {
		*m = APIIDCounters{}
		return nil
	}
	return m.UnmarshalBSON(raw.Data)
}

// collectAPIIDCounters adds every counter found under doc to out. A document is
// a counter when it has a "hits" or "identifier" field. Other embedded
// documents (except a counter's own errormap) are walked as the next segment
// of a dotted API ID.
func collectAPIIDCounters(doc bson.Raw, prefix string, out APIIDCounters) error {
	elems, err := doc.Elements()
	if err != nil {
		return err
	}
	docIsCounter := prefix != "" && isCounterDocument(doc)
	for _, e := range elems {
		val := e.Value()
		if val.Type != bsontype.EmbeddedDocument {
			continue
		}
		key := e.Key()
		if docIsCounter && key == "errormap" {
			continue
		}
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		child := val.Document()
		if isCounterDocument(child) {
			c := &Counter{}
			if err := bson.Unmarshal(child, c); err != nil {
				return err
			}
			out[path] = c
		}
		if err := collectAPIIDCounters(child, path, out); err != nil {
			return err
		}
	}
	return nil
}

func isCounterDocument(doc bson.Raw) bool {
	if _, err := doc.LookupErr("hits"); err == nil {
		return true
	}
	_, err := doc.LookupErr("identifier")
	return err == nil
}
