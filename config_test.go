package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/TykTechnologies/storage/kv"
	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TykTechnologies/tyk-pump/internal/otel"
)

func TestToUpperPumps(t *testing.T) {
	pumpNames := []string{"test1", "test2", "tEst3", "Test4"}
	initialConfig := &TykPumpConfiguration{
		Pumps: make(map[string]PumpConfig),
	}
	initialConfig.Pumps[pumpNames[0]] = PumpConfig{Type: "mongo"}
	initialConfig.Pumps[pumpNames[1]] = PumpConfig{Type: "sql"}
	initialConfig.Pumps[pumpNames[2]] = PumpConfig{Type: "mongo-aggregate"}
	initialConfig.Pumps[pumpNames[3]] = PumpConfig{Type: "csv"}
	os.Setenv(ENV_PREVIX+"_PUMPS_TEST3_TYPE", "sql-aggregate")
	defer os.Unsetenv(ENV_PREVIX + "_PUMPS_TEST3_TYPE")

	defaultPath := ""
	LoadConfig(&defaultPath, initialConfig)
	assert.Equal(t, len(pumpNames), len(initialConfig.Pumps))
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[0])].Type, "mongo")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[1])].Type, "sql")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[3])].Type, "csv")
	// Check if the pumps with lower case are empty (don't appear in the map)
	assert.Equal(t, initialConfig.Pumps[pumpNames[0]], PumpConfig{})
	assert.Equal(t, initialConfig.Pumps[pumpNames[1]], PumpConfig{})
	assert.Equal(t, initialConfig.Pumps[pumpNames[3]], PumpConfig{})

	// Checking if the index 4 overrides the index 2 (the original value was 'mongo')
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[2])].Type, "sql-aggregate")
}

func TestLoadExampleConf(t *testing.T) {
	defaultPath := "./pump.example.conf"
	initialConfig := &TykPumpConfiguration{}
	LoadConfig(&defaultPath, initialConfig)
	assert.NotZero(t, len(initialConfig.Pumps))

	for k, pump := range initialConfig.Pumps {
		assert.NotNil(t, pump)
		// Checking if the key of the map is equal to the pump type but upper case
		assert.Equal(t, k, strings.ToUpper(pump.Type))
	}
}

func TestConfigEnv(t *testing.T) {
	pumpNameCSV := "CSV"
	pumpNameTest := "TEST"

	testEnvVars := map[string]string{
		PUMPS_ENV_PREFIX + "_" + "TOM":                                   "a",
		PUMPS_ENV_PREFIX + "_" + pumpNameTest + "_FILTERS_ORGIDS":        `a`,
		PUMPS_ENV_PREFIX + "_" + pumpNameTest + "_FILTERS_APIIDS":        `b`,
		PUMPS_ENV_PREFIX + "_" + pumpNameCSV + "_META_DIR":               "/TEST",
		PUMPS_ENV_PREFIX + "_" + pumpNameTest + "_TEST":                  "TEST",
		PUMPS_ENV_PREFIX + "_" + pumpNameTest + "_TIMEOUT":               "10",
		PUMPS_ENV_PREFIX + "_" + pumpNameTest + "_OMITDETAILEDRECORDING": "true",
		PUMPS_ENV_PREFIX + "_" + pumpNameTest + "_TYPE":                  "CSV",
		PUMPS_ENV_PREFIX + "_" + pumpNameCSV + "_FILTERS_APIIDS":         `a,b,c`,
	}

	for env, val := range testEnvVars {
		os.Setenv(env, val)
	}

	defer func() {
		for env := range testEnvVars {
			os.Unsetenv(env)
		}
	}()

	cfg := &TykPumpConfiguration{}
	cfg.Pumps = make(map[string]PumpConfig)
	cfg.Pumps["CSVTEST2"] = PumpConfig{}

	defaultPath := ""
	LoadConfig(&defaultPath, cfg)

	assert.Len(t, cfg.Pumps, 3)

	assert.Contains(t, cfg.Pumps, pumpNameTest)
	assert.Contains(t, cfg.Pumps, pumpNameCSV)
	assert.Contains(t, cfg.Pumps, "CSVTEST2")

	assert.Equal(t, "csv", cfg.Pumps[pumpNameTest].Type)
	assert.Equal(t, 10, cfg.Pumps[pumpNameTest].Timeout)

	assert.Contains(t, cfg.Pumps[pumpNameTest].Meta, "meta_env_prefix")
	assert.Contains(t, cfg.Pumps[pumpNameCSV].Meta, "meta_env_prefix")

	assert.Equal(t, PUMPS_ENV_PREFIX+"_"+pumpNameCSV+PUMPS_ENV_META_PREFIX, cfg.Pumps[pumpNameCSV].Meta["meta_env_prefix"])
	assert.Equal(t, PUMPS_ENV_PREFIX+"_"+pumpNameTest+PUMPS_ENV_META_PREFIX, cfg.Pumps[pumpNameTest].Meta["meta_env_prefix"])

	assert.Len(t, cfg.Pumps[pumpNameCSV].Filters.APIIDs, 3)
}

func TestIgnoreConfig(t *testing.T) {
	defaultPath := "pump.example.conf"

	t.Run("Ignoring the config file", func(t *testing.T) {
		initialConfig := TykPumpConfiguration{PurgeDelay: 5}
		os.Setenv(ENV_PREVIX+"_OMITCONFIGFILE", "true")
		defer os.Unsetenv(ENV_PREVIX + "_OMITCONFIGFILE")
		LoadConfig(&defaultPath, &initialConfig)
		assert.Equal(t, 5, initialConfig.PurgeDelay, "TYK_OMITCONFIGFILE set to true shouldn't have unset the configuration")
	})

	t.Run("Not ignoring the config file", func(t *testing.T) {
		initialConfig := TykPumpConfiguration{PurgeDelay: 5}
		os.Setenv(ENV_PREVIX+"_OMITCONFIGFILE", "false")
		defer os.Unsetenv(ENV_PREVIX + "_OMITCONFIGFILE")
		LoadConfig(&defaultPath, &initialConfig)
		assert.Equal(t, 10, initialConfig.PurgeDelay, "TYK_OMITCONFIGFILE set to false should overwrite the configuration")
	})

	t.Run("Environment variable not set", func(t *testing.T) {
		initialConfig := TykPumpConfiguration{PurgeDelay: 5}
		LoadConfig(&defaultPath, &initialConfig)
		assert.Equal(t, 10, initialConfig.PurgeDelay, "TYK_OMITCONFIGFILE not set should overwrite the configuration")
	})

	t.Run("Config file does not exist", func(t *testing.T) {
		initialConfig := TykPumpConfiguration{PurgeDelay: 5}
		nonexistentPath := "nonexistent_config.json"
		LoadConfig(&nonexistentPath, &initialConfig)
		assert.Equal(t, 5, initialConfig.PurgeDelay, "Nonexistent config file should not affect the configuration")
	})
}

func TestTykPumpConfiguration_LoadPumpsByEnv(t *testing.T) {
	tcs := []struct {
		cfg      *TykPumpConfiguration
		wanted   map[string]PumpConfig
		setup    func()
		teardown func()
		name     string
	}{
		{
			name: "no initial pumps",
			cfg:  &TykPumpConfiguration{},
			setup: func() {
				os.Setenv(ENV_PREVIX+"_PUMPS_ENVTEST_TYPE", "mongo-pump-aggregate")
				os.Setenv(ENV_PREVIX+"_PUMPS_ENVTEST_META_MONGOURL", "mongodb://localhost:27017")
			},
			teardown: func() {
				os.Unsetenv(ENV_PREVIX + "_PUMPS_ENVTEST_TYPE")
				os.Unsetenv(ENV_PREVIX + "_PUMPS_ENVTEST_META_MONGOURL")
			},
			wanted: map[string]PumpConfig{
				"ENVTEST": {
					Type: "mongo-pump-aggregate",
					Meta: map[string]interface{}{
						"meta_env_prefix": ENV_PREVIX + "_PUMPS_ENVTEST_META",
					},
				},
			},
		},
		{
			name: "with initial pumps",
			cfg: &TykPumpConfiguration{
				Pumps: map[string]PumpConfig{
					"INITIAL": {
						Type: "csv",
						Meta: map[string]interface{}{
							"csv_dir": "/tmp",
						},
					},
				},
			},
			setup: func() {
				os.Setenv(ENV_PREVIX+"_PUMPS_ENVTEST_TYPE", "mongo-pump-aggregate")
				os.Setenv(ENV_PREVIX+"_PUMPS_ENVTEST_META_MONGOURL", "mongodb://localhost:27017")
			},
			teardown: func() {
				os.Unsetenv(ENV_PREVIX + "_PUMPS_ENVTEST_TYPE")
				os.Unsetenv(ENV_PREVIX + "_PUMPS_ENVTEST_META_MONGOURL")
			},
			wanted: map[string]PumpConfig{
				"INITIAL": {
					Type: "csv",
					Meta: map[string]interface{}{
						"csv_dir": "/tmp",
					},
				},
				"ENVTEST": {
					Type: "mongo-pump-aggregate",
					Meta: map[string]interface{}{
						"meta_env_prefix": ENV_PREVIX + "_PUMPS_ENVTEST_META",
					},
				},
			},
		},
		{
			name: "type env var not found and type in cfg is empty",
			cfg:  &TykPumpConfiguration{},
			setup: func() {
				os.Setenv(ENV_PREVIX+"_PUMPS_ENVTEST_META_MONGOURL", "mongodb://localhost:27017")
			},
			teardown: func() {
				os.Unsetenv(ENV_PREVIX + "_PUMPS_ENVTEST_META_MONGOURL")
			},
			wanted: map[string]PumpConfig{},
		},
		{
			name: "type env var not found but type in cfg is set",
			cfg: &TykPumpConfiguration{
				Pumps: map[string]PumpConfig{
					"ENVTEST": {
						Type: "mongo",
					},
				},
			},
			setup: func() {
				// Deliberately not setting the TYPE env var for ENVTEST
				os.Setenv(ENV_PREVIX+"_PUMPS_ENVTEST_META_MONGOURL", "mongodb://localhost:27017")
			},
			teardown: func() {
				os.Unsetenv(ENV_PREVIX + "_PUMPS_ENVTEST_META_MONGOURL")
			},
			wanted: map[string]PumpConfig{
				"ENVTEST": {
					Type: "mongo", // Expecting the predefined type to be retained
					Meta: map[string]interface{}{
						"meta_env_prefix": ENV_PREVIX + "_PUMPS_ENVTEST_META",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			defer tc.teardown()

			err := tc.cfg.LoadPumpsByEnv()
			assert.NoError(t, err)
			assert.Equal(t, tc.wanted, tc.cfg.Pumps)
		})
	}
}

func TestLoadPumpsByEnv(t *testing.T) {
	t.Run("preserves existing meta config and adds env prefix", func(t *testing.T) {
		os.Setenv("TYK_PMP_PUMPS_ELASTICSEARCH_META_SSLCAFILE", "env_var_nonexistent_ca.pem")
		defer os.Unsetenv("TYK_PMP_PUMPS_ELASTICSEARCH_META_SSLCAFILE")

		// Start with existing config that has ssl_ca_file in Meta
		cfg := &TykPumpConfiguration{
			Pumps: map[string]PumpConfig{
				"ELASTICSEARCH": {
					Type: "elasticsearch",
					Meta: map[string]any{
						"ssl_ca_file":       "conf_nonexistent_ca.pem",
						"elasticsearch_url": "https://localhost:9200",
					},
				},
			},
		}

		err := cfg.LoadPumpsByEnv()

		assert.NoError(t, err)
		assert.Contains(t, cfg.Pumps, "ELASTICSEARCH")

		// Original Meta values should be preserved; pump will override its meta config during Init() -> processPumpEnvVars() calls
		assert.Contains(t, cfg.Pumps["ELASTICSEARCH"].Meta, "ssl_ca_file")
		assert.Equal(t, "conf_nonexistent_ca.pem", cfg.Pumps["ELASTICSEARCH"].Meta["ssl_ca_file"])
		assert.Contains(t, cfg.Pumps["ELASTICSEARCH"].Meta, "elasticsearch_url")

		assert.Contains(t, cfg.Pumps["ELASTICSEARCH"].Meta, "meta_env_prefix")
		assert.Equal(t, PUMPS_ENV_PREFIX+"_ELASTICSEARCH"+PUMPS_ENV_META_PREFIX,
			cfg.Pumps["ELASTICSEARCH"].Meta["meta_env_prefix"])
	})
}

func TestKVStoresFromEnv(t *testing.T) {
	t.Setenv("TYK_PMP_KV_STORES", `{
		"vault-prod": {
			"type": "hashicorp_vault",
			"required": true,
			"config": {"address": "http://localhost:8200", "namespace": "team-a"}
		}
	}`)

	cfg := &TykPumpConfiguration{}
	require.NoError(t, envconfig.Process(ENV_PREVIX, cfg))

	require.Len(t, cfg.KV.Stores, 1)
	sc, ok := cfg.KV.Stores["vault-prod"]
	require.True(t, ok, "TYK_PMP_KV_STORES must define vault-prod")
	require.Equal(t, kv.Vault, sc.Type)
	require.True(t, sc.Required)
	require.JSONEq(t, `{"address": "http://localhost:8200", "namespace": "team-a"}`, string(sc.Config))
}

func TestKVStoresFromEnvOverridesFile(t *testing.T) {
	t.Setenv("TYK_PMP_KV_STORES", `{"vault-prod": {"type": "hashicorp_vault", "config": {"namespace": "from-env"}}}`)

	cfg := &TykPumpConfiguration{}
	cfg.KV.Stores = kv.Stores{
		"vault-prod": {Type: kv.Vault, Config: []byte(`{"namespace": "from-file"}`)},
		"local-file": {Type: kv.File, Config: []byte(`{"base_path": "/etc/secrets"}`)},
	}

	require.NoError(t, envconfig.Process(ENV_PREVIX, cfg))

	require.Len(t, cfg.KV.Stores, 2)
	require.JSONEq(t, `{"namespace": "from-env"}`, string(cfg.KV.Stores["vault-prod"].Config), "env overrides the file entry")
	require.Equal(t, kv.File, cfg.KV.Stores["local-file"].Type, "entry absent from env JSON is retained")
}

// writeConfigFile writes a Pump JSON config to a temp dir and returns its path.
func writeConfigFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pump.conf")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestOpenTelemetryConfig_AbsentBlock(t *testing.T) {
	// Backwards compatibility: an existing config without the block and no
	// OTel env vars means metrics stay off.
	path := writeConfigFile(t, `{"purge_delay": 10}`)

	cfg := &TykPumpConfiguration{}
	LoadConfig(&path, cfg)

	assert.Equal(t, 10, cfg.PurgeDelay)
	assert.Nil(t, cfg.OpenTelemetry.Metrics.Enabled, "metrics must be disabled unless enabled is explicitly true")
	assert.False(t, cfg.OpenTelemetry.MetricsEnabled())
	assert.Empty(t, cfg.OpenTelemetry.Metrics.Exporter)
	assert.Empty(t, cfg.OpenTelemetry.Metrics.Endpoint)
	assert.Empty(t, cfg.OpenTelemetry.Metrics.ResourceName)
	assert.Empty(t, cfg.OpenTelemetry.Metrics.DeploymentEnvironment)
}

func TestOpenTelemetryConfig_JSONBlock(t *testing.T) {
	path := writeConfigFile(t, `{
		"opentelemetry": {
			"metrics": {
				"enabled": true,
				"exporter": "http",
				"endpoint": "otel-collector:4318",
				"headers": {"x-team": "platform"},
				"export_interval": 15,
				"deployment_environment": "staging"
			}
		}
	}`)

	cfg := &TykPumpConfiguration{}
	LoadConfig(&path, cfg)

	m := cfg.OpenTelemetry.Metrics
	assert.True(t, cfg.OpenTelemetry.MetricsEnabled())
	assert.Equal(t, "http", m.Exporter)
	assert.Equal(t, "otel-collector:4318", m.Endpoint)
	assert.Equal(t, map[string]string{"x-team": "platform"}, m.Headers)
	assert.Equal(t, 15, m.ExportInterval)
	assert.Equal(t, "staging", m.DeploymentEnvironment)

	t.Run("defaults", func(t *testing.T) {
		path := writeConfigFile(t, `{"opentelemetry": {"metrics": {"enabled": true}}}`)

		cfg := &TykPumpConfiguration{}
		LoadConfig(&path, cfg)
		otelCfg := cfg.OpenTelemetry
		otelCfg.SetDefaults()

		assert.True(t, otelCfg.MetricsEnabled())
		assert.Equal(t, "tyk-pump", otelCfg.Metrics.ResourceName)
		assert.Equal(t, "unknown", otelCfg.Metrics.DeploymentEnvironment)
		assert.Equal(t, "grpc", otelCfg.Metrics.Exporter)
		assert.Equal(t, "localhost:4317", otelCfg.Metrics.Endpoint)
		assert.Equal(t, 60, otelCfg.Metrics.ExportInterval)
	})
}

func TestOpenTelemetryConfig_EnvOverridesFile(t *testing.T) {
	path := writeConfigFile(t, `{
		"opentelemetry": {
			"metrics": {
				"enabled": false,
				"exporter": "http",
				"endpoint": "file-collector:4318",
				"headers": {"x-from": "file"},
				"deployment_environment": "from-file"
			}
		}
	}`)
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_ENABLED", "true")
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_EXPORTER", "grpc")
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_ENDPOINT", "env-collector:4317")
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_HEADERS", "x-from:env")
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_DEPLOYMENTENVIRONMENT", "from-env")

	cfg := &TykPumpConfiguration{}
	LoadConfig(&path, cfg)

	m := cfg.OpenTelemetry.Metrics
	assert.True(t, cfg.OpenTelemetry.MetricsEnabled(), "env true must override file false")
	assert.Equal(t, "grpc", m.Exporter)
	assert.Equal(t, "env-collector:4317", m.Endpoint)
	assert.Equal(t, map[string]string{"x-from": "env"}, m.Headers)
	assert.Equal(t, "from-env", m.DeploymentEnvironment)
}

// gatewayOpenTelemetryMetricsEnv lists the Gateway's environment variables for
// the shared opentelemetry.metrics settings (the library's MetricsConfig).
// They are spelled out on purpose: envconfig derives names from struct field
// names, so a field rename silently breaks TYK_GW_* <-> TYK_PMP_* parity, and
// deriving these strings would hide it. Do not generate this list.
var gatewayOpenTelemetryMetricsEnv = []string{
	"TYK_GW_OPENTELEMETRY_METRICS_ENABLED",
	"TYK_GW_OPENTELEMETRY_METRICS_EXPORTER",
	"TYK_GW_OPENTELEMETRY_METRICS_ENDPOINT",
	"TYK_GW_OPENTELEMETRY_METRICS_HEADERS",
	"TYK_GW_OPENTELEMETRY_METRICS_CONNECTIONTIMEOUT",
	"TYK_GW_OPENTELEMETRY_METRICS_RESOURCENAME",
	"TYK_GW_OPENTELEMETRY_METRICS_TLS_ENABLE",
	"TYK_GW_OPENTELEMETRY_METRICS_TLS_INSECURESKIPVERIFY",
	"TYK_GW_OPENTELEMETRY_METRICS_TLS_CAFILE",
	"TYK_GW_OPENTELEMETRY_METRICS_TLS_CERTFILE",
	"TYK_GW_OPENTELEMETRY_METRICS_TLS_KEYFILE",
	"TYK_GW_OPENTELEMETRY_METRICS_TLS_MAXVERSION",
	"TYK_GW_OPENTELEMETRY_METRICS_TLS_MINVERSION",
	"TYK_GW_OPENTELEMETRY_METRICS_EXPORTINTERVAL",
	"TYK_GW_OPENTELEMETRY_METRICS_TEMPORALITY",
	"TYK_GW_OPENTELEMETRY_METRICS_SHUTDOWNTIMEOUT",
	"TYK_GW_OPENTELEMETRY_METRICS_RETRY_ENABLED",
	"TYK_GW_OPENTELEMETRY_METRICS_RETRY_INITIALINTERVAL",
	"TYK_GW_OPENTELEMETRY_METRICS_RETRY_MAXINTERVAL",
	"TYK_GW_OPENTELEMETRY_METRICS_RETRY_MAXELAPSEDTIME",
	"TYK_GW_OPENTELEMETRY_METRICS_CARDINALITYLIMIT",
}

// pumpOnlyOpenTelemetryMetricsEnv are the Pump settings the Gateway does not
// have. deployment_environment and health_metrics are shared with MDCB and the
// Dashboard.
var pumpOnlyOpenTelemetryMetricsEnv = []string{
	"TYK_PMP_OPENTELEMETRY_METRICS_DEPLOYMENTENVIRONMENT",
	"TYK_PMP_OPENTELEMETRY_METRICS_HEALTHMETRICS",
}

// TestOpenTelemetryEnvVarParity is the guard for Gateway env-name parity. It
// reflects over the config struct, derives every env var name the way
// envconfig does, and requires the result to be exactly the Gateway's names
// with TYK_GW_ swapped for TYK_PMP_, plus the Pump-only settings.
func TestOpenTelemetryEnvVarParity(t *testing.T) {
	cfgType := reflect.TypeOf(TykPumpConfiguration{})
	field, ok := cfgType.FieldByName("OpenTelemetry")
	require.True(t, ok, "TykPumpConfiguration.OpenTelemetry must exist")
	got := envconfigNames(ENV_PREVIX+"_OPENTELEMETRY", field.Type)

	want := append([]string{}, pumpOnlyOpenTelemetryMetricsEnv...)
	for _, gw := range gatewayOpenTelemetryMetricsEnv {
		want = append(want, "TYK_PMP_"+strings.TrimPrefix(gw, "TYK_GW_"))
	}
	sort.Strings(got)
	sort.Strings(want)
	assert.Equal(t, want, got, "the Pump's opentelemetry env vars must be the Gateway's with TYK_GW_ replaced by TYK_PMP_")

	t.Run("every derived name is read", func(t *testing.T) {
		// The spelled-out names drive envconfig end to end, so the reflection
		// above cannot drift from what envconfig really reads.
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_ENABLED", "true")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_EXPORTER", "http")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_ENDPOINT", "otel-collector:4318")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_HEADERS", "x-team:platform")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_CONNECTIONTIMEOUT", "3")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_RESOURCENAME", "pump-env")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_TLS_ENABLE", "true")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_TLS_INSECURESKIPVERIFY", "true")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_TLS_CAFILE", "/ca.pem")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_TLS_CERTFILE", "/cert.pem")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_TLS_KEYFILE", "/key.pem")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_TLS_MAXVERSION", "1.3")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_TLS_MINVERSION", "1.2")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_EXPORTINTERVAL", "15")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_TEMPORALITY", "delta")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_SHUTDOWNTIMEOUT", "9")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_RETRY_ENABLED", "false")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_RETRY_INITIALINTERVAL", "100")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_RETRY_MAXINTERVAL", "200")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_RETRY_MAXELAPSEDTIME", "300")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_CARDINALITYLIMIT", "50")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_DEPLOYMENTENVIRONMENT", "staging")
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_HEALTHMETRICS", "false")

		cfg := &TykPumpConfiguration{}
		require.NoError(t, envconfig.Process(ENV_PREVIX, cfg))

		m := cfg.OpenTelemetry.Metrics
		assert.True(t, cfg.OpenTelemetry.MetricsEnabled())
		assert.Equal(t, "http", m.Exporter)
		assert.Equal(t, "otel-collector:4318", m.Endpoint)
		assert.Equal(t, map[string]string{"x-team": "platform"}, m.Headers)
		assert.Equal(t, 3, m.ConnectionTimeout)
		assert.Equal(t, "pump-env", m.ResourceName)
		assert.True(t, m.TLS.Enable)
		assert.True(t, m.TLS.InsecureSkipVerify)
		assert.Equal(t, "/ca.pem", m.TLS.CAFile)
		assert.Equal(t, "/cert.pem", m.TLS.CertFile)
		assert.Equal(t, "/key.pem", m.TLS.KeyFile)
		assert.Equal(t, "1.3", m.TLS.MaxVersion)
		assert.Equal(t, "1.2", m.TLS.MinVersion)
		assert.Equal(t, 15, m.ExportInterval)
		assert.Equal(t, "delta", m.Temporality)
		assert.Equal(t, 9, m.ShutdownTimeout)
		if assert.NotNil(t, m.Retry.Enabled) {
			assert.False(t, *m.Retry.Enabled)
		}
		assert.Equal(t, 100, m.Retry.InitialInterval)
		assert.Equal(t, 200, m.Retry.MaxInterval)
		assert.Equal(t, 300, m.Retry.MaxElapsedTime)
		assert.Equal(t, 50, m.CardinalityLimit)
		assert.Equal(t, "staging", m.DeploymentEnvironment)
		assert.False(t, cfg.OpenTelemetry.HealthMetricsEnabled())
	})
}

func TestOpenTelemetryConfig_HealthMetrics(t *testing.T) {
	load := func(t *testing.T, json string) otel.OpenTelemetry {
		t.Helper()
		path := writeConfigFile(t, json)
		cfg := &TykPumpConfiguration{}
		LoadConfig(&path, cfg)
		return cfg.OpenTelemetry
	}

	t.Run("omitted means on", func(t *testing.T) {
		c := load(t, `{"opentelemetry": {"metrics": {"enabled": true}}}`)
		assert.Nil(t, c.Metrics.HealthMetrics)
		assert.True(t, c.HealthMetricsEnabled())
	})

	t.Run("false in the file turns the family off", func(t *testing.T) {
		c := load(t, `{"opentelemetry": {"metrics": {"enabled": true, "health_metrics": false}}}`)
		assert.False(t, c.HealthMetricsEnabled())
		assert.True(t, c.MetricsEnabled(), "only the family is off")
	})

	t.Run("metrics off wins", func(t *testing.T) {
		c := load(t, `{"opentelemetry": {"metrics": {"enabled": false, "health_metrics": true}}}`)
		assert.False(t, c.HealthMetricsEnabled())
	})

	t.Run("env false overrides file true", func(t *testing.T) {
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_HEALTHMETRICS", "false")
		c := load(t, `{"opentelemetry": {"metrics": {"enabled": true, "health_metrics": true}}}`)
		assert.False(t, c.HealthMetricsEnabled())
	})

	t.Run("env true overrides file false", func(t *testing.T) {
		t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_HEALTHMETRICS", "true")
		c := load(t, `{"opentelemetry": {"metrics": {"enabled": true, "health_metrics": false}}}`)
		assert.True(t, c.HealthMetricsEnabled())
	})
}

// envconfigNames returns the env var names envconfig derives for every leaf
// field of t under prefix: PREFIX_FIELDNAME upper-cased, nested structs add
// their field name, embedded structs add nothing.
func envconfigNames(prefix string, t reflect.Type) []string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		key := prefix + "_" + strings.ToUpper(f.Name)
		ft := f.Type
		if ft.Kind() == reflect.Ptr && ft.Elem().Kind() == reflect.Struct {
			ft = ft.Elem()
		}
		switch {
		case ft.Kind() == reflect.Struct && f.Anonymous:
			names = append(names, envconfigNames(prefix, ft)...)
		case ft.Kind() == reflect.Struct:
			names = append(names, envconfigNames(key, ft)...)
		default:
			names = append(names, key)
		}
	}
	return names
}

func TestOpenTelemetryEnv_DoesNotCreatePumps(t *testing.T) {
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_ENABLED", "true")
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_EXPORTER", "grpc")
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_ENDPOINT", "otel-collector:4317")
	t.Setenv("TYK_PMP_OPENTELEMETRY_METRICS_DEPLOYMENTENVIRONMENT", "e2e")

	cfg := &TykPumpConfiguration{}
	require.NoError(t, cfg.LoadPumpsByEnv())

	assert.Empty(t, cfg.Pumps, "TYK_PMP_OPENTELEMETRY_* must not be read as pump definitions")
}
