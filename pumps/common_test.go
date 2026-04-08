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

	"github.com/kelseyhightower/envconfig"
	"github.com/mitchellh/mapstructure"
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

// TestPumpEnvVarOverride tests the generic behavior of environment variable overrides
// for pump configurations. This test validates that the processPumpEnvVars mechanism
// (which uses mapstructure.Decode + envconfig.Process) correctly overrides configuration
// values with environment variables. This behavior is common to all pumps.
func TestPumpEnvVarOverride(t *testing.T) {
	type TestPumpConfig struct {
		Topic       string   `json:"topic" mapstructure:"topic"`
		SSLCAFile   string   `json:"ssl_ca_file" mapstructure:"ssl_ca_file"`
		SSLCertFile string   `json:"ssl_cert_file" mapstructure:"ssl_cert_file"`
		SSLKeyFile  string   `json:"ssl_key_file" mapstructure:"ssl_key_file"`
		Broker      []string `json:"broker" mapstructure:"broker"`
		UseSSL      bool     `json:"use_ssl" mapstructure:"use_ssl"`
		Timeout     int      `json:"timeout" mapstructure:"timeout"`
	}

	t.Run("environment variable overrides config file setting", func(t *testing.T) {
		os.Setenv("TEST_PUMP_SSLCAFILE", "env_override_ca.pem")
		defer os.Unsetenv("TEST_PUMP_SSLCAFILE")

		config := map[string]any{
			"broker":      []string{"localhost:9092"},
			"topic":       "test-topic",
			"ssl_ca_file": "config_file_ca.pem",
		}

		testConf := &TestPumpConfig{}
		err := mapstructure.Decode(config, testConf)
		assert.NoError(t, err)

		// Simulate what processPumpEnvVars does
		err = envconfig.Process("TEST_PUMP", testConf)
		assert.NoError(t, err)

		// Environment variable should override config file value
		assert.Equal(t, "env_override_ca.pem", testConf.SSLCAFile)
		assert.Equal(t, []string{"localhost:9092"}, testConf.Broker)
		assert.Equal(t, "test-topic", testConf.Topic)
	})

	t.Run("loads from environment variable when no config file setting", func(t *testing.T) {
		os.Setenv("TEST_PUMP_SSLCAFILE", "env_only_ca.pem")
		defer os.Unsetenv("TEST_PUMP_SSLCAFILE")

		config := map[string]any{
			"broker": []string{"localhost:9092"},
			"topic":  "test-topic",
		}

		testConf := &TestPumpConfig{}
		err := mapstructure.Decode(config, testConf)
		assert.NoError(t, err)

		err = envconfig.Process("TEST_PUMP", testConf)
		assert.NoError(t, err)

		assert.Equal(t, "env_only_ca.pem", testConf.SSLCAFile)
	})

	t.Run("loads from config file when no environment variable", func(t *testing.T) {
		config := map[string]any{
			"broker":      []string{"localhost:9092"},
			"topic":       "test-topic",
			"ssl_ca_file": "config_only_ca.pem",
		}

		testConf := &TestPumpConfig{}
		err := mapstructure.Decode(config, testConf)
		assert.NoError(t, err)

		err = envconfig.Process("TEST_PUMP", testConf)
		assert.NoError(t, err)

		assert.Equal(t, "config_only_ca.pem", testConf.SSLCAFile)
	})

	t.Run("environment variable overrides multiple SSL config fields", func(t *testing.T) {
		os.Setenv("TEST_PUMP_SSLCAFILE", "env_ca.pem")
		os.Setenv("TEST_PUMP_SSLCERTFILE", "env_cert.pem")
		os.Setenv("TEST_PUMP_SSLKEYFILE", "env_key.pem")
		os.Setenv("TEST_PUMP_USESSL", "true")
		defer func() {
			os.Unsetenv("TEST_PUMP_SSLCAFILE")
			os.Unsetenv("TEST_PUMP_SSLCERTFILE")
			os.Unsetenv("TEST_PUMP_SSLKEYFILE")
			os.Unsetenv("TEST_PUMP_USESSL")
		}()

		config := map[string]any{
			"broker":        []string{"localhost:9092"},
			"topic":         "test-topic",
			"ssl_ca_file":   "config_ca.pem",
			"ssl_cert_file": "config_cert.pem",
			"ssl_key_file":  "config_key.pem",
			"use_ssl":       false,
		}

		testConf := &TestPumpConfig{}
		err := mapstructure.Decode(config, testConf)
		assert.NoError(t, err)

		err = envconfig.Process("TEST_PUMP", testConf)
		assert.NoError(t, err)

		// All SSL fields should be overridden by environment variables
		assert.Equal(t, "env_ca.pem", testConf.SSLCAFile)
		assert.Equal(t, "env_cert.pem", testConf.SSLCertFile)
		assert.Equal(t, "env_key.pem", testConf.SSLKeyFile)
		assert.True(t, testConf.UseSSL)
	})

	t.Run("environment variable does not affect other config fields", func(t *testing.T) {
		os.Setenv("TEST_PUMP_SSLCAFILE", "env_ca.pem")
		defer os.Unsetenv("TEST_PUMP_SSLCAFILE")

		config := map[string]any{
			"broker":      []string{"localhost:9092", "localhost:9093"},
			"topic":       "analytics-topic",
			"ssl_ca_file": "config_ca.pem",
			"timeout":     30,
		}

		testConf := &TestPumpConfig{}
		err := mapstructure.Decode(config, testConf)
		assert.NoError(t, err)

		err = envconfig.Process("TEST_PUMP", testConf)
		assert.NoError(t, err)

		// Only SSLCAFile should be overridden
		assert.Equal(t, "env_ca.pem", testConf.SSLCAFile)
		// Other fields should remain unchanged
		assert.Equal(t, []string{"localhost:9092", "localhost:9093"}, testConf.Broker)
		assert.Equal(t, "analytics-topic", testConf.Topic)
		assert.Equal(t, 30, testConf.Timeout)
	})
}

// TestNewTLSConfig tests the TLS configuration creation with various settings.
//
// Backward Compatibility Testing:
// Several test cases verify that partial or misconfigured mTLS settings (e.g., cert without key)
// log warnings but still succeed in creating a TLS config. This behavior is intentional to maintain
// backward compatibility with existing pump deployments.
// These tests are marked with "(backward compatible)" in their names to clearly indicate
// this is expected behavior, not a bug.
func TestNewTLSConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tls_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	caFile, certFile, keyFile, invalidCAFile := generateTestCerts(t, tempDir)

	logger := logrus.NewEntry(logrus.New())

	testCases := []struct {
		logger    *logrus.Entry
		validate  func(t *testing.T, cfg *tls.Config)
		name      string
		cfg       TLSConfig
		expectErr bool
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
			logger: logger,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.Len(t, cfg.Certificates, 1)
				assert.NotNil(t, cfg.RootCAs)
				assert.Equal(t, "test.something.com", cfg.ServerName)
				assert.False(t, cfg.InsecureSkipVerify)
			},
			expectErr: false,
		},
		{
			name:   "valid config with defaults only",
			cfg:    TLSConfig{},
			logger: logger,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.False(t, cfg.InsecureSkipVerify)
				assert.Empty(t, cfg.Certificates)
				assert.Nil(t, cfg.RootCAs)
			},
			expectErr: false,
		},
		{
			name: "client cert and key only",
			cfg: TLSConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			},
			logger: logger,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.Len(t, cfg.Certificates, 1)
				assert.Nil(t, cfg.RootCAs)
			},
			expectErr: false,
		},
		{
			name: "CA cert only",
			cfg: TLSConfig{
				CAFile: caFile,
			},
			logger: logger,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.RootCAs)
				assert.Empty(t, cfg.Certificates)
			},
			expectErr: false,
		},
		{
			name: "insecure skip verify with CA cert - CA loaded but verification disabled",
			cfg: TLSConfig{
				CAFile:             caFile,
				InsecureSkipVerify: true,
			},
			logger: logger,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.True(t, cfg.InsecureSkipVerify)
				assert.NotNil(t, cfg.RootCAs)
			},
			expectErr: false,
		},
		{
			name: "cert without key - logs warning and creates TLS config without client cert (backward compatible)",
			cfg: TLSConfig{
				CertFile: certFile,
			},
			logger: logger,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.Empty(t, cfg.Certificates)
			},
			expectErr: false,
		},
		{
			name: "key without cert - logs warning and creates TLS config without client cert (backward compatible)",
			cfg: TLSConfig{
				KeyFile: keyFile,
			},
			logger: logger,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.Empty(t, cfg.Certificates)
			},
			expectErr: false,
		},
		{
			name: "CA cert with key only - CA loaded, client cert skipped (backward compatible)",
			cfg: TLSConfig{
				CAFile:  caFile,
				KeyFile: keyFile,
			},
			logger: logger,
			validate: func(t *testing.T, cfg *tls.Config) {
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.RootCAs)
				assert.Empty(t, cfg.Certificates)
			},
			expectErr: false,
		},
		{
			name: "invalid cert file",
			cfg: TLSConfig{
				CertFile: "nonexistent_cert.pem",
				KeyFile:  keyFile,
			},
			logger:    logger,
			expectErr: true,
		},
		{
			name: "invalid key file",
			cfg: TLSConfig{
				CertFile: certFile,
				KeyFile:  "nonexistent_key.pem",
			},
			logger:    logger,
			expectErr: true,
		},
		{
			name: "invalid CA cert file - file not found",
			cfg: TLSConfig{
				CAFile: "nonexistent_ca.pem",
			},
			logger:    logger,
			expectErr: true,
		},
		{
			name: "invalid CA cert file - malformed PEM data",
			cfg: TLSConfig{
				CAFile: invalidCAFile,
			},
			logger:    logger,
			expectErr: true,
		},
		{
			name:      "logger must be provided",
			cfg:       TLSConfig{},
			logger:    nil,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tlsConfig, err := NewTLSConfig(tc.cfg, tc.logger)

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

func generateTestCerts(t *testing.T, tempDir string) (string, string, string, string) {
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

	caFile := filepath.Join(tempDir, "ca.pem")
	caOut, err := os.Create(caFile)
	require.NoError(t, err)
	defer caOut.Close()

	err = pem.Encode(caOut, &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	require.NoError(t, err)

	certFile := filepath.Join(tempDir, "client-cert.pem")
	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	require.NoError(t, err)

	keyFile := filepath.Join(tempDir, "client-key.pem")
	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)
	defer keyOut.Close()

	clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientPrivateKey)
	require.NoError(t, err)

	err = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER})
	require.NoError(t, err)

	invalidCAFile := filepath.Join(tempDir, "invalid_ca.pem")
	err = os.WriteFile(invalidCAFile, []byte("This is not a valid PEM certificate\n"), 0o600)
	require.NoError(t, err)

	return caFile, certFile, keyFile, invalidCAFile
}
