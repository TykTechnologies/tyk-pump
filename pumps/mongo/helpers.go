package mongo

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
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

func LoadCertficateAndKeyFromFile(path string) (*tls.Certificate, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cert tls.Certificate
	for {
		block, rest := pem.Decode(raw)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, block.Bytes)
		} else {
			cert.PrivateKey, err = parsePrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("Failure reading private key from \"%s\": %s", path, err)
			}
		}
		raw = rest
	}

	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("No certificate found in \"%s\"", path)
	} else if cert.PrivateKey == nil {
		return nil, fmt.Errorf("No private key found in \"%s\"", path)
	}

	return &cert, nil
}

func parsePrivateKey(der []byte) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("Found unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("Failed to parse private key")
}

func GetMongoType(session mgo.SessionManager) MongoType {
	// Querying for the features which 100% not supported by AWS DocumentDB
	var result struct {
		Code int `bson:"code"`
	}
	session.Run("features", &result)

	switch result.Code {
	case AWSDBError:
		return AWSDocumentDB
	case CosmosDBError:
		return CosmosDB
	default:
		return StandardMongo
	}
}

func NewSession(conf BaseConfig, timeout int) (mgo.SessionManager, error) {
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
	dialer := mgo.NewDialer()
	mgoSession, err := dialer.DialWithInfo(dialInfo)

	return mgoSession, err
}
