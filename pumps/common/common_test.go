package common

import (
	"os"
	"testing"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/stretchr/testify/assert"
)

func TestGetName(t *testing.T) {
	p := &Pump{}

	assert.Equal(t, "CommonPump", p.GetName())
}
func TestEnvPrefix(t *testing.T) {
	p := &Pump{}

	assert.Equal(t, "", p.GetEnvPrefix())
	p.envPrefix = "test"

	assert.Equal(t, "test", p.GetEnvPrefix())
}

func TestFilters(t *testing.T) {
	p := &Pump{}

	assert.Equal(t, analytics.AnalyticsFilters{}, p.filters)

	filters := analytics.AnalyticsFilters{
		APIIDs:  []string{"test", "test2"},
		OrgsIDs: []string{"org1", "org2"},
	}
	p.SetFilters(filters)
	assert.NotEqual(t, analytics.AnalyticsFilters{}, p.filters)

	actualFilters := p.GetFilters()
	assert.Equal(t, filters, actualFilters)
}

func TestTimeout(t *testing.T) {
	p := &Pump{}

	assert.Equal(t, 0, p.Timeout)

	p.SetTimeout(10)

	actualTimeout := p.GetTimeout()
	assert.Equal(t, 10, actualTimeout)
}

func TestOmitDetailedRecording(t *testing.T) {
	p := &Pump{}

	assert.Equal(t, false, p.OmitDetailedRecording)

	p.SetOmitDetailedRecording(true)

	actualOmitDetailedRecording := p.GetOmitDetailedRecording()
	assert.Equal(t, true, actualOmitDetailedRecording)
}

func TestMaxRecordSize(t *testing.T) {
	p := &Pump{}

	assert.Equal(t, 0, p.maxRecordSize)

	p.SetMaxRecordSize(10)

	actualSize := p.GetMaxRecordSize()
	assert.Equal(t, 10, actualSize)
}

func TestShutdown(t *testing.T) {
	p := &Pump{}

	err := p.Shutdown()
	assert.Equal(t, nil, err)
}

func TestProcessEnvVars(t *testing.T) {
	type config struct {
		Var string
	}

	tcs := []struct {
		testName string

		config config
		envs   map[string]string

		customEnvPrefix  string
		defaultEnvPrefix string

		expectedVar string
	}{
		{
			testName: "Default env prefix",
			config:   config{"base_value"},
			envs:     map[string]string{"TEST_VAR": "env_test_var_value"},

			customEnvPrefix:  "",
			defaultEnvPrefix: "TEST",

			expectedVar: "env_test_var_value",
		},
		{
			testName: "custom env prefix",
			config:   config{"base_value"},
			envs:     map[string]string{"AUX_TEST_VAR": "env_aux_test_var_value", "TEST_VAR": "env_test_var_value"},

			customEnvPrefix:  "AUX_TEST",
			defaultEnvPrefix: "",

			expectedVar: "env_aux_test_var_value",
		},
		{
			testName: "custom+default env prefix",
			config:   config{"base_value"},
			envs:     map[string]string{"AUX_TEST_VAR": "env_aux_test_var_value", "TEST_VAR": "env_test_var_value"},

			customEnvPrefix:  "AUX_TEST",
			defaultEnvPrefix: "TEST",

			expectedVar: "env_aux_test_var_value",
		},
		{
			testName: "custom+default different than set env prefix",
			config:   config{"base_value"},
			envs:     map[string]string{"AUX_TEST_VAR": "env_aux_test_var_value", "TEST_VAR": "env_test_var_value"},

			customEnvPrefix:  "AUX_TEST_2",
			defaultEnvPrefix: "TEST_2",

			expectedVar: "base_value",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			p := &Pump{envPrefix: tc.customEnvPrefix}
			log := logger.GetLogger()
			p.Log = log.WithField("prefix", "common")

			for env, val := range tc.envs {
				os.Setenv(env, val)
			}
			defer func() {
				for env := range tc.envs {
					os.Unsetenv(env)
				}
			}()

			cfg := &tc.config
			p.ProcessEnvVars(p.Log, cfg, tc.defaultEnvPrefix)

			assert.Equal(t, tc.expectedVar, cfg.Var)

		})
	}
}
