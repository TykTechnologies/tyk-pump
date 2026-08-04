package pumps

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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

func TestHasUnresolvedKVReference(t *testing.T) {
	cases := []struct {
		name string
		cfg  any
		want bool
	}{
		{"whole-value reference", map[string]any{"connection_string": "kv://vault/db#password"}, true},
		{"inline token", map[string]any{"url": "https://$kv{env:HOST}/v1"}, true},
		{"reference nested in map", map[string]any{"meta": map[string]any{"dsn": "kv://secrets/dsn"}}, true},
		{"malformed marker still detected", map[string]any{"x": "$kv{env:X"}, true},
		{"plain value, no reference", map[string]any{"connection_string": "mongodb://localhost:27017/tyk"}, false},
		{"empty config", map[string]any{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasUnresolvedKVReference(tc.cfg))
		})
	}
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
