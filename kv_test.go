package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TykTechnologies/storage/kv"
	"github.com/TykTechnologies/storage/kv/resolver"
)

func TestResolveKVReferences_NoStoresWithNoKVRefs_IsNoOp(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "value",
	}

	reg, res, err := resolveKVReferences(t.Context(), cfg)
	require.NoError(t, err)
	assert.Nil(t, reg, "no stores declared means nothing to keep open")
	assert.Nil(t, res)
}

func TestResolveKVReferences_NoStoresWithKVRefs_IsError(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
	}

	reg, res, err := resolveKVReferences(t.Context(), cfg)
	require.Error(t, err)
	assert.Nil(t, reg)
	assert.Nil(t, res)
}

func TestResolveKVReferences_ResolvesTopLevel(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
		KV:                     inlineStore("secrets", map[string]string{"statsd": "statsd-host:8125"}),
	}

	res := mustResolve(t, cfg)
	assert.Equal(t, "statsd-host:8125", cfg.StatsdConnectionString)
	assert.NotNil(t, res, "declared stores must stay usable for the pump env var overrides")
}

func TestResolveKVReferences_ResolvesInsidePumpMeta(t *testing.T) {
	cfg := &TykPumpConfiguration{
		Pumps: map[string]PumpConfig{
			"sql": {
				Type: "sql",
				Meta: map[string]interface{}{
					"connection_string": "kv://secrets/sql_dsn",
				},
			},
		},
		KV: inlineStore("secrets", map[string]string{"sql_dsn": "postgres://user:pass@db:5432/tyk"}),
	}

	mustResolve(t, cfg)
	assert.Equal(t, "postgres://user:pass@db:5432/tyk",
		cfg.Pumps["sql"].Meta["connection_string"])
}

func TestResolveKVReferences_MissingStoreFailsFast(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://missing/statsd",
		KV:                     inlineStore("secrets", map[string]string{"x": "y"}),
	}

	reg, res, err := resolveKVReferences(context.Background(), cfg)
	require.Error(t, err)
	assert.Nil(t, reg, "a failed resolution must not leak an open registry")
	assert.Nil(t, res)
}

func TestResolveKVReferences_RoundTripPreservesValues(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
		LogLevel:               "debug",
		PurgeDelay:             42,
		KV:                     inlineStore("secrets", map[string]string{"statsd": "resolved"}),
	}

	mustResolve(t, cfg)
	assert.Equal(t, "resolved", cfg.StatsdConnectionString, "reference must be resolved")
	assert.Equal(t, "debug", cfg.LogLevel, "unrelated string must survive the round trip")
	assert.Equal(t, 42, cfg.PurgeDelay, "unrelated number must survive the round trip")
}

func TestResolveKVReferences_ResolverStaysUsable(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
		KV:                     inlineStore("secrets", map[string]string{"statsd": "statsd-host:8125", "dsn": "postgres://db"}),
	}

	res := mustResolve(t, cfg)
	require.NotNil(t, res)

	resolved, err := res.ResolveAll(t.Context(), []byte(`{"connection_string":"kv://secrets/dsn"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"connection_string":"postgres://db"}`, string(resolved))
}

func TestCloseKVStores_IsSafeWithoutStores(t *testing.T) {
	kvRegistry = nil
	assert.NotPanics(t, func() { closeKVStores(context.Background()) })
}

func TestCloseKVRegistry_InvalidatesResolver(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
		KV:                     inlineStore("secrets", map[string]string{"statsd": "statsd-host:8125"}),
	}

	reg, res, err := resolveKVReferences(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, res)

	doc := []byte(`{"statsd_connection_string":"kv://secrets/statsd"}`)

	_, err = res.ResolveAll(context.Background(), doc)
	require.NoError(t, err, "the resolver must work while the registry is open")

	closeKVRegistry(context.Background(), reg)

	_, err = res.ResolveAll(context.Background(), doc)
	require.Error(t, err, "closing the registry must invalidate the resolver")
}

func mustResolve(t *testing.T, cfg *TykPumpConfiguration) resolver.Resolver {
	t.Helper()

	reg, res, err := resolveKVReferences(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { closeKVRegistry(context.Background(), reg) })

	return res
}

func inlineStore(name string, data map[string]string) kv.Config {
	raw, _ := json.Marshal(map[string]any{"data": data})
	return kv.Config{
		Stores: map[string]kv.StoreConfig{
			name: {Type: kv.Inline, Config: raw},
		},
	}
}
