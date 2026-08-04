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
func resolveKVReferences(ctx context.Context, config *TykPumpConfiguration) error {
	marshaledBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	if len(config.KV.Stores) == 0 {
		if resolver.ContainsReferences(marshaledBytes) {
			return errors.New("config contains KV references but no stores are configured")
		}

		return nil
	}

	reg, err := registry.NewFromConfig(
		ctx,
		marshaledBytes,
		registry.WithFactories(enterpriseKVFactories()),
		registry.WithInitLogger(kvLogger{l: log}),
	)
	if err != nil {
		return fmt.Errorf("initialize KV registry: %w", err)
	}
	defer func() {
		if cerr := reg.Close(context.WithoutCancel(ctx)); cerr != nil {
			log.WithError(cerr).Warn("failed to close KV store registry after config resolution")
		}
	}()

	resolvedBytes, err := resolver.NewResolver(reg).ResolveAll(ctx, marshaledBytes)
	if err != nil {
		return fmt.Errorf("resolve KV references in config: %w", err)
	}

	err = json.Unmarshal(resolvedBytes, config)
	if err != nil {
		return fmt.Errorf("decode resolved config: %w", err)
	}

	return nil
}

func enterpriseKVFactories() map[kv.ProviderType]kv.ProviderFactory {
	factories := make(map[kv.ProviderType]kv.ProviderFactory)

	// FIX:: Uncomment after providers are added and released
	// factories[kv.AWS] = aws.NewFactory()
	// factories[kv.Azure] = azure.NewFactory()
	// factories[kv.GCP] = gcp.NewFactory()

	return factories
}
