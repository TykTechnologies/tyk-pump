// contents lightly modified from github.com/go-mgo/mgo@3569c88678d88179dcbd68d02ab081cbca3cd4d0

package mgohacks

import (
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// modified copy of mgo.indexSpec that removes "omitempty" from ExpireAfter
type ttlIndexSpec struct {
	Name, NS         string
	Key              bson.D
	Unique           bool    ",omitempty"
	DropDups         bool    "dropDups,omitempty"
	Background       bool    ",omitempty"
	Sparse           bool    ",omitempty"
	Bits, Min, Max   int     ",omitempty"
	BucketSize       float64 "bucketSize,omitempty"
	ExpireAfter      int     "expireAfterSeconds"
	Weights          bson.D  ",omitempty"
	DefaultLanguage  string  "default_language,omitempty"
	LanguageOverride string  "language_override,omitempty"
}

// EnsureTTLIndex mirrors mgo.EnsureIndex but always provides the "ExpireAfter" field (allowing it to bet set to 0).
// For internal simplicity this does not perform any index existence caching.
func EnsureTTLIndex(c *mgo.Collection, index mgo.Index) error {
	keyInfo, err := parseIndexKey(index.Key)
	if err != nil {
		return err
	}

	session := c.Database.Session
	spec := ttlIndexSpec{
		Name:             keyInfo.name,
		NS:               c.FullName,
		Key:              keyInfo.key,
		Unique:           index.Unique,
		DropDups:         index.DropDups,
		Background:       index.Background,
		Sparse:           index.Sparse,
		Bits:             index.Bits,
		Min:              index.Min,
		Max:              index.Max,
		BucketSize:       index.BucketSize,
		ExpireAfter:      int(index.ExpireAfter / time.Second),
		Weights:          keyInfo.weights,
		DefaultLanguage:  index.DefaultLanguage,
		LanguageOverride: index.LanguageOverride,
	}

NextField:
	for name, weight := range index.Weights {
		for i, elem := range spec.Weights {
			if elem.Name == name {
				spec.Weights[i].Value = weight
				continue NextField
			}
		}
		panic("weight provided for field that is not part of index key: " + name)
	}

	cloned := session.Clone()
	defer cloned.Close()
	cloned.SetMode(mgo.Strong, false)
	cloned.EnsureSafe(&mgo.Safe{})
	db := c.Database.With(cloned)

	return db.Run(bson.D{{"createIndexes", c.Name}, {"indexes", []ttlIndexSpec{spec}}}, nil)
}
