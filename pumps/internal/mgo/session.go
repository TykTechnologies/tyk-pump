package mgo

import (
	mgo "gopkg.in/mgo.v2"
)

type SessionManager interface {
	Clone() SessionManager
	Close()
	Copy() SessionManager
	DB(name string) DatabaseManager
	Run(cmd interface{}, result interface{}) error
}

type Session struct {
	session *mgo.Session
}

func NewSessionManager(s *mgo.Session) SessionManager {
	return &Session{
		session: s,
	}
}

func (s *Session) BuildInfo() (info mgo.BuildInfo, err error) {
	return s.session.BuildInfo()
}

func (s *Session) Close() {
	s.session.Close()
}

func (s *Session) Clone() SessionManager {
	return &Session{
		session: s.session.Clone(),
	}
}

func (s *Session) Copy() SessionManager {
	return &Session{
		session: s.session.Copy(),
	}
}

func (s *Session) DB(name string) DatabaseManager {
	d := &Database{
		db: s.session.DB(name),
	}
	return d
}

func (s *Session) Run(cmd interface{}, result interface{}) error {
	return s.session.Run(cmd, result)
}
