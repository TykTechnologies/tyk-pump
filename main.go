package main

import (
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/TykTechnologies/tyk-pump/config"
	"github.com/TykTechnologies/tyk-pump/handler"
	logger "github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/storage"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var SystemConfig config.TykPumpConfiguration
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
	version            = kingpin.Version(config.VERSION)
)


func main() {

	kingpin.Parse()

	config := config.LoadConfig(conf)
	if *debugMode {
		config.LogLevel = "debug"
	}

	pumpHandler := handler.NewPumpHandler(config)
	err := pumpHandler.Init()
	if err != nil {
		log.Error("Error initializing Tyk Pump", err)
		return
	}

	if *demoMode != "" {
		log.Warning("BUILDING DEMO DATA AND EXITING...")
		log.Warning("Starting from date: ", time.Now().AddDate(0, 0, -30))
		demo.DemoInit(*demoMode, *demoApiMode, *demoApiVersionMode)
		demo.GenerateDemoData(time.Now().AddDate(0, 0, -30), 30, *demoMode, pumpHandler.WriteToPumps)

		return
	}



	// start the worker loop
	pumpHandler.PurgeLoop()
}

