package pumps

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodingRequest(t *testing.T) {
	pump := &CommonPumpConfig{}
	pump.SetDecodingRequest(true)
	actualValue := pump.GetDecodedRequest()
	assert.Equal(t, actualValue, pump.decodeRequestBase64)
	assert.True(t, actualValue)
}

func TestSetDecodingResponse(t *testing.T) {
	pump := &CommonPumpConfig{}
	pump.SetDecodingResponse(true)
	actualValue := pump.GetDecodedResponse()
	assert.Equal(t, actualValue, pump.decodeResponseBase64)
	assert.True(t, actualValue)
}

func TestNewTLSConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tls_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	caFile, certFile, keyFile := generateTestCerts(t, tempDir)

	logger := logrus.NewEntry(logrus.New())

	testCases := []struct {
		name      string
		cfg       TLSConfig
		expectErr bool
		validate  func(t *testing.T, cfg *tls.Config)
	}{
		{
			name: "valid config with all files",
			cfg: TLSConfig{
				CertFile:           certFile,
				KeyFile:            keyFile,
				CAFile:             caFile,
				ServerName:         "test.something.com",
				InsecureSkipVerify: false,
			},
			expectErr: false,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.Len(t, cfg.Certificates, 1)
				assert.NotNil(t, cfg.RootCAs)
				assert.Equal(t, "test.something.com", cfg.ServerName)
				assert.False(t, cfg.InsecureSkipVerify)
			},
		},
		{
			name: "client cert and key only",
			cfg: TLSConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			},
			expectErr: false,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.Len(t, cfg.Certificates, 1)
				assert.Nil(t, cfg.RootCAs)
			},
		},
		{
			name: "CA cert only",
			cfg: TLSConfig{
				CAFile: caFile,
			},
			expectErr: false,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.RootCAs)
				assert.Empty(t, cfg.Certificates)
			},
		},
		{
			name: "insecure skip verify with CA cert warning",
			cfg: TLSConfig{
				CAFile:             caFile,
				InsecureSkipVerify: true,
			},
			expectErr: false,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.True(t, cfg.InsecureSkipVerify)
				assert.NotNil(t, cfg.RootCAs)
			},
		},
		{
			name: "cert without key - should fail",
			cfg: TLSConfig{
				CertFile: certFile,
			},
			expectErr: true,
		},
		{
			name: "key without cert - should fail",
			cfg: TLSConfig{
				KeyFile: keyFile,
			},
			expectErr: true,
		},
		{
			name: "invalid cert file",
			cfg: TLSConfig{
				CertFile: "nonexistent_cert.pem",
				KeyFile:  keyFile,
			},
			expectErr: true,
		},
		{
			name: "invalid key file",
			cfg: TLSConfig{
				CertFile: certFile,
				KeyFile:  "nonexistent_key.pem",
			},
			expectErr: true,
		},
		{
			name: "invalid CA cert file",
			cfg: TLSConfig{
				CAFile: "nonexistent_ca.pem",
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tlsConfig, err := NewTLSConfig(tc.cfg, logger)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, tlsConfig)
			} else {
				assert.NoError(t, err)
				if tc.validate != nil {
					tc.validate(t, tlsConfig)
				}
			}
		})
	}
}

func generateTestCerts(t *testing.T, tempDir string) (caFile, certFile, keyFile string) {
	caPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"testorg"},
			Country:      []string{"UK"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, caPrivateKey.Public(), caPrivateKey)
	require.NoError(t, err)

	clientPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	clientTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, &clientTemplate, clientPrivateKey.Public(), clientPrivateKey)
	require.NoError(t, err)

	caFile = filepath.Join(tempDir, "ca.pem")
	caOut, err := os.Create(caFile)
	require.NoError(t, err)
	defer caOut.Close()

	err = pem.Encode(caOut, &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	require.NoError(t, err)

	certFile = filepath.Join(tempDir, "client-cert.pem")
	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	require.NoError(t, err)

	keyFile = filepath.Join(tempDir, "client-key.pem")
	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)
	defer keyOut.Close()

	clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientPrivateKey)
	require.NoError(t, err)

	err = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER})
	require.NoError(t, err)

	return caFile, certFile, keyFile
}
