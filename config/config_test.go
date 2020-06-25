package config

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {

	config := TykPumpConfiguration{}

	tmpFile, err := ioutil.TempFile(os.TempDir(), "tmp-conf.json")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(tmpFile.Name())
	confPath := tmpFile.Name()

	// Example writing to the file
	confJson := []byte("{  \"analytics_storage_type\": \"redis\"\n}")
	if _, err = tmpFile.Write(confJson); err != nil {
		t.Fatal("Failed to write to temporary file", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal("Failed to close the temporary file", err)
	}

	errConfig := LoadConfig(&confPath, &config)

	//Testing correctly conf load
	if config.AnalyticsStorageType != "redis" || errConfig != nil {
		t.Fatal("AnalyticsStorageType should be redis")
	}

	nonExistingPath := "/non-existing-path"
	errConfig = LoadConfig(&nonExistingPath, &config)
	if errConfig == nil {
		t.Fatal("LoadConfig should fail trying to load a non-existing-path.")
	}

	os.Setenv(ENV_PREVIX+"_PURGEDELAY", "'1'")
	defer os.Setenv(ENV_PREVIX+"_PURGEDELAY", "")
	errConfig = LoadConfig(&confPath, &config)
	if errConfig == nil {
		t.Fatal("LoadConfig should fail trying to load a string env var in a int env val.")
	}

	tmpFile2, err := ioutil.TempFile(os.TempDir(), "tmp-conf-2.json")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(tmpFile2.Name())
	confPath2 := tmpFile2.Name()
	errConfig = LoadConfig(&confPath2, &config)
	if errConfig == nil {
		t.Fatal("LoadConf should throw an error trying to unmarshal incorrect config file.")
	}

}
