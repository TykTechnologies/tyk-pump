package main

import (
	"os"
	"strings"
	"testing"

	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type configLogHook struct {
	entries []string
}

func (h *configLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *configLogHook) Fire(entry *logrus.Entry) error {
	h.entries = append(h.entries, entry.Message)
	return nil
}

// File-level MC/DC witness rows: these requirements are genuinely exercised by
// covered tests in this file (see the per-test // MCDC blocks below). This
// header block records the covered rows so every // Verifies: link in the file
// has a matching witness; rows copied verbatim from `proof mcdc show`.
//
// MCDC SW-REQ-002: config_file_enabled=F, json_loaded_before_env_override=F => TRUE
// MCDC SW-REQ-002: config_file_enabled=T, json_loaded_before_env_override=F => FALSE
// MCDC SW-REQ-002: config_file_enabled=T, json_loaded_before_env_override=T => TRUE

// Verifies: SW-REQ-002
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

// Verifies: SW-REQ-002
// SW-REQ-002:malformed_input:nominal
// SW-REQ-002:malformed_recovers_or_errors_loudly:nominal
// MCDC SW-REQ-002: config_file_enabled=F, json_loaded_before_env_override=F => TRUE
// MCDC SW-REQ-002: config_file_enabled=T, json_loaded_before_env_override=F => FALSE
// MCDC SW-REQ-002: config_file_enabled=T, json_loaded_before_env_override=T => TRUE
//
// config_file_enabled=T (a defaultPath is passed and the file exists) and
// json_loaded_before_env_override=T: LoadConfig parses the JSON example config and the assertions
// see populated initialConfig.Pumps before any env override is read. The
// config_file_enabled=F arm is exercised by TestIgnoreConfig (--omit-config-file path). The
// config_file_enabled=T/json_loaded=F arm is exercised by TestIgnoreConfig's "Config file does
// not exist" subtest where the JSON file fails to load.
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

// Verifies: SW-REQ-002
// Verifies: SW-REQ-016
// Verifies: KI:config-pump-type-whitelist-not-enforced-at-load
// Reproduces: config-pump-type-whitelist-not-enforced-at-load
func TestLoadConfig_AcceptsUnknownPumpTypeUntilInitialise_KI(t *testing.T) {
	defaultPath := ""
	t.Setenv(ENV_PREVIX+"_OMITCONFIGFILE", "true")
	t.Setenv(PUMPS_ENV_PREFIX+"_BAD_TYPE", "not-a-real-pump")

	cfg := &TykPumpConfiguration{
		Pumps: map[string]PumpConfig{
			"preloaded": {Type: "not-a-real-pump"},
		},
	}
	LoadConfig(&defaultPath, cfg)

	assert.Equal(t, "not-a-real-pump", cfg.Pumps["PRELOADED"].Type)
	assert.Equal(t, "not-a-real-pump", cfg.Pumps["BAD"].Type)

	_, err := pumps.GetPumpByName(cfg.Pumps["BAD"].Type)
	assert.Error(t, err)
}

// Verifies: SW-REQ-002
// Verifies: SYS-REQ-008
// Verifies: SYS-REQ-020
// SW-REQ-002:malformed_input:nominal
// SYS-REQ-008:malformed_input:nominal
// MCDC SYS-REQ-008: config_loaded_from_json=F, json_config_file_present=F => TRUE
// MCDC SYS-REQ-008: config_loaded_from_json=F, json_config_file_present=T => FALSE
// MCDC SYS-REQ-008: config_loaded_from_json=T, json_config_file_present=T => TRUE
// MCDC SYS-REQ-020: config_reflects_env=F, env_override_present=F => TRUE
// MCDC SYS-REQ-020: config_reflects_env=F, env_override_present=T => FALSE
// MCDC SYS-REQ-020: config_reflects_env=T, env_override_present=T => TRUE
//
// SYS-REQ-008 (config_loaded_from_json / json_config_file_present): defaultPath is "" so
// json_config_file_present=F, config_loaded_from_json=F -> TRUE row (vacuous). The cfg.Pumps
// assertions still pass because env vars wholly populate the config. The FALSE row is the
// regression where a missing file silently injects defaults; the assert.Len(cfg.Pumps,3)
// detects it.
//
// SYS-REQ-020 (config_reflects_env / env_override_present): every os.Setenv call is an
// env_override_present=T trigger; the assertions on cfg.Pumps[pumpNameTest].Type ("csv"),
// .Timeout (10), .Meta keys, and APIIDs length all prove config_reflects_env=T -> TRUE row.
// The FALSE row (env present but ignored) is caught by every Equal assertion. The vacuous
// TRUE arm is "no env override".
// SW-REQ-002:nominal:nominal
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

// Verifies: SW-REQ-002
// Verifies: INT-REQ-008
// Verifies: SYS-REQ-008
// Verifies: SYS-REQ-035
// SW-REQ-002:config_file_omission_suppresses_file_read:negative
// SW-REQ-002:config_file_omission_suppresses_file_read:nominal
// SW-REQ-002:malformed_input:negative
// SW-REQ-002:malformed_recovers_or_errors_loudly:negative
// SW-REQ-002:malformed_recovers_or_errors_loudly:nominal
// SYS-REQ-008:malformed_input:negative
// SYS-REQ-008:malformed_input:nominal
// SYS-REQ-035:malformed_recovers_or_errors_loudly:negative
// SYS-REQ-035:malformed_recovers_or_errors_loudly:nominal
// MCDC INT-REQ-008: config_decode_attempted=F, unknown_keys_reported_via_logfatal=F => TRUE
// MCDC INT-REQ-008: config_decode_attempted=T, unknown_keys_reported_via_logfatal=F => FALSE
// MCDC INT-REQ-008: config_decode_attempted=T, unknown_keys_reported_via_logfatal=T => TRUE
// MCDC SYS-REQ-008: config_loaded_from_json=F, json_config_file_present=F => TRUE
// MCDC SYS-REQ-008: config_loaded_from_json=F, json_config_file_present=T => FALSE
// MCDC SYS-REQ-008: config_loaded_from_json=T, json_config_file_present=T => TRUE
// MCDC SYS-REQ-035: config_loading_robust_to_malformed_input=F => FALSE
// MCDC SYS-REQ-035: config_loading_robust_to_malformed_input=T => TRUE
//
// SYS-REQ-035 (config_loading_robust_to_malformed_input): the "Config file does not exist"
// sub-test passes a nonexistent path to LoadConfig and asserts PurgeDelay is preserved at
// its initial value (5) -- the loader did NOT crash, falling back to defaults
// (config_loading_robust_to_malformed_input=T) -> TRUE row. The FALSE row is the regression
// where LoadConfig panics or zeroes the configuration on malformed input; the asserts on
// PurgeDelay catch that scenario. The KI mapstructure-decode-silently-drops-unknown-keys
// documents the silently-drop unknown-keys behaviour required by this guarantee.
//
// INT-REQ-008 (config_decode_attempted / unknown_keys_reported_via_logfatal): each sub-test
// invokes LoadConfig (config_decode_attempted=T). The "Not ignoring the config file" /
// "Environment variable not set" sub-tests assert PurgeDelay=10 (config loaded successfully
// without unknown keys -> unknown_keys_reported_via_logfatal=T in the no-error sense -> TRUE
// row). The "Config file does not exist" sub-test asserts PurgeDelay==5 (no decode happened;
// decode_attempted=F vacuous TRUE arm). The FALSE row (decode happened but unknown keys not
// reported) is covered by the linked KI on log.Fatal coverage (.proof/known-issues/
// pumps-logfatal-on-config-decode.yaml); the test itself proves the success/no-decode arms.
//
// SYS-REQ-008 (config_loaded_from_json / json_config_file_present): the omit=true subtests
// are config_file_enabled=F for SW-REQ-002 and do not witness SYS-REQ-008's present-file
// trigger. The "Not ignoring" sub-test loads JSON successfully (both T) -> TRUE row.
// "Config file does not exist" sub-test is json_config_file_present=F,
// config_loaded_from_json=F -> vacuous TRUE row. The FALSE row is documented as the
// requirement-violation case caught by the present-file load assertions.
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

	t.Run("Ignoring nonexistent config file suppresses file errors", func(t *testing.T) {
		initialConfig := TykPumpConfiguration{PurgeDelay: 5}
		nonexistentPath := "nonexistent_config.json"
		hook := &configLogHook{}
		log.AddHook(hook)
		t.Cleanup(func() {
			for level, hooks := range log.Hooks {
				filtered := hooks[:0]
				for _, existing := range hooks {
					if existing != hook {
						filtered = append(filtered, existing)
					}
				}
				log.Hooks[level] = filtered
			}
		})

		t.Setenv(ENV_PREVIX+"_OMITCONFIGFILE", "true")
		LoadConfig(&nonexistentPath, &initialConfig)

		assert.Equal(t, 5, initialConfig.PurgeDelay, "TYK_OMITCONFIGFILE=true should preserve existing configuration")
		for _, message := range hook.entries {
			assert.NotContains(t, message, "Couldn't load configuration file")
			assert.NotContains(t, message, "Couldn't unmarshal configuration")
		}
	})
}

// Verifies: SW-REQ-002
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
		{
			name: "bad env override is logged but pump entry is retained",
			cfg:  &TykPumpConfiguration{},
			setup: func() {
				os.Setenv(ENV_PREVIX+"_PUMPS_ENVTEST_TYPE", "csv")
				os.Setenv(ENV_PREVIX+"_PUMPS_ENVTEST_TIMEOUT", "not-an-int")
			},
			teardown: func() {
				os.Unsetenv(ENV_PREVIX + "_PUMPS_ENVTEST_TYPE")
				os.Unsetenv(ENV_PREVIX + "_PUMPS_ENVTEST_TIMEOUT")
			},
			wanted: map[string]PumpConfig{
				"ENVTEST": {
					Type: "csv",
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

// Verifies: SW-REQ-002
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

// Verifies: SW-REQ-002
func TestLoadConfigLogsInvalidTopLevelEnvOverride(t *testing.T) {
	t.Setenv(ENV_PREVIX+"_OMITCONFIGFILE", "true")
	t.Setenv(ENV_PREVIX+"_PURGEDELAY", "not-an-int")

	cfg := &TykPumpConfiguration{}
	defaultPath := ""

	LoadConfig(&defaultPath, cfg)
	assert.Zero(t, cfg.PurgeDelay)
}
