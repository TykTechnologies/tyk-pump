package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
