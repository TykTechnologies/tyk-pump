package handler

import (
	"errors"
	"os"
	"strings"


	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/config"
	"github.com/TykTechnologies/tyk-pump/instrumentation"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/server"
	"github.com/TykTechnologies/tyk-pump/storage"
)

//PumpHandler is the main struct to handle all the tyk-pump process.
type PumpHandler struct{
	 SystemConfig *config.TykPumpConfiguration

	 AnalyticsStorage storage.AnalyticsStorage
	 externalAnalyticStorage bool

	 UptimeStorage storage.AnalyticsStorage
	 externalUptimeStorage bool

	 Pumps []pumps.Pump
	 UptimePump *pumps.MongoPump

	 logger  *logrus.Logger
	 log  *logrus.Entry
}

//SetLogger set he PumpHandler logger to a given *logrus.Logger.
func (handler *PumpHandler) SetLogger(log *logrus.Logger){
	handler.logger = log
}

//initLogger initialise the pump main logger. It checks in TYK_LOGLEVEL and in the config log_level field to set the log level. ENV var > config specification
func (handler *PumpHandler) initLogger() error{
	handler.logger =  logger.GetLogger()
	handler.log = handler.logger.WithField("prefix","main")


	//If TYK_LOGLEVEL != "", the log level is set in the logger.GetLogger() method
	if os.Getenv("TYK_LOGLEVEL") == "" {
		level := strings.ToLower(handler.SystemConfig.LogLevel)
		switch level {
		case "", "info":
			// default, do nothing
		case "error":
			handler.logger.Level = logrus.ErrorLevel
		case "warn":
			handler.logger.Level = logrus.WarnLevel
		case "debug":
			handler.logger.Level = logrus.DebugLevel
		default:
			handler.log.Error("Invalid log level %q specified in config, must be error, warn, debug or info. ", level)
			return errors.New("invalid log level %q specified in config. It must be error, warn, debug or info")
		}
	}
	return nil
}

//SetAnalyticsStorage set an external ANALYTICS storage. It's specially useful when we want to use tyk-pump as an external package.
func (handler *PumpHandler) SetAnalyticsStorage(storage storage.AnalyticsStorage)  {
	handler.AnalyticsStorage = storage
	handler.externalAnalyticStorage = true
}

//SetUptimeStorage set an external UPTIME storage. It's specially useful when we want to use tyk-pump as an external package.
func (handler *PumpHandler) SetUptimeStorage(storage storage.AnalyticsStorage)  {
	handler.UptimeStorage = storage
	handler.externalUptimeStorage = true
}

//setupAnalyticStorage setup and init the tyk-pump ANALYTICS storage. If it's already set it means we're using pump as an external package.
func (handler *PumpHandler) setupAnalyticStorage() error{
	//if the analytic storage is set externally, don't setup it again
	if !handler.externalAnalyticStorage {
		switch handler.SystemConfig.AnalyticsStorageType {
		case "redis":
			handler.AnalyticsStorage = &storage.RedisClusterStorageManager{}
		case "queue":
			handler.AnalyticsStorage = &storage.Queue{}
		default:
			handler.AnalyticsStorage = &storage.RedisClusterStorageManager{}
		}
	}
	err := handler.AnalyticsStorage.Init(handler.SystemConfig.AnalyticsStorageConfig)
	if err != nil {
		return err
	}
	return nil
}

//setupUptimeStorage setup and init the tyk-pump UPTIME storage. If it's already set it means we're using pump as an external package.
func (handler *PumpHandler) setupUptimeStorage() error{
	//if the uptime storage is set externally, don't setup it again
	if !handler.externalUptimeStorage {
		switch handler.SystemConfig.AnalyticsStorageType {
		case "redis":
			handler.UptimeStorage = &storage.RedisClusterStorageManager{}
		case "queue":
			handler.UptimeStorage = &storage.Queue{}
		default:
			handler.UptimeStorage = &storage.RedisClusterStorageManager{}
		}
	}
	//if we don't want to purge uptime data, just ignore the initialization
	if !handler.SystemConfig.DontPurgeUptimeData {
		// Copy across the redis configuration
		uptimeConf := handler.SystemConfig.AnalyticsStorageConfig

		// Swap key prefixes for uptime purger
		uptimeConf.RedisKeyPrefix = "host-checker:"
		err := handler.UptimeStorage.Init(uptimeConf)
		if err !=nil {
			return err
		}
	}

	return nil
}

//storeVersion set the version of tyk-pump in analytics storage. It only works when the analytics is REDIS.
func (handler *PumpHandler) storeVersion() error{
	if handler.SystemConfig.AnalyticsStorageType == "redis"{
		var versionStore = &storage.RedisClusterStorageManager{}
		versionConf := handler.SystemConfig.AnalyticsStorageConfig
		versionStore.KeyPrefix = "version-check-"
		versionStore.Config = versionConf
		connected := versionStore.Connect()
		if !connected {
			return errors.New("error connecting to version storage")
		}
		err :=versionStore.SetKey("pump", config.VERSION, 0)
		if err != nil {
			return errors.New("error setting version key in storage:"+err.Error())
		}
	}
	return nil
}

// initPumps initialise each configured pump and store it in handler.Pumps variable. When initializing, it set the filters, timeout and detailed recording of each pump.
// It can fail when the pump name is invalid or when in an error initialise the pump. For this last error, please check Init func of each pump.
func (handler *PumpHandler) initPumps() error{
	handler.Pumps = make([]pumps.Pump, len(handler.SystemConfig.Pumps))
	i := 0
	for key, pmp := range handler.SystemConfig.Pumps {
		pumpTypeName := pmp.Type
		if pumpTypeName == "" {
			pumpTypeName = key
		}

		pmpType, err := pumps.GetPumpByName(pumpTypeName)
		if err != nil {
			handler.log.Error("Pump load error (skipping): ", err)
			return err
		} else {
			thisPmp := pmpType.New()
			thisPmp.SetFilters(pmp.Filters)
			thisPmp.SetTimeout(pmp.Timeout)
			thisPmp.SetOmitDetailedRecording(pmp.OmitDetailedRecording)
			initErr := thisPmp.Init(pmp.Meta)
			if initErr != nil {
				handler.logger.Error("Pump init error (skipping): ", initErr)
				return initErr
			} else {
				handler.log.Info("Init Pump: ", thisPmp.GetName())
				handler.Pumps[i] = thisPmp
			}
		}
		i++
	}
	return nil
}
func (handler *PumpHandler) initUptimeData() error{
	if !handler.SystemConfig.DontPurgeUptimeData {
		handler.log.Info("'dont_purge_uptime_data' set to false, attempting to start Uptime pump! ", handler.UptimePump.GetName())
		handler.UptimePump = &pumps.MongoPump{}
		err := handler.UptimePump.Init(handler.SystemConfig.UptimePumpConfig)
		if err != nil {
			return err
		}
		handler.log.Info("Init Uptime Pump: ", handler.UptimePump.GetName())
	}
	return nil
}


//Init initializes each component needed to be ready to stat sending the data. Logger, Instrumentation, Analytics and Uptime Storages, each pump and health check endpoint.
func (handler *PumpHandler) Init() error{
	//Setting up the main logger
	if err := handler.initLogger(); err != nil {
		return err
	}

	//Setting up the Tyk Instrumentation
	if os.Getenv("TYK_INSTRUMENTATION") == "1" {
		instrumentation.SetupInstrumentation(handler.SystemConfig)
	}

	//Setting up the analytic storage.
	if err := handler.setupAnalyticStorage(); err != nil {
		return errors.New("error setting up analytic storage:"+err.Error())
	}

	//Setting up the version in the analytic storage. It only works when the storage type is redis.
	if err:= handler.storeVersion(); err != nil {
		return err
	}


	//Setting up the uptime storage.
	if err := handler.setupUptimeStorage();err != nil{
		return errors.New("error setting up uptime storage:"+err.Error())
	}


	//Initializing each pump. It can fail when we Init each pump or if we create an non-existing pump.
	if err:= handler.initPumps(); err != nil {
		return err
	}

	//Enabling health check endpoint
	if handler.SystemConfig.EnableHealthCheck{
		go server.ServeHealthCheck(handler.SystemConfig.HealthCheckEndpointName, handler.SystemConfig.HealthCheckEndpointPort)
	}


	//Setting defaults for storage_expiration_time when purge_chunk is active
	if handler.SystemConfig.PurgeChunk > 0 {
		handler.logger.WithField("PurgeChunk", handler.SystemConfig.PurgeChunk).Info("PurgeChunk enabled")
		if handler.SystemConfig.StorageExpirationTime == 0 {
			handler.SystemConfig.StorageExpirationTime = 60
			handler.logger.WithField("StorageExpirationTime", 60).Warn("StorageExpirationTime not set, but PurgeChunk enabled, overriding to 60s")
		}
	}


	return nil
}

//NewPumpHandler creates a new pump handler given a TykPumpConfiguration.
//Initializing the handler is required after this.
func NewPumpHandler(config *config.TykPumpConfiguration) *PumpHandler{
	return &PumpHandler{
		SystemConfig: config,
	}
}