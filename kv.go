package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TykTechnologies/storage/kv/registry"
	"github.com/TykTechnologies/storage/kv/resolver"
	"github.com/sirupsen/logrus"
)

type kvLogger struct{ l *logrus.Logger }

func (a kvLogger) Warn(msg string, fields map[string]any)  { a.l.WithFields(fields).Warn(msg) }
func (a kvLogger) Debug(msg string, fields map[string]any) { a.l.WithFields(fields).Debug(msg) }
func (a kvLogger) Error(msg string, fields map[string]any) { a.l.WithFields(fields).Error(msg) }

// resolveKVReferences dereferences KV references in an already-loaded config:
// it builds the KV registry from the kv section and resolves every reference in
// a single pass. It is a no-op when no stores are configured.
func resolveKVReferences(ctx context.Context, config *TykPumpConfiguration) error {
	// No stores can be referenced.
	if len(config.KV.Stores) == 0 {
		return nil
	}

	marshaledBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	reg, err := registry.NewFromConfig(
		ctx,
		marshaledBytes,
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

	res := resolver.NewResolver(reg)

	resolvedBytes, err := res.ResolveAll(ctx, marshaledBytes)
	if err != nil {
		return fmt.Errorf("resolve KV references in config: %w", err)
	}

	err = json.Unmarshal(resolvedBytes, config)
	if err != nil {
		return fmt.Errorf("decode resolved config: %w", err)
	}

	return nil
}
