package main

import (
	"context"
	"strings"
	"sync"
	"time"

	"os"

	"github.com/TykTechnologies/logrus"
	prefixed "github.com/TykTechnologies/logrus-prefixed-formatter"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	logger "github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/server"
	"github.com/TykTechnologies/tyk-pump/storage"
	"github.com/gocraft/health"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

var SystemConfig TykPumpConfiguration
var AnalyticsStore storage.AnalyticsStorage
var UptimeStorage storage.AnalyticsStorage
var Pumps []pumps.Pump
var UptimePump pumps.MongoPump

var log = logger.GetLogger()

var mainPrefix = "main"

var (
	help               = kingpin.CommandLine.HelpFlag.Short('h')
	conf               = kingpin.Flag("conf", "path to the config file").Short('c').Default("pump.conf").String()
	demoMode           = kingpin.Flag("demo", "pass orgID string to generate demo data").Default("").String()
	demoApiMode        = kingpin.Flag("demo-api", "pass apiID string to generate demo data").Default("").String()
	demoApiVersionMode = kingpin.Flag("demo-api-version", "pass apiID string to generate demo data").Default("").String()
	debugMode          = kingpin.Flag("debug", "enable debug mode").Bool()
	version            = kingpin.Version(VERSION)
)

func Init() {
	SystemConfig = TykPumpConfiguration{}

	kingpin.Parse()
	log.Formatter = new(prefixed.TextFormatter)

	envDemo := os.Getenv("TYK_PMP_BUILDDEMODATA")
	if envDemo != "" {
		log.Warning("Demo mode active via environemnt variable")
		demoMode = &envDemo
	}

	log.WithFields(logrus.Fields{
		"prefix": mainPrefix,
	}).Info("## Tyk Analytics Pump, ", VERSION, " ##")

	LoadConfig(conf, &SystemConfig)

	// If no environment variable is set, check the configuration file:
	if os.Getenv("TYK_LOGLEVEL") == "" {
		level := strings.ToLower(SystemConfig.LogLevel)
		switch level {
		case "", "info":
			// default, do nothing
		case "error":
			log.Level = logrus.ErrorLevel
		case "warn":
			log.Level = logrus.WarnLevel
		case "debug":
			log.Level = logrus.DebugLevel
		default:
			log.WithFields(logrus.Fields{
				"prefix": "main",
			}).Fatalf("Invalid log level %q specified in config, must be error, warn, debug or info. ", level)
		}
	}

	// If debug mode flag is set, override previous log level parameter:
	if *debugMode {
		log.Level = logrus.DebugLevel
	}

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
				thisPmp.SetFilters(pmp.Filters)
				thisPmp.SetTimeout(pmp.Timeout)
				thisPmp.SetOmitDetailedRecording(pmp.OmitDetailedRecording)
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

func StartPurgeLoop(secInterval int, chunkSize int64, expire time.Duration,omitDetails bool) {
	for range time.Tick(time.Duration(secInterval) * time.Second) {
		job := instrument.NewJob("PumpRecordsPurge")

		AnalyticsValues := AnalyticsStore.GetAndDeleteSet(storage.ANALYTICS_KEYNAME, chunkSize, expire)
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
					if omitDetails {
						decoded.RawRequest = ""
						decoded.RawResponse = ""
					}
					keys[i] = interface{}(decoded)
					job.Event("record")
				}
			}

			// Send to pumps
			writeToPumps(keys, job, startTime, int(secInterval))

			job.Timing("purge_time_all", time.Since(startTime).Nanoseconds())
		}

		if !SystemConfig.DontPurgeUptimeData {
			UptimeValues := UptimeStorage.GetAndDeleteSet(storage.UptimeAnalytics_KEYNAME, chunkSize, expire)
			UptimePump.WriteUptimeData(UptimeValues)
		}
	}
}

func writeToPumps(keys []interface{}, job *health.Job, startTime time.Time, purgeDelay int) {
	// Send to pumps
	if Pumps != nil {
		var wg sync.WaitGroup
		wg.Add(len(Pumps))
		for _, pmp := range Pumps {
			go execPumpWriting(&wg, pmp, &keys, purgeDelay, startTime, job)
		}
		wg.Wait()
	} else {
		log.WithFields(logrus.Fields{
			"prefix": mainPrefix,
		}).Warning("No pumps defined!")
	}
}

func filterData(pump pumps.Pump, keys []interface{}) []interface{} {
	filters := pump.GetFilters()
	if !filters.HasFilter() && !pump.GetOmitDetailedRecording() {
		return keys
	}
	filteredKeys := keys[:]
	newLenght := 0

	for _, key := range filteredKeys {
		decoded := key.(analytics.AnalyticsRecord)
		if pump.GetOmitDetailedRecording() {
			decoded.RawRequest = ""
			decoded.RawResponse = ""
		}
		if filters.ShouldFilter(decoded) {
			continue
		}
		filteredKeys[newLenght] = decoded
		newLenght++
	}
	filteredKeys = filteredKeys[:newLenght]
	return filteredKeys
}

func execPumpWriting(wg *sync.WaitGroup, pmp pumps.Pump, keys *[]interface{}, purgeDelay int, startTime time.Time, job *health.Job) {
	timer := time.AfterFunc(time.Duration(purgeDelay)*time.Second, func() {
		if pmp.GetTimeout() == 0 {
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Warning("Pump  ", pmp.GetName(), " is taking more time than the value configured of purge_delay. You should try to set a timeout for this pump.")
		} else if pmp.GetTimeout() > purgeDelay {
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Warning("Pump  ", pmp.GetName(), " is taking more time than the value configured of purge_delay. You should try lowering the timeout configured for this pump.")
		}
	})
	defer timer.Stop()
	defer wg.Done()

	log.WithFields(logrus.Fields{
		"prefix": mainPrefix,
	}).Debug("Writing to: ", pmp.GetName())

	ch := make(chan error, 1)
	//Load pump timeout
	timeout := pmp.GetTimeout()
	var ctx context.Context
	var cancel context.CancelFunc
	//Initialize context depending if the pump has a configured timeout
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	defer cancel()

	go func(ch chan error, ctx context.Context, pmp pumps.Pump, keys *[]interface{}) {
		filteredKeys := filterData(pmp, *keys)

		ch <- pmp.WriteData(ctx, filteredKeys)
	}(ch, ctx, pmp, keys)

	select {
	case err := <-ch:
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Warning("Error Writing to: ", pmp.GetName(), " - Error:", err)
		}
	case <-ctx.Done():
		switch ctx.Err() {
		case context.Canceled:
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Warning("The writing to ", pmp.GetName(), " have got canceled.")
		case context.DeadlineExceeded:
			log.WithFields(logrus.Fields{
				"prefix": mainPrefix,
			}).Warning("Timeout Writing to: ", pmp.GetName())
		}
	}
	if job != nil {
		job.Timing("purge_time_"+pmp.GetName(), time.Since(startTime).Nanoseconds())
	}
}

func main() {
	Init()
	SetupInstrumentation()
	go server.ServeHealthCheck(SystemConfig.HealthCheckEndpointName, SystemConfig.HealthCheckEndpointPort)

	// Store version which will be read by dashboard and sent to
	// vclu(version check and licecnse utilisation) service
	storeVersion()

	// Create the store
	setupAnalyticsStore()

	// prime the pumps
	initialisePumps()

	if *demoMode != "" {
		log.Warning("BUILDING DEMO DATA AND EXITING...")
		log.Warning("Starting from date: ", time.Now().AddDate(0, 0, -30))
		demo.DemoInit(*demoMode, *demoApiMode, *demoApiVersionMode)
		demo.GenerateDemoData(time.Now().AddDate(0, 0, -30), 30, *demoMode, writeToPumps)

		return
	}

	// Don't enable chunking if zero value
	if SystemConfig.PurgeChunk == 0 {
		SystemConfig.PurgeChunk = -1
	}

	if SystemConfig.PurgeChunk > 0 {
		log.WithField("PurgeChunk", SystemConfig.PurgeChunk).Info("PurgeChunk enabled")
		if SystemConfig.StorageExpirationTime == 0 {
			SystemConfig.StorageExpirationTime = 60
			log.WithField("StorageExpirationTime", 60).Warn("StorageExpirationTime not set, but PurgeChunk enabled, overriding to 60s")
		}
	}

	// start the worker loop
	log.WithFields(logrus.Fields{
		"prefix": mainPrefix,
	}).Infof("Starting purge loop @%d, chunk size %d", SystemConfig.PurgeDelay, SystemConfig.PurgeChunk)


	StartPurgeLoop(SystemConfig.PurgeDelay,SystemConfig.PurgeChunk, time.Duration(SystemConfig.StorageExpirationTime)*time.Second, SystemConfig.OmitDetailedRecording)
}
