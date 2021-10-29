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
	// This feature adds a new configuration field in each pump called filters and its structure is the following:
	// ```json
	// "filters":{
	//   "api_ids":[],
	//   "org_ids":[],
	//   "response_codes":[],
	//   "skip_api_ids":[],
	//   "skip_org_ids":[],
	//   "skip_response_codes":[]
	// }
	// ```
	// The fields api_ids, org_ids and response_codes works as allow list (APIs and orgs where we want to send the analytics records) and the fields skip_api_ids, skip_org_ids and skip_response_codes works as block list.

	// The priority is always block list configurations over allow list.

	// An example of configuration would be:
	// ```json
	// "csv": {
	//  "type": "csv",
	//  "filters": {
	//    "org_ids": ["org1","org2"]
	//  },
	//  "meta": {
	//    "csv_dir": "./bar"
	//  }
	// }
	// ```
	Filters               analytics.AnalyticsFilters `json:"filters"`
	// You can configure a different timeout for each pump with the configuration option `timeout`. Its default value is 0 seconds, which means that the pump will wait for the writing operation forever. 

	// An example of this configuration would be:
	// ```json
	// "mongo": {
	//   "type": "mongo",
	//   "timeout":5,
	//   "meta": {
	//     "collection_name": "tyk_analytics",
	//     "mongo_url": "mongodb://username:password@{hostname:port},{hostname:port}/{db_name}"
	//   }
	// }
	// ```
	//
	// In case that any pump doesn't have a configured timeout, and it takes more seconds to write than the value configured for the purge loop in the `purge_delay` config option, you will see the following warning message: `Pump PMP_NAME is taking more time than the value configured of purge_delay. You should try to set a timeout for this pump.`. 
	//
	// In case that you have a configured timeout, but it still takes more seconds to write than the value configured for the purge loop in the `purge_delay` config option, you will see the following warning message: `Pump PMP_NAME is taking more time than the value configured of purge_delay. You should try lowering the timeout configured for this pump.`. 
	Timeout               int                        `json:"timeout"`
	OmitDetailedRecording bool                       `json:"omit_detailed_recording"`
	MaxRecordSize         int                        `json:"max_record_size"` // in bytes
	Meta                  map[string]interface{}     `json:"meta"`            // TODO: convert this to json.RawMessage and use regular json.Unmarshal
}

type UptimeConf struct {
	// TYKCONFIGEXPAND
	pumps.MongoConf
	// TYKCONFIGEXPAND
	pumps.SQLConf
	UptimeType string `json:"uptime_type"`
}

type TykPumpConfiguration struct {
	// The number of seconds the Pump waits between checking for analytics data and purge it from Redis.
	PurgeDelay              int                        `json:"purge_delay"`
	// The maximum number of records to pull from Redis at a time. If it's unset or 0, all the analytics records in Redis are pulled. If it's setted, `storage_expiration_time` is used to reset the analytics record TTL.
	PurgeChunk              int64                      `json:"purge_chunk"`
	// The number of seconds for the analytics records TTL. It only works if `purge_chunk` is enabled. Defaults to 60 seconds.
	StorageExpirationTime   int64                      `json:"storage_expiration_time"`
	// Setting this to false will create a pump that pushes uptime data to Uptime Pump, so the Dashboard can read it. Disable by setting to true
	DontPurgeUptimeData     bool                       `json:"dont_purge_uptime_data"`
	UptimePumpConfig        UptimeConf                 `json:"uptime_pump_config"`
	Pumps                   map[string]PumpConfig      `json:"pumps"`
	AnalyticsStorageType    string                     `json:"analytics_storage_type"`
	AnalyticsStorageConfig  storage.RedisStorageConfig `json:"analytics_storage_config"`
	StatsdConnectionString  string                     `json:"statsd_connection_string"`
	StatsdPrefix            string                     `json:"statsd_prefix"`
	// Set the logger details for tyk-pump. The posible values are: `info`,`debug`,`error` and `warn`. By default, the log level is `info`. 
	LogLevel                string                     `json:"log_level"`
	// Set the logger format. The possible values are: `text` and `json`. By default, the log format is `text`.
	LogFormat               string                     `json:"log_format"`
	HealthCheckEndpointName string                     `json:"health_check_endpoint_name"`
	HealthCheckEndpointPort int                        `json:"health_check_endpoint_port"`
	// Setting this to true will avoid writing raw_request and raw_response fields for each request in pumps. Defaults to false.
	OmitDetailedRecording   bool                       `json:"omit_detailed_recording"`
	// `max_record_size` defines maximum size (in bytes) for Raw Request and Raw Response logs, this value defaults to 0. Is not set then tyk-pump will not trim any data and will store the full information.
	// This can also be set at a pump level. For example:
	// ```{.json}
	// "csv": {
	//   "type": "csv",
	//   "max_record_size":1000,
	//   "meta": {
	//     "csv_dir": "./"
	//   }
	// }
	// ```
	MaxRecordSize           int                        `json:"max_record_size"` // in bytes
	OmitConfigFile          bool                       `json:"omit_config_file"`
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

	shouldOmit, omitEnvExist := os.LookupEnv(ENV_PREVIX + "_OMITCONFIGFILE")
	if configStruct.OmitConfigFile || (omitEnvExist && strings.ToLower(shouldOmit) == "true") {
		*configStruct = TykPumpConfiguration{}
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
