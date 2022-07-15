package mongo

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"gopkg.in/mgo.v2"
)

type MongoType int

const (
	StandardMongo MongoType = iota
	AWSDocumentDB
)

func loadCertficateAndKeyFromFile(path string) (*tls.Certificate, error) {
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

func GetMongoType(session *mgo.Session) MongoType {
	// Querying for the features which 100% not supported by AWS DocumentDB
	var result struct {
		Code int `bson:"code"`
	}
	err := session.Run("features", &result)
	if err != nil {
		return StandardMongo
	}

	if result.Code == 303 {
		return AWSDocumentDB
	} else {
		return StandardMongo
	}
}

func DialInfo(conf BaseConfig) (dialInfo *mgo.DialInfo, err error) {
	if dialInfo, err = mgo.ParseURL(conf.MongoURL); err != nil {
		return dialInfo, err
	}

	if conf.MongoUseSSL {
		dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			tlsConfig := &tls.Config{}
			if conf.MongoSSLInsecureSkipVerify {
				tlsConfig.InsecureSkipVerify = true
			}

			if conf.MongoSSLCAFile != "" {
				caCert, err := ioutil.ReadFile(conf.MongoSSLCAFile)
				if err != nil {
					return nil, errors.New("Can't load mongo CA certificates: " + err.Error())
				}
				caCertPool := x509.NewCertPool()
				caCertPool.AppendCertsFromPEM(caCert)
				tlsConfig.RootCAs = caCertPool
			}

			if conf.MongoSSLAllowInvalidHostnames {
				tlsConfig.InsecureSkipVerify = true
				tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
					// Code copy/pasted and adapted from
					// https://github.com/golang/go/blob/81555cb4f3521b53f9de4ce15f64b77cc9df61b9/src/crypto/tls/handshake_client.go#L327-L344, but adapted to skip the hostname verification.
					// See https://github.com/golang/go/issues/21971#issuecomment-412836078.

					// If this is the first handshake on a connection, process and
					// (optionally) verify the server's certificates.
					certs := make([]*x509.Certificate, len(rawCerts))
					for i, asn1Data := range rawCerts {
						cert, err := x509.ParseCertificate(asn1Data)
						if err != nil {
							return err
						}
						certs[i] = cert
					}

					opts := x509.VerifyOptions{
						Roots:         tlsConfig.RootCAs,
						CurrentTime:   time.Now(),
						DNSName:       "", // <- skip hostname verification
						Intermediates: x509.NewCertPool(),
					}

					for i, cert := range certs {
						if i == 0 {
							continue
						}
						opts.Intermediates.AddCert(cert)
					}
					_, err := certs[0].Verify(opts)

					return err
				}
			}

			if conf.MongoSSLPEMKeyfile != "" {
				cert, err := loadCertficateAndKeyFromFile(conf.MongoSSLPEMKeyfile)
				if err != nil {
					return nil, errors.New("Can't load mongo client certificate: " + err.Error())
				}

				tlsConfig.Certificates = []tls.Certificate{*cert}
			}

			return tls.Dial("tcp", addr.String(), tlsConfig)
		}
	}

	return dialInfo, err
}
