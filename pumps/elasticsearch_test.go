package pumps

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"
	"time"
)

func Test_getTLSConfig(t *testing.T) {
	certFile, keyFile, err := createSelfSignedCertificate()
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(certFile.Name())
	defer os.Remove(keyFile.Name())

	tests := []struct {
		args    *ElasticsearchConf
		want    *tls.Config
		name    string
		wantErr bool
	}{
		{
			name: "SSLCertFile, SSLKeyfile are set and InsecureSkipVerify = true",
			args: &ElasticsearchConf{
				SSLCertFile:           certFile.Name(),
				SSLKeyFile:            keyFile.Name(),
				SSLInsecureSkipVerify: true,
			},
			// #nosec G402
			want: &tls.Config{
				Certificates:       getCertificate(certFile.Name(), keyFile.Name()),
				InsecureSkipVerify: true,
			},
			wantErr: false,
		},
		{
			// No error expected. It should fail when sending data to ES, because we're using a self-signed certificate
			// and InsecureSkipVerify is false
			name: "SSLCertFile, SSLKeyfile are set and InsecureSkipVerify = false",
			args: &ElasticsearchConf{
				SSLCertFile:           certFile.Name(),
				SSLKeyFile:            keyFile.Name(),
				SSLInsecureSkipVerify: true,
			},
			// #nosec G402
			want: &tls.Config{
				Certificates:       getCertificate(certFile.Name(), keyFile.Name()),
				InsecureSkipVerify: true,
			},
			wantErr: false,
		},
		{
			name: "SSLKeyFile not set -> error expected because CertFile is set",
			args: &ElasticsearchConf{
				SSLCertFile:           certFile.Name(),
				SSLKeyFile:            "",
				SSLInsecureSkipVerify: true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "CertFile not set -> error expected because KeyFile is set",
			args: &ElasticsearchConf{
				SSLCertFile:           "",
				SSLKeyFile:            keyFile.Name(),
				SSLInsecureSkipVerify: true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "CertFile and KeyFile not set -> no error expected. It must return a tls.Config with InsecureSkipVerify = true",
			args: &ElasticsearchConf{
				SSLCertFile:           "",
				SSLKeyFile:            "",
				SSLInsecureSkipVerify: true,
			},
			// #nosec G402
			want: &tls.Config{
				InsecureSkipVerify: true,
			},
			wantErr: false,
		},
		{
			name: "Invalid CertFile -> error expected.",
			args: &ElasticsearchConf{
				SSLCertFile:           "invalid.cert",
				SSLKeyFile:            "invalid.key",
				SSLInsecureSkipVerify: true,
			},
			// #nosec G402
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pump := ElasticsearchPump{
				esConf: tt.args,
			}
			got, err := pump.GetTLSConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTLSConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTLSConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createSelfSignedCertificate() (*os.File, *os.File, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Testing Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 180),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		return nil, nil, err
	}
	out := &bytes.Buffer{}
	err = pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return nil, nil, err
	}
	certFile, err := os.CreateTemp("", "test.*.crt")
	if err != nil {
		return nil, nil, err
	}
	_, err = certFile.Write(out.Bytes())
	if err != nil {
		return nil, nil, err
	}

	out.Reset()
	block, err := pemBlockForKey(priv)
	if err != nil {
		return nil, nil, err
	}
	err = pem.Encode(out, block)
	if err != nil {
		return nil, nil, err
	}
	keyFile, err := os.CreateTemp("", "test.*.key")
	if err != nil {
		return nil, nil, err
	}
	_, err = keyFile.Write(out.Bytes())
	if err != nil {
		return nil, nil, err
	}
	return certFile, keyFile, nil
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) (*pem.Block, error) {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}, nil
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, fmt.Errorf("Unable to marshal ECDSA private key: %v", err)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}, nil
	default:
		return nil, fmt.Errorf("unknown private key type")
	}
}

func getCertificate(certFile, keyFile string) []tls.Certificate {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}

	return []tls.Certificate{cert}
}
