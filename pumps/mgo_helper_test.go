// Test Helper for Mongo

package pumps

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

func defaultConf() MongoConf {
	return MongoConf{
		CollectionName:             colName,
		MongoURL:                   dbAddr,
		MongoSSLInsecureSkipVerify: true,
		MaxInsertBatchSizeBytes:    10 * MiB,
		MaxDocumentSizeBytes:       10 * MiB,
	}
}
