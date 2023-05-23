package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToUpperPumps(t *testing.T) {
	pumpNames := []string{"test1", "test2", "TEST3", "Test4", "test3"} // index 4 must override index 2

	initialConfig := &TykPumpConfiguration{
		Pumps: map[string]PumpConfig{
			pumpNames[0]: {
				Type: "mongo",
				Name: "mongo-pump",
				Meta: map[string]interface{}{
					"meta_env_prefix": "test",
				},
			},
			pumpNames[1]: {
				Type: "sql",
				Name: "sql-pump",
				Meta: map[string]interface{}{
					"meta_env_prefix": "test2",
				},
			},
			pumpNames[2]: {
				Type: "mongo",
			},
			pumpNames[3]: {
				Type: "sql",
			},
			pumpNames[4]: {
				Type: "sql",
			},
		},
	}
	defaultPath := ""
	LoadConfig(&defaultPath, initialConfig)
	assert.Equal(t, len(pumpNames)-1, len(initialConfig.Pumps))
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[0])].Type, "mongo")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[1])].Type, "sql")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[2])].Type, "sql")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[3])].Type, "sql")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[0])].Name, "mongo-pump")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[1])].Name, "sql-pump")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[0])].Meta["meta_env_prefix"], "test")
	assert.Equal(t, initialConfig.Pumps[strings.ToUpper(pumpNames[1])].Meta["meta_env_prefix"], "test2")
	// Check if the pumps with lower case are empty (don't appear in the map)
	assert.Equal(t, initialConfig.Pumps[pumpNames[0]], PumpConfig{})
	assert.Equal(t, initialConfig.Pumps[pumpNames[1]], PumpConfig{})
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
	config := TykPumpConfiguration{
		PurgeDelay: 10,
	}
	os.Setenv(ENV_PREVIX+"_OMITCONFIGFILE", "true")
	defaultPath := ""
	LoadConfig(&defaultPath, &config)

	assert.Equal(t, 0, config.PurgeDelay, "TYK_OMITCONFIGFILE should have unset the configuation")

	os.Unsetenv(ENV_PREVIX + "_OMITCONFIGFILE")

	config = TykPumpConfiguration{}
	config.PurgeDelay = 30
	LoadConfig(&defaultPath, &config)

	assert.Equal(t, 30, config.PurgeDelay, "TYK_OMITCONFIGFILE should not have unset the configuation")
}
