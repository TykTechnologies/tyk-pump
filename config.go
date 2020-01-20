package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/kelseyhightower/envconfig"

	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/storage"
)

const ENV_PREVIX = "TYK_PMP"

type PumpConfig struct {
	Name    string                 `json:"name"` // Deprecated
	Type    string                 `json:"type"`
	Timeout int                    `json:"timeout"`
	Meta    map[string]interface{} `json:"meta"` // TODO: convert this to json.RawMessage and use regular json.Unmarshal
}

type TykPumpConfiguration struct {
	PurgeDelay             int64                      `json:"purge_delay"`
	PurgeChunk             int64                      `json:"purge_chunk"`
	DontPurgeUptimeData    bool                       `json:"dont_purge_uptime_data"`
	UptimePumpConfig       pumps.MongoConf            `json:"uptime_pump_config"`
	Pumps                  map[string]PumpConfig      `json:"pumps"`
	AnalyticsStorageType   string                     `json:"analytics_storage_type"`
	AnalyticsStorageConfig storage.RedisStorageConfig `json:"analytics_storage_config"`
	StatsdConnectionString string                     `json:"statsd_connection_string"`
	StatsdPrefix           string                     `json:"statsd_prefix"`
	LogLevel               string                     `json:"log_level"`
	HealthEndpoint         string                     `json:"health_endpoint"`
	HealthPort             int                        `json:"health_port"`
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
