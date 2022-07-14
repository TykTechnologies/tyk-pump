// Test Helper for Mongo

package mongo

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const dbAddr = "127.0.0.1:27017"
const colName = "test_collection"

type Conn struct {
	Session *mgo.Session
}

func (c *Conn) ConnectDb() {
	if c.Session == nil {
		var err error
		c.Session, err = mgo.Dial(dbAddr)
		if err != nil {
			panic("Unable to connect to mongo")
		}
	}
}

func (c *Conn) CleanDb() {
	sess := c.Session.Copy()
	defer sess.Close()

	if err := sess.DB("").DropDatabase(); err != nil {
		panic(err)
	}
}

func (c *Conn) CleanCollection() {
	sess := c.Session.Copy()
	defer sess.Close()

	if err := sess.DB("").C(colName).DropCollection(); err != nil {
		panic(err)
	}
}

func (c *Conn) CleanIndexes() {
	sess := c.Session.Copy()
	defer sess.Close()

	indexes, err := sess.DB("").C(colName).Indexes()
	if err != nil {
		panic(err)
	}
	for _, index := range indexes {
		sess.DB("").C(colName).DropIndexName(index.Name)
	}

}

func (c *Conn) InsertDoc() {
	sess := c.Session.Copy()
	defer sess.Close()

	if err := sess.DB("").C(colName).Insert(bson.M{"foo": "bar"}); err != nil {
		panic(err)
	}
}

func (c *Conn) GetCollectionStats() (colStats bson.M) {
	sess := c.Session.Copy()
	defer sess.Close()

	data := bson.D{{Name: "collStats", Value: colName}}

	if err := sess.DB("").Run(data, &colStats); err != nil {
		panic(err)
	}

	return colStats
}

func (c *Conn) GetIndexes() ([]mgo.Index, error) {
	sess := c.Session.Copy()
	defer sess.Close()

	return sess.DB("").C(colName).Indexes()
}

func defaultConf() MongoConf {
	conf := MongoConf{
		CollectionName:          colName,
		MaxInsertBatchSizeBytes: 10 * MiB,
		MaxDocumentSizeBytes:    10 * MiB,
	}

	conf.MongoURL = dbAddr
	conf.MongoSSLInsecureSkipVerify = true

	return conf
}
