package main

import (
	"encoding/json"
	"github.com/kelseyhightower/envconfig"
	"io/ioutil"
	"github.com/TykTechnologies/tyk-pump/storage"
)

const ENV_PREVIX string = "TYK_PMP"

type PumpConfig struct {
	Name string                 `json:"name"`
	Meta map[string]interface{} `json:"meta"`
}

type TykPumpConfiguration struct {
	PurgeDelay             int                                  `json:"purge_delay"`
	DontPurgeUptimeData    bool                                 `json:"dont_purge_uptime_data"`
	UptimePumpConfig       interface{}                          `json:"uptime_pump_config"`
	Pumps                  map[string]PumpConfig                `json:"pumps"`
	AnalyticsStorageType   string                               `json:"analytics_storage_type"`
	AnalyticsStorageConfig storage.RedisStorageConfig           `json:"analytics_storage_config"`
}

func LoadConfig(filePath *string, configStruct *TykPumpConfiguration) {
	configuration, err := ioutil.ReadFile(*filePath)
	if err != nil {
		log.Fatal("Couldn't load configuration file: ", err)

	}

	marshalErr := json.Unmarshal(configuration, &configStruct)
	if marshalErr != nil {
		log.Fatal("Couldn't unmarshal configuration: ", marshalErr)
	}

	overrideErr := envconfig.Process(ENV_PREVIX, configStruct)
	if overrideErr != nil {
		log.Error("Failed to process environment variables after file load: ", overrideErr)
	}
}
