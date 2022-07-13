package pumps

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSVPump_Init(t *testing.T) {

	assert.Equal(t, "CSV Pump", pmp.GetName())

	tcs := []struct{
		testName string

		envVars map[string]string
		config map[string]interface{}

		expectedCSVDir string
	}{
		{
			testName: "Config file csvdir",
			config: map[string]interface{}{"csv_dir":"test1"},
			expectedCSVDir: "test1",
		},
		{
			testName: "env var csvdir",
			config: map[string]interface{}{"csv_dir":"test1"},
			expectedCSVDir: "test1",
		},
	}

	for _,tc  := range tcs{
		t.Run(tc.testName, func(t *testing.T) {
			pmp := &CSVPump{}

			for env, val := range tc.envVars{
				os.Setenv(env,val)
			}
			defer func(){
				for env, _ := range tc.envVars{
					os.Unsetenv(env)
				}
			}()

			err := pmp.Init(tc.config)
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedCSVDir, pmp.csvConf)
		})
	}
	cfg := make(map[string]interface{})
	cfg["csv_dir"] = "test1"
	err := pmp.Init(cfg)
	assert.Nil(t, err)
	assert.Equal(t, "test1", pmp.csvConf.CSVDir)



	os.Setenv(csvDefaultENV+"_CSVDIR", "test2")
	os.Unsetenv(dummyDefaultENV + "_CSVDIR")
	err = pmp.Init(cfg)
	assert.Nil(t, err)
	assert.Equal(t, "test2", pmp.csvConf.CSVDir)

}
