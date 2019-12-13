package main

import (
	"time"

	"os"

	"github.com/gocraft/health"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"

	"github.com/TykTechnologies/logrus"
	prefixed "github.com/TykTechnologies/logrus-prefixed-formatter"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/storage"
	logger "github.com/TykTechnologies/tykcommon-logger"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var SystemConfig TykPumpConfiguration
var AnalyticsStore storage.AnalyticsStorage
var UptimeStorage storage.AnalyticsStorage
var Pumps []pumps.Pump
var UptimePump pumps.MongoPump

var log = logger.GetLogger()

var mainPrefix = "main"
var buildDemoData string

var (
	help     = kingpin.CommandLine.HelpFlag.Short('h')
	conf     = kingpin.Flag("conf", "path to the config file").Short('c').Default("pump.conf").String()
	demoMode = kingpin.Flag("demo", "pass orgID string to generate demo data").Default("").String()
	version  = kingpin.Version(VERSION)
)

func init() {
	SystemConfig = TykPumpConfiguration{}

	kingpin.Parse()

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

	LoadConfig(conf, &SystemConfig)
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

func storeVersion() {
	var versionStore = &storage.RedisClusterStorageManager{}
	versionConf := SystemConfig.AnalyticsStorageConfig
	versionStore.KeyPrefix = "version-check-"
	versionStore.Config = versionConf
	versionStore.Connect()
	versionStore.SetKey("pump", VERSION, 0)
}

func initialisePumps() {
	Pumps = make([]pumps.Pump, len(SystemConfig.Pumps))
	i := 0
	for key, pmp := range SystemConfig.Pumps {
		pumpTypeName := pmp.Type
		if pumpTypeName == "" {
			pumpTypeName = key
		}

		pmpType, err := pumps.GetPumpByName(pumpTypeName)
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
		log.WithFields(logrus.Fields{
			"prefix": mainPrefix,
		}).Info("'dont_purge_uptime_data' set to false, attempting to start Uptime pump! ", UptimePump.GetName())
		UptimePump = pumps.MongoPump{}
		UptimePump.Init(SystemConfig.UptimePumpConfig)
		log.WithFields(logrus.Fields{
			"prefix": mainPrefix,
		}).Info("Init Uptime Pump: ", UptimePump.GetName())
	}	

}

func StartPurgeLoop(secInterval int) {
	for range time.Tick(time.Duration(secInterval) * time.Second) {
		job := instrument.NewJob("PumpRecordsPurge")

		AnalyticsValues := AnalyticsStore.GetAndDeleteSet(storage.ANALYTICS_KEYNAME)
		if len(AnalyticsValues) > 0 {
			startTime := time.Now()

			// Convert to something clean
			keys := make([]interface{}, len(AnalyticsValues))

			for i, v := range AnalyticsValues {
				decoded := analytics.AnalyticsRecord{}
				err := msgpack.Unmarshal([]byte(v.(string)), &decoded)
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
	}
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
	SetupInstrumentation()

	// Store version which will be read by dashboard and sent to
	// vclu(version check and licecnse utilisation) service
	storeVersion()

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

	StartPurgeLoop(SystemConfig.PurgeDelay)
}
