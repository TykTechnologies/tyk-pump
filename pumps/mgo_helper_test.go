// Test Helper for Mongo

package pumps

import (
	"context"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/dbm"
	"github.com/TykTechnologies/storage/persistent/id"
	"github.com/TykTechnologies/storage/persistent/index"
)

const dbAddr = "127.0.0.1:27017"
const colName = "test_collection"

type Conn struct {
	Store persistent.PersistentStorage
}

func (c *Conn) TableName() string {
	return colName
}

// SetObjectID is a dummy function to satisfy the interface
func (*Conn) GetObjectID() id.ObjectId {
	return ""
}

// SetObjectID is a dummy function to satisfy the interface
func (*Conn) SetObjectID(id.ObjectId) {
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
	ID  id.ObjectId `bson:"_id"`
	Foo string      `bson:"foo"`
}

func (d Doc) GetObjectID() id.ObjectId {
	return d.ID
}

func (d *Doc) SetObjectID(id id.ObjectId) {
	d.ID = id
}

func (d Doc) TableName() string {
	return colName
}

func (c *Conn) InsertDoc() {
	doc := Doc{
		Foo: "bar",
	}
	doc.SetObjectID(id.NewObjectID())
	err := c.Store.Insert(context.Background(), &doc)
	if err != nil {
		panic(err)
	}
}

func (c *Conn) GetCollectionStats() (colStats dbm.DBM) {
	var err error
	colStats, err = c.Store.DBTableStats(context.Background(), c)
	if err != nil {
		panic(err)
	}
	return colStats
}

func (c *Conn) GetIndexes() ([]index.Index, error) {
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
	conf.BaseMongoConf.MongoDriverType = "mgo"

	return conf
}

func defaultSelectiveConf() MongoSelectiveConf {
	conf := MongoSelectiveConf{
		MaxInsertBatchSizeBytes: 10 * MiB,
		MaxDocumentSizeBytes:    10 * MiB,
	}

	conf.MongoURL = dbAddr
	conf.MongoSSLInsecureSkipVerify = true

	return conf
}
