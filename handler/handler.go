package handler

import (
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/config"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/storage"
)

type PumpHandler struct{
	 SystemConfig *config.TykPumpConfiguration

	 AnalyticsStorage storage.AnalyticsStorage
	 UptimeStorage storage.AnalyticsStorage

	 Pumps []pumps.Pump
	 UptimePump *pumps.MongoPump

	 logger  *logrus.Logger

}

func (handler *PumpHandler) SetLogger(log *logrus.Logger){
	handler.logger = log
}

func (handler *PumpHandler) SetAnalyticsStorage(storage storage.AnalyticsStorage)  {
		handler.AnalyticsStorage = storage
}

func (handler *PumpHandler) SetUptimeStorage(storage storage.AnalyticsStorage)  {
	handler.UptimeStorage = storage
}


func (handler *PumpHandler) setupAnalyticStorage(){
	switch handler.SystemConfig.AnalyticsStorageType {
	case "redis":
		handler.AnalyticsStorage = &storage.RedisClusterStorageManager{}
	default:
		handler.AnalyticsStorage = &storage.RedisClusterStorageManager{}
	}

	handler.AnalyticsStorage.Init(handler.SystemConfig.AnalyticsStorageConfig)
}

func (handler *PumpHandler) setupUptimeStorage(){
	switch handler.SystemConfig.AnalyticsStorageType {
	case "redis":
		handler.UptimeStorage = &storage.RedisClusterStorageManager{}
	default:
		handler.UptimeStorage = &storage.RedisClusterStorageManager{}
	}

	// Copy across the redis configuration
	uptimeConf := handler.SystemConfig.AnalyticsStorageConfig

	// Swap key prefixes for uptime purger
	uptimeConf.RedisKeyPrefix = "host-checker:"
	handler.UptimeStorage.Init(uptimeConf)
}

func (handler *PumpHandler) storeVersion() {
	var versionStore = &storage.RedisClusterStorageManager{}
	versionConf := handler.SystemConfig.AnalyticsStorageConfig
	versionStore.KeyPrefix = "version-check-"
	versionStore.Config = versionConf
	versionStore.Connect()
	versionStore.SetKey("pump", config.VERSION, 0)
}

func (handler *PumpHandler) Init(config *config.TykPumpConfiguration) {
	handler.SystemConfig = config

	if handler.AnalyticsStorage == nil {
		handler.setupAnalyticStorage()
	}

	if handler.UptimeStorage == nil {
		handler.setupUptimeStorage()
	}



}