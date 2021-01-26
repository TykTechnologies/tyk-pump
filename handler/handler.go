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

type PumpHandler struct{
	 SystemConfig *config.TykPumpConfiguration

	 AnalyticsStorage storage.AnalyticsStorage
	 externalAnalyticStorage bool

	 UptimeStorage storage.AnalyticsStorage
	 externalUptimeStorage bool

	 Pumps []pumps.Pump
	 UptimePump *pumps.MongoPump

	 logger  *logrus.Logger
	 loggerPrefix string
}

func (handler *PumpHandler) SetLogger(log *logrus.Logger){
	handler.logger = log
}
func (handler *PumpHandler) initLogger() error{
	handler.logger = logger.GetLogger()
	handler.loggerPrefix = "main"

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
			handler.logger.WithFields(logrus.Fields{
				"prefix": "main",
			}).Error("Invalid log level %q specified in config, must be error, warn, debug or info. ", level)
			return errors.New("invalid log level %q specified in config. It must be error, warn, debug or info")
		}
	}
	return nil
}

func (handler *PumpHandler) SetAnalyticsStorage(storage storage.AnalyticsStorage)  {
	handler.AnalyticsStorage = storage
	handler.externalAnalyticStorage = true
}
func (handler *PumpHandler) SetUptimeStorage(storage storage.AnalyticsStorage)  {
	handler.UptimeStorage = storage
	handler.externalUptimeStorage = true
}

func (handler *PumpHandler) setupAnalyticStorage() error{
	switch handler.SystemConfig.AnalyticsStorageType {
	case "redis":
		handler.AnalyticsStorage = &storage.RedisClusterStorageManager{}
	case "queue":
		handler.AnalyticsStorage = &storage.Queue{}
	default:
		handler.AnalyticsStorage = &storage.RedisClusterStorageManager{}
	}

	return handler.AnalyticsStorage.Init(handler.SystemConfig.AnalyticsStorageConfig)
}
func (handler *PumpHandler) setupUptimeStorage() error{
	switch handler.SystemConfig.AnalyticsStorageType {
	case "redis":
		handler.UptimeStorage = &storage.RedisClusterStorageManager{}
	case "queue":
		handler.UptimeStorage = &storage.Queue{}
	default:
		handler.UptimeStorage = &storage.RedisClusterStorageManager{}
	}

	// Copy across the redis configuration
	uptimeConf := handler.SystemConfig.AnalyticsStorageConfig

	// Swap key prefixes for uptime purger
	uptimeConf.RedisKeyPrefix = "host-checker:"
	return handler.UptimeStorage.Init(uptimeConf)
}
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
			handler.logger.WithFields(logrus.Fields{
				"prefix": handler.loggerPrefix,
			}).Error("Pump load error (skipping): ", err)
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
				handler.logger.WithFields(logrus.Fields{
					"prefix": handler.loggerPrefix,
				}).Info("Init Pump: ", thisPmp.GetName())
				handler.Pumps[i] = thisPmp
			}
		}
		i++
	}
	return nil
}
func (handler *PumpHandler) initUptimeData() error{
	if !handler.SystemConfig.DontPurgeUptimeData {
		handler.logger.WithFields(logrus.Fields{
			"prefix": handler.loggerPrefix,
		}).Info("'dont_purge_uptime_data' set to false, attempting to start Uptime pump! ", handler.UptimePump.GetName())
		handler.UptimePump = &pumps.MongoPump{}
		err := handler.UptimePump.Init(handler.SystemConfig.UptimePumpConfig)
		if err != nil {
			return err
		}
		handler.logger.WithFields(logrus.Fields{
			"prefix": handler.loggerPrefix,
		}).Info("Init Uptime Pump: ", handler.UptimePump.GetName())
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

	//Setting up the analytic storage. If it's already setup, it means that we're using Tyk Pump as a package
	if handler.AnalyticsStorage == nil {
		err := handler.setupAnalyticStorage()
		if err != nil {
			return errors.New("error setting up analytic storage:"+err.Error())
		}

		err =handler.storeVersion()
		if err != nil {
			return err
		}

	}
	//Setting up the uptime storage. If it's already setup, it means that we're using Tyk Pump as a package
	if handler.UptimeStorage == nil {
		err := handler.setupUptimeStorage()
		if err != nil {
			return errors.New("error setting up uptime storage:"+err.Error())
		}
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

func NewPumpHandler(config *config.TykPumpConfiguration) *PumpHandler{
	return &PumpHandler{
		SystemConfig: config,
	}
}