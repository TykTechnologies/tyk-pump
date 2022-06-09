package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		AnalyticsStorageType: "test",
	}
	os.Setenv(ENV_PREVIX+"_OMITCONFIGFILE", "true")
	defaultPath := ""
	LoadConfig(&defaultPath, &config)

	assert.Equal(t, "", config.AnalyticsStorageType, "TYK_OMITCONFIGFILE should have unset the configuation")

	os.Unsetenv(ENV_PREVIX + "_OMITCONFIGFILE")

	config = TykPumpConfiguration{}
	config.AnalyticsStorageType = "test"
	LoadConfig(&defaultPath, &config)

	assert.Equal(t, "test", config.AnalyticsStorageType, "TYK_OMITCONFIGFILE should not have unset the configuation")

}
