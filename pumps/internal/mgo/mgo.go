package mgo

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	mgo "gopkg.in/mgo.v2"
)

type Dialer interface {
	Dial(string) (SessionManager, error)
	DialWithInfo(*DialInfo) (SessionManager, error)
	DialWithTimeout(string, time.Duration) (SessionManager, error)
}

type DialInfo struct {
	SSLPEMKeyFile string
	SSLCAFile     string

	Addrs                    []string
	Direct                   bool
	Timeout                  time.Duration
	FailFast                 bool
	Database                 string
	ReplicaSetName           string
	Source                   string
	Service                  string
	ServiceHost              string
	Mechanism                string
	Username                 string
	Password                 string
	PoolLimit                int
	DialServer               func(addr *mgo.ServerAddr) (net.Conn, error)
	InsecureSkipVerify       bool
	SSLAllowInvalidHostnames bool
}

type dialer struct{}

func NewDialInfo(src *mgo.DialInfo) *DialInfo {
	return &DialInfo{
		Addrs:          src.Addrs,
		Direct:         src.Direct,
		Timeout:        src.Timeout,
		FailFast:       src.FailFast,
		Database:       src.Database,
		ReplicaSetName: src.ReplicaSetName,
		Source:         src.Source,
		Service:        src.Service,
		ServiceHost:    src.ServiceHost,
		Mechanism:      src.Mechanism,
		Username:       src.Username,
		Password:       src.Password,
		PoolLimit:      src.PoolLimit,
		DialServer:     src.DialServer,
	}
}

func ParseURL(url string) (*DialInfo, error) {
	di, err := mgo.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return NewDialInfo(di), nil
}

func NewDialer() Dialer {
	return new(dialer)
}

func (d *dialer) Dial(url string) (SessionManager, error) {
	s, err := mgo.Dial(url)
	se := &Session{
		session: s,
	}
	return se, err
}

func (d *dialer) DialWithInfo(info *DialInfo) (SessionManager, error) {
	mgoInfo := &mgo.DialInfo{
		Addrs:          info.Addrs,
		Direct:         info.Direct,
		Timeout:        info.Timeout,
		FailFast:       info.FailFast,
		Database:       info.Database,
		ReplicaSetName: info.ReplicaSetName,
		Source:         info.Source,
		Service:        info.Service,
		ServiceHost:    info.ServiceHost,
		Mechanism:      info.Mechanism,
		Username:       info.Username,
		Password:       info.Password,
		PoolLimit:      info.PoolLimit,
		DialServer:     info.DialServer,
	}

	if info.SSLCAFile != "" || info.SSLPEMKeyFile != "" {
		tlsConfig := &tls.Config{}

		if info.SSLCAFile != "" {
			if _, err := os.Stat(info.SSLCAFile); os.IsNotExist(err) {
				return nil, err
			}

			roots := x509.NewCertPool()
			var ca []byte
			var err error

			if ca, err = ioutil.ReadFile(info.SSLCAFile); err != nil {
				return nil, fmt.Errorf("invalid pem file: %s", err.Error())
			}
			roots.AppendCertsFromPEM(ca)
			tlsConfig.RootCAs = roots

		}

		if info.SSLPEMKeyFile != "" {
			cert, err := tls.LoadX509KeyPair(info.SSLPEMKeyFile, info.SSLPEMKeyFile)
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		if info.InsecureSkipVerify {
			tlsConfig.InsecureSkipVerify = true
		}

		if info.SSLAllowInvalidHostnames {
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

		mgoInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
			return conn, err
		}

		mgoInfo.Source = "$external"
		mgoInfo.Mechanism = "MONGODB-X509"
	}

	s, err := mgo.DialWithInfo(mgoInfo)

	se := &Session{
		session: s,
	}
	return se, err
}

func (d *dialer) DialWithTimeout(url string, timeout time.Duration) (SessionManager, error) {
	s, err := mgo.DialWithTimeout(url, timeout)
	se := &Session{
		session: s,
	}
	return se, err
}
