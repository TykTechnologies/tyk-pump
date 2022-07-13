package pumps

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSVPump_Init(t *testing.T) {

	tcs := []struct {
		testName string

		envVars map[string]string
		config  map[string]interface{}

		expectedCSVDir string
	}{
		{
			testName:       "Config file-  csvdir",
			config:         map[string]interface{}{"csv_dir": "test1"},
			expectedCSVDir: "test1",
		},
		{
			testName:       "Env vars- csvdir",
			config:         map[string]interface{}{},
			envVars:        map[string]string{csvDefaultENV + "_CSVDIR": "test2"},
			expectedCSVDir: "test2",
		},
		{
			testName:       "Config file + Env vars - csvdir",
			config:         map[string]interface{}{"csv_dir": "test4"},
			envVars:        map[string]string{csvDefaultENV + "_CSVDIR": "test5"},
			expectedCSVDir: "test5",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			pmp := &CSVPump{}
			assert.Equal(t, "CSV Pump", pmp.GetName())

			for env, val := range tc.envVars {
				os.Setenv(env, val)
			}
			defer func() {
				for env := range tc.envVars {
					os.Unsetenv(env)
				}
			}()

			err := pmp.Init(tc.config)
			assert.Nil(t, err)
			assert.NotNil(t, pmp.csvConf)
			assert.Equal(t, tc.expectedCSVDir, pmp.csvConf.CSVDir)
		})
	}
}
