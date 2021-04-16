package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/kelseyhightower/envconfig"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/storage"
)

const ENV_PREVIX = "TYK_PMP"
const PUMPS_ENV_PREFIX = pumps.PUMPS_ENV_PREFIX

type PumpConfig struct {
	Name                  string                     `json:"name"` // Deprecated
	Type                  string                     `json:"type"`
	Filters               analytics.AnalyticsFilters `json:"filters"`
	Timeout               int                        `json:"timeout"`
	OmitDetailedRecording bool                       `json:"omit_detailed_recording"`
	Meta                  map[string]interface{}     `json:"meta"` // TODO: convert this to json.RawMessage and use regular json.Unmarshal
}
type PumpConfigs map[string]PumpConfig

type TykPumpConfiguration struct {
	PurgeDelay              int                        `json:"purge_delay"`
	PurgeChunk              int64                      `json:"purge_chunk"`
	StorageExpirationTime   int64                      `json:"storage_expiration_time"`
	DontPurgeUptimeData     bool                       `json:"dont_purge_uptime_data"`
	UptimePumpConfig        map[string]interface{}     `json:"uptime_pump_config"`
	Pumps                   map[string]PumpConfig      `json:"pumps" `
	AnalyticsStorageType    string                     `json:"analytics_storage_type"`
	AnalyticsStorageConfig  storage.RedisStorageConfig `json:"analytics_storage_config"`
	StatsdConnectionString  string                     `json:"statsd_connection_string"`
	StatsdPrefix            string                     `json:"statsd_prefix"`
	LogLevel                string                     `json:"log_level"`
	HealthCheckEndpointName string                     `json:"health_check_endpoint_name"`
	HealthCheckEndpointPort int                        `json:"health_check_endpoint_port"`
	OmitDetailedRecording   bool                       `json:"omit_detailed_recording"`
}

func LoadConfig(filePath *string, configStruct *TykPumpConfiguration) {

	configuration, err := ioutil.ReadFile(*filePath)
	if err != nil {
		log.Error("Couldn't load configuration file: ", err)
	}

	marshalErr := json.Unmarshal(configuration, &configStruct)
	if marshalErr != nil {
		log.Error("Couldn't unmarshal configuration: ", marshalErr)
	}

	overrideErr := envconfig.Process(ENV_PREVIX, configStruct)
	if overrideErr != nil {
		log.Error("Failed to process environment variables after file load: ", overrideErr)
	}

	errLoadEnvPumps := configStruct.LoadPumpsByEnv()
	if errLoadEnvPumps != nil {
		log.Fatal(err)
	}
}

func (cfg *TykPumpConfiguration) LoadPumpsByEnv() error {
	if len(cfg.Pumps) == 0 {
		cfg.Pumps = make(map[string]PumpConfig)
	}
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, PUMPS_ENV_PREFIX) {

			// We trim everything after PUMPS_ENV_PREFIX. For example, if we have TYK_PUMP_PUMPS_CSV_TYPE we would have CSV_TYPE here
			envWoPrefix := strings.TrimPrefix(env, PUMPS_ENV_PREFIX+"_")

			//We split everything after the trim to have an slice with the keywords
			envSplit := strings.Split(envWoPrefix, "_")
			if len(envSplit) < 2 {
				return errors.New(fmt.Sprintf("Problem reading env variable %v", env))
			}

			//The name of the pump is always going to be the first keyword after the PUMPS_ENV_PREFIX
			pmpName := envSplit[0]

			pmp := PumpConfig{}

			//First we check if the config json already have this pump
			if _, ok := cfg.Pumps[pmpName]; ok {
				//check if the pump has any meta field. It should if it's already declared but just in case
				if len(pmp.Meta) == 0 {
					pmp.Meta = make(map[string]interface{})
				}

				pmp.Meta["env_prefix"] = PUMPS_ENV_PREFIX + "_" + pmpName

				continue
			}
			//If the config don't have this pump declared BUT the name is equal to one of our available pump names, we add it to the config.
			if _, ok := pumps.AvailablePumps[strings.ToLower(pmpName)]; ok {
				pmp.Type = strings.ToLower(pmpName)
				pmp.Meta = make(map[string]interface{})
				pmp.Meta["env_prefix"] = PUMPS_ENV_PREFIX + "_" + pmpName

				cfg.Pumps[pmpName] = pmp
				continue
			}

			foundType := false
			//If the pump name is not declared nor an available pump name, we look for a TYPE to create that pump.
			for _, pmpCfg := range envSplit[1:] {
				if strings.HasPrefix(pmpCfg, "TYPE") {
					//Env var set validation. An example of a valid input would be CSVTEST_TYPE=CSV.
					pmpType := strings.Split(pmpCfg, "=")
					if len(pmpType) < 2 {
						return errors.New(fmt.Sprintf("TYPE present but not set for %v", pmpName))
					}

					//We check here if that TYPE exists.
					_, err := pumps.GetPumpByName(strings.ToLower(pmpType[1]))
					if err != nil {
						log.Warnf("Problem creating pump of type %v", pmpType[1])
						break
					}

					pmp.Type = strings.ToLower(pmpType[1])
					pmp.Meta = make(map[string]interface{})
					pmp.Meta["env_prefix"] = PUMPS_ENV_PREFIX + "_" + pmpName

					foundType = true
					break
				}
			}
			if foundType {
				cfg.Pumps[pmpName] = pmp
			}
		}
	}
	return nil
}
