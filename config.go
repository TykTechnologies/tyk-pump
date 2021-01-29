package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/kelseyhightower/envconfig"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/storage"
)

const ENV_PREVIX = "TYK_PMP"

type PumpConfig struct {
	Name                  string                     `json:"name"` // Deprecated
	Type                  string                     `json:"type"`
	Filters               analytics.AnalyticsFilters `json:"filters"`
	Timeout               int                        `json:"timeout"`
	OmitDetailedRecording bool                       `json:"omit_detailed_recording"`
	Meta                  map[string]interface{}     `json:"meta"` // TODO: convert this to json.RawMessage and use regular json.Unmarshal
}

type ObfuscateAuthHeader struct {
	ObfuscateKeys  bool   `json:"obfuscate_keys"`
	AuthHeaderName string `json:"auth_header_name"`
}

type TykPumpConfiguration struct {
	PurgeDelay              int                        `json:"purge_delay"`
	PurgeChunk              int64                      `json:"purge_chunk"`
	StorageExpirationTime   int64                      `json:"storage_expiration_time"`
	DontPurgeUptimeData     bool                       `json:"dont_purge_uptime_data"`
	UptimePumpConfig        map[string]interface{}     `json:"uptime_pump_config"`
	Pumps                   map[string]PumpConfig      `json:"pumps"`
	AnalyticsStorageType    string                     `json:"analytics_storage_type"`
	AnalyticsStorageConfig  storage.RedisStorageConfig `json:"analytics_storage_config"`
	StatsdConnectionString  string                     `json:"statsd_connection_string"`
	StatsdPrefix            string                     `json:"statsd_prefix"`
	LogLevel                string                     `json:"log_level"`
	HealthCheckEndpointName string                     `json:"health_check_endpoint_name"`
	HealthCheckEndpointPort int                        `json:"health_check_endpoint_port"`
	OmitDetailedRecording   bool                       `json:"omit_detailed_recording"`
	base64DecodeRawData			bool											 `json:"base64_decode_raw_data"`
	obfuscateAuthHeader		  ObfuscateAuthHeader
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
