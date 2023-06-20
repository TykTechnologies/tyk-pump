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
	// Deprecated.
	Name string `json:"name"`
	// Sets the pump type. This is needed when the pump key does not equal to the pump name type.
	Type string `json:"type"`
	// This feature adds a new configuration field in each pump called filters and its structure is
	// the following:
	// ```{.json}
	// "filters":{
	//   "api_ids":[],
	//   "org_ids":[],
	//   "response_codes":[],
	//   "skip_api_ids":[],
	//   "skip_org_ids":[],
	//   "skip_response_codes":[]
	// }
	// ```
	// The fields api_ids, org_ids and response_codes works as allow list (APIs and orgs where we
	// want to send the analytics records) and the fields skip_api_ids, skip_org_ids and
	// skip_response_codes works as block list.
	//
	// The priority is always block list configurations over allow list.
	//
	// An example of configuration would be:
	// ```{.json}
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
	Filters analytics.AnalyticsFilters `json:"filters"`
	// By default, a pump will wait forever for each write operation to complete; you can configure an optional timeout by setting the configuration option `timeout`.
	// If you have deployed multiple pumps, then you can configure each timeout independently. The timeout is in seconds and defaults to 0.
	//
	// The timeout is configured within the main pump config as shown here; note that this example would configure a 5 second timeout:
	// ```{.json}
	// "pump_name": {
	//   ...
	//   "timeout":5,
	//   "meta": {...}
	// }
	// ```
	//
	// Tyk will inform you if the pump's write operation is taking longer than the purging loop (configured via `purge_delay`) as this will mean that data is purged before being written to the target data sink.
	//
	// If there is no timeout configured and pump's write operation is taking longer than the purging loop, the following warning log will be generated:
	// `Pump {pump_name} is taking more time than the value configured of purge_delay. You should try to set a timeout for this pump.`
	//
	// If there is a timeout configured, but pump's write operation is still taking longer than the purging loop, the following warning log will be generated:
	// `Pump {pump_name} is taking more time than the value configured of purge_delay. You should try lowering the timeout configured for this pump.`.
	Timeout int `json:"timeout"`
	// Setting this to true will avoid writing raw_request and raw_response fields for each request
	// in pumps. Defaults to `false`.
	OmitDetailedRecording bool `json:"omit_detailed_recording"`
	// Defines maximum size (in bytes) for Raw Request and Raw Response logs, this value defaults
	// to 0. If it is not set then tyk-pump will not trim any data and will store the full
	// information. This can also be set at a pump level. For example:
	// ```{.json}
	// "csv": {
	//   "type": "csv",
	//   "max_record_size":1000,
	//   "meta": {
	//     "csv_dir": "./"
	//   }
	// }
	// ```
	MaxRecordSize int `json:"max_record_size"`
	// IgnoreFields defines a list of analytics fields that will be ignored when writing to the pump.
	// This can be used to avoid writing sensitive information to the Database, or data that you don't really need to have.
	// The field names must be the same as the JSON tags of the analytics record fields.
	// For example: `["api_key", "api_version"]`.
	IgnoreFields []string `json:"ignore_fields"`
	// Meta is a map of configuration values that are specific to each pump. For example, the
	// `csv` pump requires a `csv_dir` value to be set, that need to be set in the `meta` map.
	Meta map[string]interface{} `json:"meta"`
	// Setting this to true allows the Raw Request to be decoded from base 64 for all pumps. This is set to false by default.
	DecodeRawRequest bool `json:"raw_request_decoded"`
	// Setting this to true allows the Raw Response to be decoded from base 64 for all pumps. This is set to false by default.
	DecodeRawResponse bool `json:"raw_response_decoded"`
}

type UptimeConf struct {
	// TYKCONFIGHEADERSTART
	// HEADER Mongo Uptime Pump
	// In `uptime_pump_config` you can configure a mongo uptime pump. By default, the uptime pump
	// is going to be `mongo` type, so it's not necessary to specify it here. The minimum required
	// configurations for uptime pumps are:
	//
	// * `collection_name` - That determines the uptime collection name in mongo. By default,
	// `tyk_uptime_analytics`.
	// * `mongo_url` - The uptime pump mongo connection url. It is usually something like
	// "mongodb://username:password@{hostname:port},{hostname:port}/{db_name}".
	// TYKCONFIGHEADEREND
	// TYKCONFIGEXPAND
	pumps.MongoConf
	// TYKCONFIGHEADERSTART
	// HEADER SQL Uptime Pump
	// *Supported in Tyk Pump v1.5.0+*
	//
	// In `uptime_pump_config` you can configure a SQL uptime pump. To do that, you need to add the
	// field `uptime_type` with `sql` value. You can also use different types of SQL Uptime pumps,
	// like `postgres` or `sqlite` using the `type` field.
	//
	// An example of a SQL Postgres uptime pump would be:
	// ```{.json}
	// "uptime_pump_config": {
	//     "uptime_type": "sql",
	//     "type": "postgres",
	//     "connection_string": "host=sql_host port=sql_port user=sql_usr dbname=dbname password=sql_pw",
	//     "table_sharding": false
	// },
	// ```
	//
	// Take into account that you can also set `log_level` field into the `uptime_pump_config` to `debug`,
	// `info` or `warning`. By default, the SQL logger verbosity is `silent`.
	// TYKCONFIGHEADEREND
	// TYKCONFIGEXPAND
	pumps.SQLConf
	// Determines the uptime type. Options are `mongo` and `sql`. Defaults to `mongo`.
	UptimeType string `json:"uptime_type"`
}

type TykPumpConfiguration struct {
	// The number of seconds the Pump waits between checking for analytics data and purge it from
	// Redis.
	PurgeDelay int `json:"purge_delay"`
	// The maximum number of records to pull from Redis at a time. If it's unset or `0`, all the
	// analytics records in Redis are pulled. If it's set, `storage_expiration_time` is used to
	// reset the analytics record TTL.
	PurgeChunk int64 `json:"purge_chunk"`
	// The number of seconds for the analytics records TTL. It only works if `purge_chunk` is
	// enabled. Defaults to `60` seconds.
	StorageExpirationTime int64 `json:"storage_expiration_time"`
	// Setting this to `false` will create a pump that pushes uptime data to Uptime Pump, so the
	// Dashboard can read it. Disable by setting to `true`.
	DontPurgeUptimeData bool       `json:"dont_purge_uptime_data"`
	UptimePumpConfig    UptimeConf `json:"uptime_pump_config"`
	// The default environment variable prefix for each pump follows this format:
	// `TYK_PMP_PUMPS_{PUMP-NAME}_`, for example `TYK_PMP_PUMPS_KAFKA_`.
	//
	// You can also set custom names for each pump specifying the pump type. For example, if you
	// want a Kafka pump which is called `PROD` you need to create `TYK_PMP_PUMPS_PROD_TYPE=kafka`
	// and configure it using the `TYK_PMP_PUMPS_PROD_` prefix.
	Pumps map[string]PumpConfig `json:"pumps"`
	// Sets the analytics storage type. Where the pump will be fetching data from. Currently, only
	// the `redis` option is supported.
	AnalyticsStorageType string `json:"analytics_storage_type"`
	// Example Redis storage configuration:
	// ```{.json}
	//   "analytics_storage_config": {
	//     "type": "redis",
	//     "host": "localhost",
	//     "port": 6379,
	//     "hosts": null,
	//     "username": "",
	//     "password": "",
	//     "database": 0,
	//     "optimisation_max_idle": 100,
	//     "optimisation_max_active": 0,
	//     "enable_cluster": false,
	//     "redis_use_ssl": false,
	//     "redis_ssl_insecure_skip_verify": false
	//   },
	// ```
	AnalyticsStorageConfig storage.RedisStorageConfig `json:"analytics_storage_config"`
	// Connection string for StatsD monitoring for information please see the
	// [Instrumentation docs](https://tyk.io/docs/basic-config-and-security/report-monitor-trigger-events/instrumentation/).
	StatsdConnectionString string `json:"statsd_connection_string"`
	// Custom prefix value. For example separate settings for production and staging.
	StatsdPrefix string `json:"statsd_prefix"`
	// Set the logger details for tyk-pump. The posible values are: `info`,`debug`,`error` and
	// `warn`. By default, the log level is `info`.
	LogLevel string `json:"log_level"`
	// Set the logger format. The possible values are: `text` and `json`. By default, the log
	// format is `text`.
	LogFormat string `json:"log_format"`
	// TYKCONFIGHEADERSTART
	// HEADER Health Check
	// From v2.9.4, we have introduced a `/health` endpoint to confirm the Pump is running. You
	// need to configure the following settings. This returns a HTTP 200 OK response if the Pump is
	// running.
	// TYKCONFIGHEADEREND
	// The default is "hello".
	HealthCheckEndpointName string `json:"health_check_endpoint_name"`
	// The default port is 8083.
	HealthCheckEndpointPort int `json:"health_check_endpoint_port"`
	// Setting this to true will avoid writing raw_request and raw_response fields for each request
	// in pumps. Defaults to false.
	OmitDetailedRecording bool `json:"omit_detailed_recording"`
	// Defines maximum size (in bytes) for Raw Request and Raw Response logs, this value defaults
	// to 0. If it is not set then tyk-pump will not trim any data and will store the full
	// information. This can also be set at a pump level. For example:
	// ```{.json}
	// "csv": {
	//   "type": "csv",
	//   "max_record_size":1000,
	//   "meta": {
	//     "csv_dir": "./"
	//   }
	// }
	// ```
	MaxRecordSize int `json:"max_record_size"`
	// Defines if tyk-pump should ignore all the values in configuration file. Specially useful when setting all configurations in environment variables.
	OmitConfigFile bool `json:"omit_config_file"`
	// Enable debugging of Tyk Pump by exposing profiling information, the same as the gateway https://tyk.io/docs/troubleshooting/tyk-gateway/profiling/
	HTTPProfile bool `json:"enable_http_profiler"`

	// Setting this to true allows the Raw Request to be decoded from base 64
	// for all pumps. This is set to false by default.
	DecodeRawRequest bool `json:"raw_request_decoded"`

	// Setting this to true allows the Raw Response to be decoded from base 64 for all pumps. This is set to false by default.
	DecodeRawResponse bool `json:"raw_response_decoded"`
}

func LoadConfig(filePath *string, configStruct *TykPumpConfiguration) {
	if !configStruct.shouldOmitConfigFile() {
		configuration, err := ioutil.ReadFile(*filePath)
		if err != nil {
			log.Error("Couldn't load configuration file: ", err)
		}

		marshalErr := json.Unmarshal(configuration, &configStruct)
		if marshalErr != nil {
			log.Error("Couldn't unmarshal configuration: ", marshalErr)
		}
	}

	toUpperMap := make(map[string]PumpConfig)
	for pumpName := range configStruct.Pumps {
		upperPumpName := strings.ToUpper(pumpName)
		toUpperMap[upperPumpName] = configStruct.Pumps[pumpName]
	}
	configStruct.Pumps = toUpperMap

	overrideErr := envconfig.Process(ENV_PREVIX, configStruct)
	if overrideErr != nil {
		log.Error("Failed to process environment variables after file load: ", overrideErr)
	}

	errLoadEnvPumps := configStruct.LoadPumpsByEnv()
	if errLoadEnvPumps != nil {
		log.Fatal("error loading pumps env vars:", errLoadEnvPumps)
	}
}

func (cfg *TykPumpConfiguration) shouldOmitConfigFile() bool {
	shouldOmit, omitEnvExist := os.LookupEnv(ENV_PREVIX + "_OMITCONFIGFILE")
	return omitEnvExist && strings.EqualFold(shouldOmit, "true")
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
			pmpName := strings.ToUpper(envSplit[0])

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
