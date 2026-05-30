// Test Helper for Mongo

package pumps

import (
	"context"
	"os"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
)

const (
	dbAddr  = "mongodb://localhost:27017/test"
	colName = "test_collection"
)

type Conn struct {
	Store persistent.PersistentStorage
}

// Verifies: SW-REQ-018
func (c *Conn) TableName() string {
	return colName
}

// Verifies: SW-REQ-018
func (*Conn) GetObjectID() model.ObjectID {
	return ""
}

// SetObjectID is a dummy function to satisfy the interface
// Verifies: SW-REQ-018
func (*Conn) SetObjectID(model.ObjectID) {
	// empty
}

// Verifies: SW-REQ-018
func (c *Conn) ConnectDb() {
	if c.Store == nil {
		var err error
		c.Store, err = persistent.NewPersistentStorage(&persistent.ClientOpts{
			Type:             "mongo-go",
			ConnectionString: dbAddr,
		})
		if err != nil {
			panic("Unable to connect to mongo: " + err.Error())
		}
	}
}

// Verifies: SW-REQ-018
func (c *Conn) CleanDb() {
	err := c.Store.DropDatabase(context.Background())
	if err != nil {
		panic(err)
	}
}

// Verifies: SW-REQ-018
func (c *Conn) CleanCollection() {
	err := c.Store.Drop(context.Background(), c)
	if err != nil {
		panic(err)
	}
}

// Verifies: SW-REQ-018
func (c *Conn) CleanIndexes() {
	err := c.Store.CleanIndexes(context.Background(), c)
	if err != nil {
		panic(err)
	}
}

type Doc struct {
	ID  model.ObjectID `bson:"_id"`
	Foo string         `bson:"foo"`
}

// Verifies: SW-REQ-018
func (d Doc) GetObjectID() model.ObjectID {
	return d.ID
}

// SetObjectID is a dummy function to satisfy the interface
// Verifies: SW-REQ-018
func (d *Doc) SetObjectID(id model.ObjectID) {
	d.ID = id
}

// Verifies: SW-REQ-018
func (d Doc) TableName() string {
	return colName
}

// Verifies: SW-REQ-018
func (c *Conn) InsertDoc() {
	doc := Doc{
		Foo: "bar",
	}
	doc.SetObjectID(model.NewObjectID())
	err := c.Store.Insert(context.Background(), &doc)
	if err != nil {
		panic(err)
	}
}

// Verifies: SW-REQ-018
func (c *Conn) GetCollectionStats() (colStats model.DBM) {
	var err error
	colStats, err = c.Store.DBTableStats(context.Background(), c)
	if err != nil {
		panic(err)
	}
	return colStats
}

// Verifies: SW-REQ-018
func (c *Conn) GetIndexes() ([]model.Index, error) {
	return c.Store.GetIndexes(context.Background(), c)
}

// Verifies: SW-REQ-018
func defaultConf() MongoConf {
	conf := MongoConf{
		CollectionName:          colName,
		MaxInsertBatchSizeBytes: 10 * MiB,
		MaxDocumentSizeBytes:    10 * MiB,
	}

	conf.MongoURL = dbAddr
	conf.MongoSSLInsecureSkipVerify = true

	if os.Getenv("MONGO_DRIVER") == "mongo-go" {
		conf.MongoDriverType = persistent.OfficialMongo
	} else {
		conf.MongoDriverType = persistent.Mgo
	}

	return conf
}

// Verifies: SW-REQ-018
func defaultSelectiveConf() MongoSelectiveConf {
	conf := MongoSelectiveConf{
		MaxInsertBatchSizeBytes: 10 * MiB,
		MaxDocumentSizeBytes:    10 * MiB,
	}

	conf.MongoURL = dbAddr
	conf.MongoSSLInsecureSkipVerify = true

	if os.Getenv("MONGO_DRIVER") == "mongo-go" {
		conf.MongoDriverType = persistent.OfficialMongo
	} else {
		conf.MongoDriverType = persistent.Mgo
	}

	return conf
}
