package storage

import (
	"context"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/TykTechnologies/storage/iamauth"
	"github.com/TykTechnologies/storage/temporal/model"
)

// iamAuthTimeout bounds the eager token mint performed when building the IAM
// credentials provider, so a slow or unresponsive auth endpoint fails fast
// instead of hanging startup or reconnection. It is a var so tests can shorten
// it.
var iamAuthTimeout = 30 * time.Second

// buildIAMAuthOption returns a storage Option that wires an IAM-based credentials
// provider, or nil when IAM auth is disabled. It returns an error for an
// unsupported provider or an invalid configuration value.
//
// Provider selection, refresh-duration parsing, and the cloud SDKs all live in
// the storage library's iamauth package; this adapter only maps Pump config onto
// iamauth.Config.
func buildIAMAuthOption(ctx context.Context, cfg IAMAuthConfig) (model.Option, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// The provider mints an initial token via a blocking network call. A context
	// deadline alone does not bound it: the google/oauth2 token source detaches
	// the token HTTP request from the construction context's deadline. Injecting
	// an HTTP client with a timeout (which the token source honours via
	// oauth2.HTTPClient) does bound each token fetch, so a slow or unresponsive
	// auth endpoint fails fast instead of hanging startup or reconnection.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: iamAuthTimeout})

	provider, err := iamauth.NewProvider(ctx, iamauth.Config{
		Provider:            strings.ToLower(strings.TrimSpace(cfg.Provider)),
		ServiceAccount:      cfg.ServiceAccount,
		RefreshBeforeExpiry: cfg.TokenRefreshBeforeExpiry,
	})
	if err != nil {
		return nil, err
	}

	return model.WithCredentialsProvider(provider), nil
}
