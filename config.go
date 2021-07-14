package main

import (
	"encoding/json"
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
const PUMPS_ENV_META_PREFIX = pumps.PUMPS_ENV_META_PREFIX

type PumpConfig struct {
	Name                  string                     `json:"name"` // Deprecated
	Type                  string                     `json:"type"`
	Filters               analytics.AnalyticsFilters `json:"filters"`
	Timeout               int                        `json:"timeout"`
	OmitDetailedRecording bool                       `json:"omit_detailed_recording"`
	Meta                  map[string]interface{}     `json:"meta"` // TODO: convert this to json.RawMessage and use regular json.Unmarshal
}

type UptimeConf struct {
	pumps.MongoConf
	pumps.SQLConf
	UptimeType string `json:"uptime_type"`
}

type TykPumpConfiguration struct {
	PurgeDelay              int                        `json:"purge_delay"`
	PurgeChunk              int64                      `json:"purge_chunk"`
	StorageExpirationTime   int64                      `json:"storage_expiration_time"`
	DontPurgeUptimeData     bool                       `json:"dont_purge_uptime_data"`
	UptimePumpConfig        UptimeConf                 `json:"uptime_pump_config"`
	Pumps                   map[string]PumpConfig      `json:"pumps"`
	AnalyticsStorageType    string                     `json:"analytics_storage_type"`
	AnalyticsStorageConfig  storage.RedisStorageConfig `json:"analytics_storage_config"`
	StatsdConnectionString  string                     `json:"statsd_connection_string"`
	StatsdPrefix            string                     `json:"statsd_prefix"`
	LogLevel                string                     `json:"log_level"`
	LogFormat               string                     `json:"log_format"`
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
		log.Fatal("error loading pumps env vars:", err)
	}
}

func (cfg *TykPumpConfiguration) LoadPumpsByEnv() error {
	if len(cfg.Pumps) == 0 {
		cfg.Pumps = make(map[string]PumpConfig)
	}

	osPumpsEnvNames := map[string]bool{}

	//first we look for all the pumps names in the env vars from the os
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, PUMPS_ENV_PREFIX) {

			// We trim everything after PUMPS_ENV_PREFIX. For example, if we have TYK_PUMP_PUMPS_CSV_TYPE we would have CSV_TYPE here
			envWoPrefix := strings.TrimPrefix(env, PUMPS_ENV_PREFIX+"_")

			//We split everything after the trim to have an slice with the keywords
			envSplit := strings.Split(envWoPrefix, "_")
			if len(envSplit) < 2 {
				log.Debug(fmt.Sprintf("Problem reading env variable %v", env))
				continue
			}

			//The name of the pump is always going to be the first keyword after the PUMPS_ENV_PREFIX
			pmpName := envSplit[0]

			osPumpsEnvNames[pmpName] = true
		}
	}

	//then we look for each pmpName specified in the env and try to initialise those pumps
	for pmpName := range osPumpsEnvNames {
		pmp := PumpConfig{}

		//First we check if the config json already have this pump
		if jsonPump, ok := cfg.Pumps[pmpName]; ok {
			//since the pump already exist in json, we try to override with env vars
			pmp = jsonPump
		}
		//We look if the pmpName is one of our available pumps. If it's not, we look if the env with the TYPE filed exists.
		if _, ok := pumps.AvailablePumps[strings.ToLower(pmpName)]; !ok {
			pmpType, found := os.LookupEnv(PUMPS_ENV_PREFIX + "_" + pmpName + "_TYPE")
			if !found {
				log.Error(fmt.Sprintf("TYPE Env var for pump %s not found", pmpName))
				continue
			}
			pmp.Type = pmpType
		} else {
			pmp.Type = pmpName
		}

		pmpType := pmp.Type
		//We fetch the env vars for that pump.
		overrideErr := envconfig.Process(PUMPS_ENV_PREFIX+"_"+pmpName, &pmp)
		if overrideErr != nil {
			log.Error("Failed to process environment variables for ", PUMPS_ENV_PREFIX+"_"+pmpName, " with err: ", overrideErr)
		}

		//init the meta map
		if len(pmp.Meta) == 0 {
			pmp.Meta = make(map[string]interface{})

		}
		//Add the meta env prefix for individual configurations
		pmp.Meta["meta_env_prefix"] = PUMPS_ENV_PREFIX + "_" + pmpName + PUMPS_ENV_META_PREFIX
		pmp.Type = strings.ToLower(pmpType)

		cfg.Pumps[pmpName] = pmp
	}
	return nil
}
