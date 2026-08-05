package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TykTechnologies/storage/kv"
	"github.com/TykTechnologies/storage/kv/registry"
	"github.com/TykTechnologies/storage/kv/resolver"
	"github.com/sirupsen/logrus"
)

type kvLogger struct{ l *logrus.Logger }

func (a kvLogger) Warn(msg string, fields map[string]any)  { a.l.WithFields(fields).Warn(msg) }
func (a kvLogger) Debug(msg string, fields map[string]any) { a.l.WithFields(fields).Debug(msg) }
func (a kvLogger) Error(msg string, fields map[string]any) { a.l.WithFields(fields).Error(msg) }

// resolveKVReferences dereferences KV references in an already-loaded config:
// it returns an error if the config contains KV references but no stores are
// configured, and is a no-op when neither references nor stores are present.
//
// It returns the store registry alongside the resolver built from it. The pump
// specific env var overrides are applied later, when each pump initialises, and have
// to resolve against the same stores - so the registry cannot be closed here. The
// caller owns closing it.
func resolveKVReferences(
	ctx context.Context,
	config *TykPumpConfiguration,
) (*registry.Registry, resolver.Resolver, error) {
	marshaledBytes, err := json.Marshal(config)
	if err != nil {
		return nil, nil, fmt.Errorf("encode config: %w", err)
	}

	if len(config.KV.Stores) == 0 {
		if resolver.ContainsReferences(marshaledBytes) {
			return nil, nil, errors.New("config contains KV references but no stores are configured")
		}

		return nil, nil, nil
	}

	reg, err := registry.NewFromConfig(
		ctx,
		marshaledBytes,
		registry.WithFactories(enterpriseKVFactories()),
		registry.WithInitLogger(kvLogger{l: log}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize KV registry: %w", err)
	}

	res := resolver.NewResolver(reg)

	resolvedBytes, err := res.ResolveAll(ctx, marshaledBytes)
	if err != nil {
		closeKVRegistry(ctx, reg)

		return nil, nil, fmt.Errorf("resolve KV references in config: %w", err)
	}

	err = json.Unmarshal(resolvedBytes, config)
	if err != nil {
		closeKVRegistry(ctx, reg)

		return nil, nil, fmt.Errorf("decode resolved config: %w", err)
	}

	return reg, res, nil
}

func closeKVRegistry(ctx context.Context, reg *registry.Registry) {
	if reg == nil {
		return
	}

	if err := reg.Close(context.WithoutCancel(ctx)); err != nil {
		log.WithError(err).Warn("failed to close KV store registry")
	}
}

func enterpriseKVFactories() map[kv.ProviderType]kv.ProviderFactory {
	factories := make(map[kv.ProviderType]kv.ProviderFactory)

	// FIX:: Uncomment after providers are added and released
	// factories[kv.AWS] = aws.NewFactory()
	// factories[kv.Azure] = azure.NewFactory()
	// factories[kv.GCP] = gcp.NewFactory()

	return factories
}
