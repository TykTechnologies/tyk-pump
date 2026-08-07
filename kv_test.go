package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TykTechnologies/storage/kv"
)

func TestResolveKVReferences_NoStoresWithNoKVRefs_IsNoOp(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "value",
	}

	stores, err := resolveKVReferences(t.Context(), cfg)
	require.NoError(t, err)
	assert.Nil(t, stores, "no stores declared means nothing to keep open")
	assert.Nil(t, stores.Resolver(), "a nil kvStores hands out no resolver")
}

func TestResolveKVReferences_NoStoresWithKVRefs_IsError(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
	}

	stores, err := resolveKVReferences(t.Context(), cfg)
	require.Error(t, err)
	assert.Nil(t, stores)
}

func TestResolveKVReferences_ResolvesTopLevel(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
		KV:                     inlineStore("secrets", map[string]string{"statsd": "statsd-host:8125"}),
	}

	stores := mustResolve(t, cfg)
	assert.Equal(t, "statsd-host:8125", cfg.StatsdConnectionString)
	assert.NotNil(t, stores.Resolver(), "declared stores must stay usable for the pump env var overrides")
}

func TestResolveKVReferences_ResolvesInsidePumpMeta(t *testing.T) {
	cfg := &TykPumpConfiguration{
		Pumps: map[string]PumpConfig{
			"sql": {
				Type: "sql",
				Meta: map[string]any{
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

	stores, err := resolveKVReferences(t.Context(), cfg)
	require.Error(t, err)
	assert.Nil(t, stores, "a failed resolution must not leak an open registry")
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

	stores := mustResolve(t, cfg)
	require.NotNil(t, stores.Resolver())

	resolved, err := stores.Resolver().ResolveAll(t.Context(), []byte(`{"connection_string":"kv://secrets/dsn"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"connection_string":"postgres://db"}`, string(resolved))
}

func TestKVStores_NilIsUsable(t *testing.T) {
	var stores *kvStores

	assert.Nil(t, stores.Resolver(), "a nil kvStores hands out no resolver")
	assert.NotPanics(t, func() { stores.Close(t.Context()) })
}

func TestKVStores_CloseInvalidatesResolver(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
		KV:                     inlineStore("secrets", map[string]string{"statsd": "statsd-host:8125"}),
	}

	stores, err := resolveKVReferences(t.Context(), cfg)
	require.NoError(t, err)
	require.NotNil(t, stores.Resolver())

	doc := []byte(`{"statsd_connection_string":"kv://secrets/statsd"}`)

	_, err = stores.Resolver().ResolveAll(t.Context(), doc)
	require.NoError(t, err, "the resolver must work while the stores are open")

	stores.Close(t.Context())

	_, err = stores.Resolver().ResolveAll(t.Context(), doc)
	require.Error(t, err, "closing the stores must invalidate the resolver")
}

// mustResolve resolves cfg and closes the stores when the test ends.
func mustResolve(t *testing.T, cfg *TykPumpConfiguration) *kvStores {
	t.Helper()

	stores, err := resolveKVReferences(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { stores.Close(t.Context()) })

	return stores
}

func inlineStore(name string, data map[string]string) kv.Config {
	//nolint:errcheck
	raw, _ := json.Marshal(map[string]any{"data": data})
	return kv.Config{
		Stores: map[string]kv.StoreConfig{
			name: {Type: kv.Inline, Config: raw},
		},
	}
}
