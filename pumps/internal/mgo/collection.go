package mgo

import (
	"github.com/jinzhu/copier"
	mgo "gopkg.in/mgo.v2"
)

// CollectionManager is an interface for mgo.Collection struct.
// All implemented methods returns interfaces when needed
type CollectionManager interface {
	Count() (int, error)
	Create(*CollectionInfo) error
	DropCollection() error
	Insert(docs ...interface{}) error
	EnsureIndex(Index) error
	Indexes() ([]Index, error)
}

type CollectionInfo struct {
	// If Capped is true new documents will replace old ones when
	// the collection is full. MaxBytes must necessarily be set
	// to define the size when the collection wraps around.
	// MaxDocs optionally defines the number of documents when it
	// wraps, but MaxBytes still needs to be set.
	Capped   bool
	MaxBytes int
}

type Index struct {
	// Index key fields; prefix name with dash (-) for descending order
	Key []string
	// Prevent two documents from having the same index key
	Name string
	// Drop documents with the same index key as a previously indexed one
	Background bool
}

type Collection struct {
	collection *mgo.Collection
}

func NewCollectionManager(c *mgo.Collection) CollectionManager {
	return &Collection{
		collection: c,
	}
}

func (c *Collection) Count() (int, error) {
	return c.collection.Count()
}

func (c *Collection) Create(info *CollectionInfo) error {
	mgoInfo := &mgo.CollectionInfo{}
	copier.Copy(&mgoInfo, &info)
	return c.collection.Create(mgoInfo)
}

func (c *Collection) DropCollection() error {
	return c.collection.DropCollection()
}

func (c *Collection) Insert(docs ...interface{}) error {
	return c.collection.Insert(docs...)
}

func (c *Collection) EnsureIndex(index Index) error {
	mgoIndex := mgo.Index{}
	copier.Copy(&mgoIndex, &index)

	return c.collection.EnsureIndex(mgoIndex)
}

func (c *Collection) Indexes() (indexes []Index, err error) {
	mgoIndexes, err := c.collection.Indexes()
	if err != nil {
		return indexes, err
	}
	copier.Copy(&indexes, &mgoIndexes)

	return indexes, nil
}
