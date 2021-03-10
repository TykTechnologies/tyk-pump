package main

import (
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics/demo"
	"github.com/TykTechnologies/tyk-pump/config"
	"github.com/TykTechnologies/tyk-pump/handler"
	logger "github.com/TykTechnologies/tyk-pump/logger"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)



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

	//parsing cli vars
	kingpin.Parse()
	//initialise the logger
	log := logger.GetLogger()

	//load the config given the conf file specified in the --conf var
	config := config.LoadConfig(conf)
	if *debugMode {
		config.LogLevel = "debug"
	}

	//create a new handler with the loaded config
	pumpHandler := handler.NewPumpHandler(config)
	//init the handler
	err := pumpHandler.Init()
	if err != nil {
		log.Error("Error initializing Tyk Pump", err)
		return
	}

	//if we specified that we're in demo mode with --demo, let's populate the pump with data
	if *demoMode != "" {
		log.Warning("BUILDING DEMO DATA AND EXITING...")
		log.Warning("Starting from date: ", time.Now().AddDate(0, 0, -30))
		demo.DemoInit(*demoMode, *demoApiMode, *demoApiVersionMode)
		demo.GenerateDemoData(time.Now().AddDate(0, 0, -30), 30, *demoMode, pumpHandler.WriteToPumps)

		return
	}

	// start the handler purge loop
	pumpHandler.PurgeLoop()
}

