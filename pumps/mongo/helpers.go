package mongo

import (
	"time"

	"github.com/TykTechnologies/tyk-pump/pumps/internal/mgo"
)

type MongoType int

const (
	StandardMongo MongoType = iota
	AWSDocumentDB
	CosmosDB
)

const (
	AWSDBError    = 303
	CosmosDBError = 115
)

func GetMongoType(session mgo.SessionManager) MongoType {
	// Querying for the features which 100% not supported by AWS DocumentDB
	var result struct {
		Code int `bson:"code"`
	}
	err := session.Run("features", &result)
	if err != nil {
		return StandardMongo
	}

	switch result.Code {
	case AWSDBError:
		return AWSDocumentDB
	case CosmosDBError:
		return CosmosDB
	default:
		return StandardMongo
	}
}

func NewSession(dialer mgo.Dialer, conf BaseConfig, timeout int) (mgo.SessionManager, error) {
	dialInfo, err := buildDialInfo(conf, timeout)
	if err != nil {
		return nil, err
	}

	mgoSession, err := dialer.DialWithInfo(dialInfo)

	return mgoSession, err
}

func buildDialInfo(conf BaseConfig, timeout int) (*mgo.DialInfo, error) {
	dialInfo, err := mgo.ParseURL(conf.MongoURL)
	if err != nil {
		return nil, err
	}

	if conf.MongoUseSSL {
		if conf.MongoSSLInsecureSkipVerify {
			dialInfo.InsecureSkipVerify = true
		}

		if conf.MongoSSLAllowInvalidHostnames {
			dialInfo.SSLAllowInvalidHostnames = true
		}

		if conf.MongoSSLCAFile != "" {
			dialInfo.SSLCAFile = conf.MongoSSLCAFile
		}
		if conf.MongoSSLPEMKeyfile != "" {
			dialInfo.SSLPEMKeyFile = conf.MongoSSLPEMKeyfile
		}
	}
	if timeout > 0 {
		dialInfo.Timeout = time.Second * time.Duration(timeout)
	}
	return dialInfo, err
}
