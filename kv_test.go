package main

import (
	"context"
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

	require.NoError(t, resolveKVReferences(t.Context(), cfg))
}

func TestResolveKVReferences_NoStoresWithKVRefs_IsError(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
	}

	require.Error(t, resolveKVReferences(t.Context(), cfg))
}

func TestResolveKVReferences_ResolvesTopLevel(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
		KV:                     inlineStore("secrets", map[string]string{"statsd": "statsd-host:8125"}),
	}

	require.NoError(t, resolveKVReferences(context.Background(), cfg))
	assert.Equal(t, "statsd-host:8125", cfg.StatsdConnectionString)
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

	require.NoError(t, resolveKVReferences(context.Background(), cfg))
	assert.Equal(t, "postgres://user:pass@db:5432/tyk",
		cfg.Pumps["sql"].Meta["connection_string"])
}

func TestResolveKVReferences_MissingStoreFailsFast(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://missing/statsd",
		KV:                     inlineStore("secrets", map[string]string{"x": "y"}),
	}

	require.Error(t, resolveKVReferences(context.Background(), cfg))
}

func TestResolveKVReferences_RoundTripPreservesValues(t *testing.T) {
	cfg := &TykPumpConfiguration{
		StatsdConnectionString: "kv://secrets/statsd",
		LogLevel:               "debug",
		PurgeDelay:             42,
		KV:                     inlineStore("secrets", map[string]string{"statsd": "resolved"}),
	}

	require.NoError(t, resolveKVReferences(context.Background(), cfg))
	assert.Equal(t, "resolved", cfg.StatsdConnectionString, "reference must be resolved")
	assert.Equal(t, "debug", cfg.LogLevel, "unrelated string must survive the round trip")
	assert.Equal(t, 42, cfg.PurgeDelay, "unrelated number must survive the round trip")
}

func inlineStore(name string, data map[string]string) kv.Config {
	raw, _ := json.Marshal(map[string]any{"data": data})
	return kv.Config{
		Stores: map[string]kv.StoreConfig{
			name: {Type: kv.Inline, Config: raw},
		},
	}
}
