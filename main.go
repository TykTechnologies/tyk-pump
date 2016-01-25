package main

import (
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/lonelycode/tyk-pump/analytics"
	"github.com/lonelycode/tyk-pump/logger"
	"github.com/lonelycode/tyk-pump/pumps"
	"github.com/lonelycode/tyk-pump/storage"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"gopkg.in/vmihailenco/msgpack.v2"
	"time"
)

var SystemConfig TykPumpConfiguration
var AnalyticsStore storage.AnalyticsStorage
var Pumps []pumps.Pump

var log = logger.GetLogger()

var mainPrefix string = "main"

func init() {
	SystemConfig = TykPumpConfiguration{}
	confFile := flag.String("c", "pump.conf", "Path to the config file")
	flag.Parse()

	log.Formatter = new(prefixed.TextFormatter)

	log.WithFields(logrus.Fields{
		"prefix": mainPrefix,
	}).Info("## Tyk Analytics Pump, ", version, " ##")

	LoadConfig(confFile, &SystemConfig)
}

func setupAnalyticsStore() {
	switch SystemConfig.AnalyticsStorageType {
	case "redis":
		AnalyticsStore = &storage.RedisClusterStorageManager{}
	default:
		AnalyticsStore = &storage.RedisClusterStorageManager{}
	}

	AnalyticsStore.Init(SystemConfig.AnalyticsStorageConfig)
}

func initialisePumps() {
	Pumps = make([]pumps.Pump, len(SystemConfig.Pumps))
	i := 0
	for name, pmp := range SystemConfig.Pumps {
		pmpType, err := pumps.GetPumpByName(name)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Error("Pump load error (skipping): ", err)
		} else {
			thisPmp := pmpType.New()
			initErr := thisPmp.Init(pmp.Meta)
			if initErr != nil {
				log.Error("Pump init error (skipping): ", initErr)
			} else {
				log.WithFields(logrus.Fields{
					"prefix": mainPrefix,
				}).Info("Init Pump: ", thisPmp.GetName())
				Pumps[i] = thisPmp
			}
		}
		i++
	}
}

func StartPurgeLoop(nextCount int) {

	time.Sleep(time.Duration(nextCount) * time.Second)

	AnalyticsValues := AnalyticsStore.GetAndDeleteSet(storage.ANALYTICS_KEYNAME)
	if len(AnalyticsValues) > 0 {
		// Convert to something clean
		keys := make([]interface{}, len(AnalyticsValues), len(AnalyticsValues))

		for i, v := range AnalyticsValues {
			decoded := analytics.AnalyticsRecord{}
			err := msgpack.Unmarshal(v.([]byte), &decoded)
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Debug("Decoded Record: ", decoded)
			if err != nil {
				log.WithFields(logrus.Fields{
					"prefix": mainPrefix,
				}).Error("Couldn't unmarshal analytics data:", err)
			} else {
				keys[i] = interface{}(decoded)
			}
		}

		// Send to pumps
		if Pumps != nil {
			for _, pmp := range Pumps {
				log.WithFields(logrus.Fields{
					"prefix": mainPrefix,
				}).Debug("Writing to: ", pmp.GetName())
				pmp.WriteData(keys)
			}
		} else {
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Warning("No pumps defined!")
		}

	}

	StartPurgeLoop(nextCount)
}

func main() {
	// Create the store
	setupAnalyticsStore()

	// prime the pumps
	initialisePumps()

	// start the worker loop
	log.WithFields(logrus.Fields{
		"prefix": mainPrefix,
	}).Info("Starting purge loop @", SystemConfig.PurgeDelay, "(s)")
	StartPurgeLoop(SystemConfig.PurgeDelay)
}
