package main

import (
	"flag"
	"time"

	"github.com/TykTechnologies/logrus"
	prefixed "github.com/TykTechnologies/logrus-prefixed-formatter"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/storage"
	logger "github.com/TykTechnologies/tykcommon-logger"
	"gopkg.in/vmihailenco/msgpack.v2"
	"os"
	"github.com/gocraft/health"
)

var SystemConfig TykPumpConfiguration
var AnalyticsStore storage.AnalyticsStorage
var UptimeStorage storage.AnalyticsStorage
var Pumps []pumps.Pump
var UptimePump pumps.MongoPump

var log = logger.GetLogger()

var mainPrefix string = "main"
var buildDemoData string

func init() {
	SystemConfig = TykPumpConfiguration{}
	confFile := flag.String("c", "pump.conf", "Path to the config file")
	demoMode := flag.String("demo", "", "Generate demo data")
	flag.Parse()

	log.Formatter = new(prefixed.TextFormatter)

	buildDemoData = *demoMode
	envDemo := os.Getenv("TYK_PMP_BUILDDEMODATA")
	if envDemo != "" {
		log.Warning("Demo mode active via environemnt variable")
		buildDemoData = envDemo
	}

	log.WithFields(logrus.Fields{
		"prefix": mainPrefix,
	}).Info("## Tyk Analytics Pump, ", VERSION, " ##")

	LoadConfig(confFile, &SystemConfig)
}

func setupAnalyticsStore() {
	switch SystemConfig.AnalyticsStorageType {
	case "redis":
		AnalyticsStore = &storage.RedisClusterStorageManager{}
		UptimeStorage = &storage.RedisClusterStorageManager{}
	default:
		AnalyticsStore = &storage.RedisClusterStorageManager{}
		UptimeStorage = &storage.RedisClusterStorageManager{}
	}

	AnalyticsStore.Init(SystemConfig.AnalyticsStorageConfig)

	// Copy across the redis configuration
	uptimeConf := SystemConfig.AnalyticsStorageConfig

	// Swap key prefixes for uptime purger
	uptimeConf.RedisKeyPrefix = "host-checker:"
	UptimeStorage.Init(uptimeConf)
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

	if !SystemConfig.DontPurgeUptimeData {
		UptimePump = pumps.MongoPump{}
		UptimePump.Init(SystemConfig.UptimePumpConfig)
		log.WithFields(logrus.Fields{
			"prefix": mainPrefix,
		}).Info("Init Uptime Pump: ", UptimePump.GetName())
	}

}

func StartPurgeLoop(nextCount int) {
	job := instrument.NewJob("PumpRecordsPurge")

	time.Sleep(time.Duration(nextCount) * time.Second)

	AnalyticsValues := AnalyticsStore.GetAndDeleteSet(storage.ANALYTICS_KEYNAME)

	if len(AnalyticsValues) > 0 {
		startTime := time.Now()

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
				job.Event("record")
			}
		}

		// Send to pumps
		writeToPumps(keys, job, startTime)

		job.Timing("purge_time_all", time.Since(startTime).Nanoseconds())

	}

	if !SystemConfig.DontPurgeUptimeData {
		UptimeValues := UptimeStorage.GetAndDeleteSet(storage.UptimeAnalytics_KEYNAME)
		UptimePump.WriteUptimeData(UptimeValues)
	}

	StartPurgeLoop(nextCount)
}

func writeToPumps(keys []interface{}, job *health.Job, startTime time.Time) {
	// Send to pumps
	if Pumps != nil {
		for _, pmp := range Pumps {
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Debug("Writing to: ", pmp.GetName())
			pmp.WriteData(keys)
			if job != nil {
				job.Timing("purge_time_"+pmp.GetName(), time.Since(startTime).Nanoseconds())
			}

		}
	} else {
		log.WithFields(logrus.Fields{
			"prefix": mainPrefix,
		}).Warning("No pumps defined!")
	}
}

func main() {
	// Create the store
	setupAnalyticsStore()

	// prime the pumps
	initialisePumps()

	if buildDemoData != "" {
		log.Warning("BUILDING DEMO DATA AND EXITING...")
		log.Warning("Starting from date: ", time.Now().AddDate(0, 0, -30))
		demo.DemoInit(buildDemoData)
		demo.GenerateDemoData(time.Now().AddDate(0, 0, -30), 30, buildDemoData, writeToPumps)

		return
	}

	// start the worker loop
	log.WithFields(logrus.Fields{
		"prefix": mainPrefix,
	}).Info("Starting purge loop @", SystemConfig.PurgeDelay, "(s)")

	SetupInstrumentation()
	StartPurgeLoop(SystemConfig.PurgeDelay)
}
