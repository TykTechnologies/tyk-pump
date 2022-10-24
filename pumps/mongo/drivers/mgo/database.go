package mgo

import mgo "gopkg.in/mgo.v2"

type DatabaseManager interface {
	// AddUser(username, password string, readOnly bool) error
	C(name string) CollectionManager
	CollectionNames() (names []string, err error)
	DropDatabase() error
	Run(cmd interface{}, result interface{}) error
}

type Database struct {
	db *mgo.Database
}

func NewDatabaseManager(d *mgo.Database) DatabaseManager {
	return &Database{
		db: d,
	}
}

func (d *Database) C(name string) CollectionManager {
	c := &Collection{
		collection: d.db.C(name),
	}
	return c
}

func (d *Database) CollectionNames() ([]string, error) {
	return d.db.CollectionNames()
}

func (d *Database) DropDatabase() error {
	return d.db.DropDatabase()
}

func (d *Database) Run(cmd interface{}, result interface{}) error {
	return d.db.Run(cmd, result)
}
