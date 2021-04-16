package main

import (
	"fmt"
	"os"
	"testing"
)

func TestConfigEnv(t *testing.T) {

	pumpNameCSV := "CSV"
	pumpNameTest := "TEST"
	os.Setenv(PUMPS_ENV_PREFIX+"_"+pumpNameCSV+"_DIR", "/TEST")
	os.Setenv(PUMPS_ENV_PREFIX+"_"+pumpNameTest+"_TYPE", "CSV")

	defer os.Unsetenv(PUMPS_ENV_PREFIX + "_" + pumpNameCSV + "_DIR")
	defer os.Unsetenv(PUMPS_ENV_PREFIX + "_" + pumpNameTest + "_TYPE")

	cfg := &TykPumpConfiguration{}
	cfg.Pumps = make(map[string]PumpConfig)
	cfg.Pumps["CSVTEST2"] = PumpConfig{}

	defaultPath := ""
	LoadConfig(&defaultPath, cfg)

	if len(cfg.Pumps) != 3 {
		t.Error(fmt.Sprintf("Pump config should have 3 pumps created and it has %v.", len(cfg.Pumps)))
	}

	if _, ok := cfg.Pumps[pumpNameTest]; !ok {
		t.Error("Pump should have a pump called " + pumpNameTest)
	}

	if cfg.Pumps[pumpNameTest].Type != "csv" {
		t.Error(pumpNameTest + " Pump TYPE should be csv")
	}

	if val, ok := cfg.Pumps[pumpNameTest].Meta["env_prefix"]; !ok || val != PUMPS_ENV_PREFIX+"_"+pumpNameTest {
		t.Error(pumpNameTest + " Pump should have a meta tag with the env prefix set and it should be " + PUMPS_ENV_PREFIX + "_" + pumpNameTest)
	}

	if _, ok := cfg.Pumps[pumpNameCSV]; !ok {
		t.Error("Pump should have a pump called " + pumpNameCSV)
	}

	if cfg.Pumps[pumpNameCSV].Type != "csv" {
		t.Error(pumpNameCSV + " Pump TYPE should be csv")
	}

	if val, ok := cfg.Pumps[pumpNameCSV].Meta["env_prefix"]; !ok || val != PUMPS_ENV_PREFIX+"_"+pumpNameCSV {
		t.Error(pumpNameCSV + " Pump should have a meta tag with the env prefix set and it should be " + PUMPS_ENV_PREFIX + "_" + pumpNameCSV)
	}
}
