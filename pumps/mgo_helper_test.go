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

func (c *Conn) TableName() string {
	return colName
}

// SetObjectID is a dummy function to satisfy the interface
func (*Conn) GetObjectID() model.ObjectID {
	return ""
}

// SetObjectID is a dummy function to satisfy the interface
func (*Conn) SetObjectID(model.ObjectID) {
	// empty
}

func (c *Conn) ConnectDb() {
	if c.Store == nil {
		var err error
		c.Store, err = persistent.NewPersistentStorage(&persistent.ClientOpts{
			Type:             "mgo",
			ConnectionString: dbAddr,
		})
		if err != nil {
			panic("Unable to connect to mongo: " + err.Error())
		}
	}
}

func (c *Conn) CleanDb() {
	err := c.Store.DropDatabase(context.Background())
	if err != nil {
		panic(err)
	}
}

func (c *Conn) CleanCollection() {
	err := c.Store.Drop(context.Background(), c)
	if err != nil {
		panic(err)
	}
}

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

func (d Doc) GetObjectID() model.ObjectID {
	return d.ID
}

func (d *Doc) SetObjectID(id model.ObjectID) {
	d.ID = id
}

func (d Doc) TableName() string {
	return colName
}

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

func (c *Conn) GetCollectionStats() (colStats model.DBM) {
	var err error
	colStats, err = c.Store.DBTableStats(context.Background(), c)
	if err != nil {
		panic(err)
	}
	return colStats
}

func (c *Conn) GetIndexes() ([]model.Index, error) {
	return c.Store.GetIndexes(context.Background(), c)
}

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
