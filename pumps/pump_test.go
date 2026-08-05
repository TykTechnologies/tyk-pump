package pumps

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/TykTechnologies/storage/kv/resolver"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPumpByName(t *testing.T) {

	dummyType, err := GetPumpByName("dummy")
	assert.NoError(t, err)
	assert.Equal(t, dummyType, &DummyPump{})

	invalidPump, err := GetPumpByName("xyz")
	assert.Error(t, err)
	assert.Nil(t, invalidPump)

	mongoPump, err := GetPumpByName("MONGO")
	assert.NoError(t, err)
	assert.Equal(t, mongoPump, &MongoPump{})

	sqlPump, err := GetPumpByName("SqL")
	assert.NoError(t, err)
	assert.Equal(t, sqlPump, &SQLPump{})
}

type fakeKVResolver struct {
	values map[string]string
	err    error
	calls  int
}

func (f *fakeKVResolver) Resolve(_ context.Context, input string) (string, error) {
	f.calls++

	if f.err != nil {
		return "", f.err
	}

	return f.substitute(input), nil
}

func (f *fakeKVResolver) ResolveAll(_ context.Context, rawJSON []byte) ([]byte, error) {
	f.calls++

	if f.err != nil {
		return nil, f.err
	}

	return []byte(f.substitute(string(rawJSON))), nil
}

func (f *fakeKVResolver) substitute(in string) string {
	for ref, value := range f.values {
		in = strings.ReplaceAll(in, ref, value)
	}

	return in
}

func installKVResolver(t *testing.T, r resolver.Resolver) {
	t.Helper()

	prev := kvResolver
	kvResolver = r
	t.Cleanup(func() { kvResolver = prev })
}

func TestResolveKVReferences(t *testing.T) {
	const mongoURL = "mongodb://localhost:27017/tyk"

	t.Run("no reference leaves the config untouched", func(t *testing.T) {
		fake := &fakeKVResolver{}
		installKVResolver(t, fake)
		cfg := &MongoConf{BaseMongoConf: BaseMongoConf{MongoURL: mongoURL}}

		require.NoError(t, resolveKVReferences(t.Context(), cfg))
		assert.Equal(t, mongoURL, cfg.MongoURL)
		assert.Zero(t, fake.calls, "a config without references must not reach the KV stores")
	})

	t.Run("resolves a whole-value reference", func(t *testing.T) {
		installKVResolver(t, &fakeKVResolver{values: map[string]string{"kv://vault/mongo#url": mongoURL}})
		cfg := &MongoConf{
			BaseMongoConf:  BaseMongoConf{MongoURL: "kv://vault/mongo#url"},
			CollectionName: "tyk_analytics",
		}

		require.NoError(t, resolveKVReferences(t.Context(), cfg))
		assert.Equal(t, mongoURL, cfg.MongoURL)
		assert.Equal(t, "tyk_analytics", cfg.CollectionName, "unrelated values must survive the round trip")
	})

	t.Run("resolves an inline reference", func(t *testing.T) {
		installKVResolver(t, &fakeKVResolver{values: map[string]string{"$kv{vault:mongo#host}": "db:27017"}})
		cfg := &MongoConf{BaseMongoConf: BaseMongoConf{MongoURL: "mongodb://$kv{vault:mongo#host}/tyk"}}

		require.NoError(t, resolveKVReferences(t.Context(), cfg))
		assert.Equal(t, "mongodb://db:27017/tyk", cfg.MongoURL)
	})

	t.Run("errors when no stores are configured", func(t *testing.T) {
		installKVResolver(t, nil)
		cfg := &MongoConf{BaseMongoConf: BaseMongoConf{MongoURL: "kv://vault/mongo#url"}}

		err := resolveKVReferences(t.Context(), cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no KV stores are configured")
	})

	t.Run("propagates a resolution failure", func(t *testing.T) {
		installKVResolver(t, &fakeKVResolver{err: errors.New("vault unreachable")})
		cfg := &MongoConf{BaseMongoConf: BaseMongoConf{MongoURL: "kv://vault/mongo#url"}}

		err := resolveKVReferences(t.Context(), cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vault unreachable")
		assert.Equal(t, "kv://vault/mongo#url", cfg.MongoURL, "a failed resolution must not partially rewrite the config")
	})

	t.Run("preserves non-string values across the round trip", func(t *testing.T) {
		installKVResolver(t, &fakeKVResolver{values: map[string]string{"kv://vault/mongo#url": mongoURL}})
		cfg := &MongoConf{
			BaseMongoConf:           BaseMongoConf{MongoURL: "kv://vault/mongo#url", MongoDBType: CosmosDB, MongoUseSSL: true},
			MaxInsertBatchSizeBytes: 80000,
			CollectionCapEnable:     true,
		}

		require.NoError(t, resolveKVReferences(t.Context(), cfg))
		assert.Equal(t, mongoURL, cfg.MongoURL)
		assert.Equal(t, CosmosDB, cfg.MongoDBType)
		assert.True(t, cfg.MongoUseSSL)
		assert.Equal(t, 80000, cfg.MaxInsertBatchSizeBytes)
		assert.True(t, cfg.CollectionCapEnable)
	})
}

func TestProcessPumpEnvVars(t *testing.T) {
	newMongoPump := func() *MongoPump {
		pump := &MongoPump{dbConf: &MongoConf{}}
		pump.log = log.WithField("prefix", mongoPrefix)

		return pump
	}

	t.Run("resolves a reference set through the meta env prefix", func(t *testing.T) {
		_, exits := captureLog(t)
		installKVResolver(t, &fakeKVResolver{values: map[string]string{"kv://vault/mongo#url": "mongodb://db:27017/tyk"}})
		t.Setenv(mongoDefaultEnv+"_MONGOURL", "kv://vault/mongo#url")

		pump := newMongoPump()
		processPumpEnvVars(pump, pump.log, pump.dbConf, mongoDefaultEnv)

		assert.Zero(t, *exits)
		assert.Equal(t, "mongodb://db:27017/tyk", pump.dbConf.MongoURL)
	})

	t.Run("resolves a reference set through a custom meta_env_prefix", func(t *testing.T) {
		_, exits := captureLog(t)
		installKVResolver(t, &fakeKVResolver{values: map[string]string{"kv://vault/mongo#url": "mongodb://db:27017/custom"}})
		t.Setenv("MYMONGO_MONGOURL", "kv://vault/mongo#url")

		pump := newMongoPump()
		pump.dbConf.EnvPrefix = "MYMONGO"
		processPumpEnvVars(pump, pump.log, pump.dbConf, mongoDefaultEnv)

		assert.Zero(t, *exits)
		assert.Equal(t, "mongodb://db:27017/custom", pump.dbConf.MongoURL)
	})

	t.Run("is fatal when a reference cannot be resolved", func(t *testing.T) {
		out, exits := captureLog(t)
		installKVResolver(t, &fakeKVResolver{err: errors.New("vault unreachable")})
		t.Setenv(mongoDefaultEnv+"_MONGOURL", "kv://vault/mongo#url")

		pump := newMongoPump()
		processPumpEnvVars(pump, pump.log, pump.dbConf, mongoDefaultEnv)

		assert.Equal(t, 1, *exits)
		assert.Contains(t, out.String(), "vault unreachable")
	})

	t.Run("plain override needs no resolver", func(t *testing.T) {
		_, exits := captureLog(t)
		installKVResolver(t, nil)
		t.Setenv(mongoDefaultEnv+"_COLLECTIONNAME", "tyk_analytics")

		pump := newMongoPump()
		processPumpEnvVars(pump, pump.log, pump.dbConf, mongoDefaultEnv)

		assert.Zero(t, *exits)
		assert.Equal(t, "tyk_analytics", pump.dbConf.CollectionName)
	})
}

func TestEnvVarNamesWithPrefix(t *testing.T) {
	// PMP_MONGO is a prefix of both PMP_MONGOAGG and PMP_MONGOSEL, so each of the
	// three deprecated prefixes must only ever see its own variables.
	t.Setenv(mongoPumpPrefix+"_MONGOURL", "mongodb://localhost:27017/tyk")
	t.Setenv(mongoPumpPrefix+"_COLLECTIONNAME", "tyk_analytics")
	t.Setenv(mongoAggregatePumpPrefix+"_MONGOURL", "mongodb://localhost:27017/agg")
	t.Setenv(mongoSelectivePumpPrefix+"_MONGOURL", "mongodb://localhost:27017/sel")
	t.Setenv(mongoPumpPrefix, "not-a-prefixed-var")

	assert.ElementsMatch(t,
		[]string{mongoPumpPrefix + "_MONGOURL", mongoPumpPrefix + "_COLLECTIONNAME"},
		envVarNamesWithPrefix(mongoPumpPrefix))
	assert.Equal(t, []string{mongoAggregatePumpPrefix + "_MONGOURL"}, envVarNamesWithPrefix(mongoAggregatePumpPrefix))
	assert.Equal(t, []string{mongoSelectivePumpPrefix + "_MONGOURL"}, envVarNamesWithPrefix(mongoSelectivePumpPrefix))
	assert.Empty(t, envVarNamesWithPrefix("PMP_NOTSET"))
}

func TestEnvVarsWithKVReference(t *testing.T) {
	t.Setenv(mongoPumpPrefix+"_MONGOURL", "kv://vault/mongo#url")
	t.Setenv(mongoPumpPrefix+"_MONGOSSLCAFILE", "$kv{consul:certs/ca}")
	t.Setenv(mongoPumpPrefix+"_COLLECTIONNAME", "tyk_analytics")
	t.Setenv(mongoPumpPrefix+"_MONGOSESSIONCONSISTENCY", "")

	refs := envVarsWithKVReference(envVarNamesWithPrefix(mongoPumpPrefix))

	assert.ElementsMatch(t,
		[]string{mongoPumpPrefix + "_MONGOURL", mongoPumpPrefix + "_MONGOSSLCAFILE"},
		refs)
}

func TestRenamedEnvVars(t *testing.T) {
	renamed := renamedEnvVars(
		[]string{mongoPumpPrefix + "_MONGOURL", mongoPumpPrefix + "_COLLECTIONNAME"},
		mongoPumpPrefix, mongoDefaultEnv)

	assert.Equal(t, []string{
		"PMP_MONGO_MONGOURL -> TYK_PMP_PUMPS_MONGO_META_MONGOURL",
		"PMP_MONGO_COLLECTIONNAME -> TYK_PMP_PUMPS_MONGO_META_COLLECTIONNAME",
	}, renamed)
}

func TestProcessLegacyPumpEnvVars(t *testing.T) {
	newMongoPump := func() *MongoPump {
		pump := &MongoPump{dbConf: &MongoConf{}}
		pump.log = log.WithField("prefix", mongoPrefix)

		return pump
	}

	t.Run("stays quiet when no deprecated var is set", func(t *testing.T) {
		out, exits := captureLog(t)
		pump := newMongoPump()

		processLegacyPumpEnvVars(pump, pump.log, pump.dbConf, mongoPumpPrefix, mongoDefaultEnv)

		assert.Zero(t, *exits)
		assert.NotContains(t, out.String(), "deprecated")
	})

	t.Run("warns and still applies the override", func(t *testing.T) {
		out, exits := captureLog(t)
		pump := newMongoPump()
		t.Setenv(mongoPumpPrefix+"_COLLECTIONNAME", "legacy_collection")

		processLegacyPumpEnvVars(pump, pump.log, pump.dbConf, mongoPumpPrefix, mongoDefaultEnv)

		assert.Zero(t, *exits)
		assert.Contains(t, out.String(), "deprecated")
		assert.Contains(t, out.String(), "PMP_MONGO_COLLECTIONNAME -> TYK_PMP_PUMPS_MONGO_META_COLLECTIONNAME")
		// backward compatibility: the deprecated var must keep working
		assert.Equal(t, "legacy_collection", pump.dbConf.CollectionName)
	})

	t.Run("is fatal when a deprecated var holds a KV reference", func(t *testing.T) {
		out, exits := captureLog(t)
		pump := newMongoPump()
		t.Setenv(mongoPumpPrefix+"_MONGOURL", "kv://vault/mongo#url")

		processLegacyPumpEnvVars(pump, pump.log, pump.dbConf, mongoPumpPrefix, mongoDefaultEnv)

		assert.Equal(t, 1, *exits)
		assert.Contains(t, out.String(), "do not support KV references")
		assert.Contains(t, out.String(), "PMP_MONGO_MONGOURL")
	})

	t.Run("points the uptime pump at its own replacement prefix", func(t *testing.T) {
		out, exits := captureLog(t)
		pump := newMongoPump()
		pump.IsUptime = true
		t.Setenv(mongoPumpPrefix+"_MONGOURL", "mongodb://localhost:27017/uptime")

		processLegacyPumpEnvVars(pump, pump.log, pump.dbConf, mongoPumpPrefix, uptimeConfEnvPrefix)

		assert.Zero(t, *exits)
		assert.Contains(t, out.String(), "PMP_MONGO_MONGOURL -> TYK_PMP_UPTIMEPUMPCONFIG_MONGOURL")
	})
}

func TestEffectiveEnvPrefix(t *testing.T) {
	t.Run("falls back to the default when meta_env_prefix is unset", func(t *testing.T) {
		pump := &MongoPump{dbConf: &MongoConf{}}
		assert.Equal(t, mongoDefaultEnv, effectiveEnvPrefix(pump, mongoDefaultEnv))
	})

	t.Run("prefers the configured meta_env_prefix", func(t *testing.T) {
		pump := &MongoPump{dbConf: &MongoConf{BaseMongoConf: BaseMongoConf{EnvPrefix: "MYMONGO"}}}
		assert.Equal(t, "MYMONGO", effectiveEnvPrefix(pump, mongoDefaultEnv))
	})
}

func captureLog(t *testing.T) (*bytes.Buffer, *int) {
	t.Helper()

	buf := &bytes.Buffer{}
	exits := 0

	prevOut, prevLevel, prevExit := log.Out, log.Level, log.ExitFunc
	log.Out, log.Level, log.ExitFunc = buf, logrus.DebugLevel, func(int) { exits++ }

	t.Cleanup(func() {
		log.Out, log.Level, log.ExitFunc = prevOut, prevLevel, prevExit
	})

	return buf, &exits
}
