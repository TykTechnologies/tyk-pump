package main

import (
	"os"
	"runtime/debug"
	"time"

	"github.com/gocraft/health"
)

var applicationGCStats = debug.GCStats{}
var instrument = health.NewStream()

// SetupInstrumentation handles all the intialisation of the instrumentation handler
// reqproof:implements SW-REQ-005
func SetupInstrumentation() {
	var enabled bool
	//instrument.AddSink(&health.WriterSink{os.Stdout})
	thisInstr := os.Getenv("TYK_INSTRUMENTATION")

	if thisInstr == "1" {
		enabled = true
	}

	if !enabled {
		return
	}

	if SystemConfig.StatsdConnectionString == "" {
		log.Error("Instrumentation is enabled, but no connectionstring set for statsd")
		return
	}

	log.Info("Sending stats to: ", SystemConfig.StatsdConnectionString, " with prefix: ", SystemConfig.StatsdPrefix)
	statsdSink, err := NewStatsDSink(SystemConfig.StatsdConnectionString,
		&StatsDSinkOptions{Prefix: SystemConfig.StatsdPrefix})

	if err != nil { //mcdc:ignore:capability-gap NewStatsDSink failure calls log.Fatal and exits the process; covered as KI logfatal-on-statsd-setup rather than in-process unit evidence [ki: logfatal-on-statsd-setup]
		log.Fatal("Failed to start StatsD check: ", err)
		return
	}

	log.Info("StatsD instrumentation sink started")
	instrument.AddSink(statsdSink)

	MonitorApplicationInstrumentation()
}

// reqproof:implements SW-REQ-005
func MonitorApplicationInstrumentation() {
	log.Info("Starting application monitoring...")
	go func() {
		job := instrument.NewJob("GCActivity")
		metadata := health.Kvs{"host": "pump"}
		applicationGCStats.PauseQuantiles = make([]time.Duration, 5)

		for {
			debug.ReadGCStats(&applicationGCStats)
			job.GaugeKv("pauses_quantile_min", float64(applicationGCStats.PauseQuantiles[0].Nanoseconds()), metadata)
			job.GaugeKv("pauses_quantile_25", float64(applicationGCStats.PauseQuantiles[1].Nanoseconds()), metadata)
			job.GaugeKv("pauses_quantile_50", float64(applicationGCStats.PauseQuantiles[2].Nanoseconds()), metadata)
			job.GaugeKv("pauses_quantile_75", float64(applicationGCStats.PauseQuantiles[3].Nanoseconds()), metadata)
			job.GaugeKv("pauses_quantile_max", float64(applicationGCStats.PauseQuantiles[4].Nanoseconds()), metadata)

			time.Sleep(5 * time.Second)
		}
	}()
}
