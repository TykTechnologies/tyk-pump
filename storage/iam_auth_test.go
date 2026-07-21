package storage

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/TykTechnologies/storage/temporal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeADC points GOOGLE_APPLICATION_CREDENTIALS at a per-run service-account key
// whose token_uri targets a local token server returning a static access token.
// This lets iamauth.NewProvider(gcp) resolve credentials and mint its initial
// token entirely offline. The server and env var are torn down with the test.
func fakeADC(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(`{"access_token":"fake-token","token_type":"Bearer","expires_in":3600}`)); err != nil {
			t.Errorf("writing token response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling key: %v", err)
	}

	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	sa := map[string]string{
		"type":         "service_account",
		"project_id":   "test-project",
		"private_key":  string(pemKey),
		"client_email": "test@test-project.iam.gserviceaccount.com",
		"client_id":    "1234567890",
		"token_uri":    srv.URL,
	}

	data, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("marshaling service account: %v", err)
	}

	path := filepath.Join(t.TempDir(), "fake-adc.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writing adc file: %v", err)
	}

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
}

func TestBuildIAMAuthOption_Disabled(t *testing.T) {
	opt, err := buildIAMAuthOption(context.Background(), IAMAuthConfig{Enabled: false})
	require.NoError(t, err)
	assert.Nil(t, opt, "no option should be returned when IAM auth is disabled")
}

func TestBuildIAMAuthOption_Success(t *testing.T) {
	// Hermetic ADC lets the gcp provider resolve credentials and mint its
	// initial token offline, so the success path returns a usable option.
	fakeADC(t)

	opt, err := buildIAMAuthOption(context.Background(), IAMAuthConfig{
		Enabled:  true,
		Provider: "gcp",
	})
	require.NoError(t, err)
	assert.NotNil(t, opt, "a credentials-provider option must be returned when IAM auth is configured")
}

func TestBuildIAMAuthOption_UnsupportedProvider(t *testing.T) {
	opt, err := buildIAMAuthOption(context.Background(), IAMAuthConfig{
		Enabled:  true,
		Provider: "aws",
	})
	require.Error(t, err)
	assert.Nil(t, opt)
	assert.Contains(t, err.Error(), "aws")
}

func TestBuildIAMAuthOption_EmptyProvider(t *testing.T) {
	opt, err := buildIAMAuthOption(context.Background(), IAMAuthConfig{Enabled: true})
	require.Error(t, err)
	assert.Nil(t, opt)
}

func TestBuildIAMAuthOption_InvalidRefreshDuration(t *testing.T) {
	opt, err := buildIAMAuthOption(context.Background(), IAMAuthConfig{
		Enabled:                  true,
		Provider:                 "gcp",
		TokenRefreshBeforeExpiry: "not-a-duration",
	})
	require.Error(t, err)
	assert.Nil(t, opt)
	// The refresh duration is parsed inside the storage iamauth package; the
	// error names the offending value.
	assert.Contains(t, err.Error(), "not-a-duration")
}

// With IAM auth enabled, createConnector must build the IAM option and hand a
// working connector back. The connector is lazy (no dial on construction), so
// this exercises the full IAM success branch offline.
func TestCreateConnector_IAMEnabled_Success(t *testing.T) {
	fakeADC(t)

	config := &TemporalStorageConfig{
		Addrs:   []string{"localhost:6379"},
		UseSSL:  false,
		IAMAuth: IAMAuthConfig{Enabled: true, Provider: "gcp"},
	}
	opts := &model.RedisOptions{Addrs: config.Addrs}
	tlsOptions := &model.TLS{Enable: false}

	conn, kv, list, err := createConnector(config, opts, tlsOptions)

	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.NotNil(t, kv)
	assert.NotNil(t, list)
}

// With IAM auth enabled but an unsupported provider, createConnector must fail
// up front, before any connector is built.
func TestCreateConnector_IAMUnsupportedProvider_Errors(t *testing.T) {
	config := &TemporalStorageConfig{
		Addrs:   []string{"localhost:6379"},
		IAMAuth: IAMAuthConfig{Enabled: true, Provider: "aws"},
	}
	opts := &model.RedisOptions{Addrs: config.Addrs}
	tlsOptions := &model.TLS{Enable: false}

	conn, _, _, err := createConnector(config, opts, tlsOptions)

	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "aws")
}

// With IAM auth disabled, createConnector must build a connector without
// touching the IAM path.
func TestCreateConnector_IAMDisabled_Success(t *testing.T) {
	config := &TemporalStorageConfig{Addrs: []string{"localhost:6379"}}
	opts := &model.RedisOptions{Addrs: config.Addrs}
	tlsOptions := &model.TLS{Enable: false}

	conn, kv, list, err := createConnector(config, opts, tlsOptions)

	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.NotNil(t, kv)
	assert.NotNil(t, list)
}
