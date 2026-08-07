package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TykTechnologies/storage/kv"
	"github.com/TykTechnologies/storage/kv/providers/aws"
	"github.com/TykTechnologies/storage/kv/providers/azure"
	"github.com/TykTechnologies/storage/kv/providers/gcp"
	"github.com/TykTechnologies/storage/kv/registry"
	"github.com/TykTechnologies/storage/kv/resolver"
	"github.com/sirupsen/logrus"
)

type kvLogger struct{ l *logrus.Logger }

func (a kvLogger) Warn(msg string, fields map[string]any)  { a.l.WithFields(fields).Warn(msg) }
func (a kvLogger) Debug(msg string, fields map[string]any) { a.l.WithFields(fields).Debug(msg) }
func (a kvLogger) Error(msg string, fields map[string]any) { a.l.WithFields(fields).Error(msg) }

// kvStores holds the KV store connections opened to resolve the configuration, together
// with the resolver that reads them. Both have the same lifetime, which is why they
// travel together.
//
// A nil *kvStores is valid and means "no KV stores are configured", so callers never
// have to branch on it.
type kvStores struct {
	registry *registry.Registry
	resolver resolver.Resolver
}

// Resolver returns the resolver to dereference references against, or nil when there
// are no stores to resolve from.
func (s *kvStores) Resolver() resolver.Resolver {
	if s == nil {
		return nil
	}

	return s.resolver
}

// Close releases the store connections. Closing empties the registry, so it reports
// every store as not found afterwards: whoever holds the Resolver must stop using it at
// the same time.
func (s *kvStores) Close(ctx context.Context) {
	if s == nil || s.registry == nil {
		return
	}

	if err := s.registry.Close(context.WithoutCancel(ctx)); err != nil {
		log.WithError(err).Warn("failed to close KV store registry")
	}
}

// resolveKVReferences dereferences KV references in an already-loaded config:
// it returns an error if the config contains KV references but no stores are
// configured, and is a no-op when neither references nor stores are present.
//
// It returns the stores it resolved against, still open. The pump specific env var
// overrides are applied later, when each pump initialises, and resolve against these
// same stores - so the caller must keep them until every pump is up, and Close them
// then. The result is nil when the config declares no stores.
func resolveKVReferences(ctx context.Context, config *TykPumpConfiguration) (*kvStores, error) {
	marshaledBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}

	if len(config.KV.Stores) == 0 {
		if resolver.ContainsReferences(marshaledBytes) {
			return nil, errors.New("config contains KV references but no stores are configured")
		}

		return nil, nil
	}

	reg, err := registry.NewFromConfig(
		ctx,
		marshaledBytes,
		registry.WithFactories(enterpriseKVFactories()),
		registry.WithInitLogger(kvLogger{l: log}),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize KV registry: %w", err)
	}

	stores := &kvStores{registry: reg, resolver: resolver.NewResolver(reg)}

	resolvedBytes, err := stores.resolver.ResolveAll(ctx, marshaledBytes)
	if err != nil {
		stores.Close(ctx)

		return nil, fmt.Errorf("resolve KV references in config: %w", err)
	}

	err = json.Unmarshal(resolvedBytes, config)
	if err != nil {
		stores.Close(ctx)

		return nil, fmt.Errorf("decode resolved config: %w", err)
	}

	return stores, nil
}

func enterpriseKVFactories() map[kv.ProviderType]kv.ProviderFactory {
	factories := make(map[kv.ProviderType]kv.ProviderFactory)

	factories[kv.AWS] = aws.NewFactory()
	factories[kv.Azure] = azure.NewFactory()
	factories[kv.GCP] = gcp.NewFactory()

	return factories
}
