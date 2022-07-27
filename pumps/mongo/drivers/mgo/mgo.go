package mgo

import "gopkg.in/mgo.v2"

type Session interface {
	DB(string) *mgo.Database
	Copy() Session
	Clone() Session
	Close()
	Run(cmd interface{}, result interface{}) error
}

type Collection interface {
}

type Database interface {
}

type MgoDatabase struct {
	*mgo.Database
}

type MgoDriver struct {
	*mgo.Session
}

func (d *MgoDriver) DB(dbname string) *mgo.Database {
	return d.DB(dbname)
}

func (d *MgoDriver) Copy() Session {
	return d.Copy()
}

func (d *MgoDriver) Clone() Session {
	return d.Clone()
}

func (d *MgoDriver) Close() {
	d.Close()
}

func (d *MgoDriver) Run(cmd interface{}, result interface{}) error {
	return d.Run(cmd, result)
}
