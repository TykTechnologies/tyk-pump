package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"io/ioutil"

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

type TykPumpConfiguration struct {
	PurgeDelay              int                        `json:"purge_delay"`
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
}

func LoadConfig(filePath *string, configStruct *TykPumpConfiguration) error {
	configuration, err := ioutil.ReadFile(*filePath)
	if err != nil {
		return errors.New(fmt.Sprintf("Couldn't load configuration file: %s", err))
	}

	marshalErr := json.Unmarshal(configuration, &configStruct)
	if marshalErr != nil {
		return errors.New(fmt.Sprintf("Couldn't unmarshal configuration: %s", marshalErr))

	}

	overrideErr := envconfig.Process(ENV_PREVIX, configStruct)
	if overrideErr != nil {
		return errors.New(fmt.Sprintf("Failed to process environment variables after file load: %s", overrideErr))
	}
	return nil
}
