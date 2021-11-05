package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/config"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/gocraft/web"
)

var defaultHealthEndpoint = "health"
var defaultHealthPort = 8083
var serverPrefix = "server"
var pumpConf config.TykPumpConfiguration
var log = logger.GetLogger()


func Serve(conf config.TykPumpConfiguration){
	configHealthEndpoint:= conf.HealthCheckEndpointName
	configHealthPort := conf.HealthCheckEndpointPort

	pumpConf = conf

	healthEndpoint := configHealthEndpoint
	if healthEndpoint == "" {
		healthEndpoint = defaultHealthEndpoint
	}
	healthPort := configHealthPort
	if healthPort == 0 {
		healthPort = defaultHealthPort
	}

	router := web.New(Context{}).
		Get("/"+healthEndpoint, (*Context).Healthcheck).
		Get("/config", (*Context).GetConfig)

	log.WithFields(logrus.Fields{
		"prefix": serverPrefix,
	}).Info("Serving health check endpoint at http://localhost:", healthPort, "/", healthEndpoint, " ...")

	if err := http.ListenAndServe(":"+fmt.Sprint(healthPort), router); err != nil {
		log.WithFields(logrus.Fields{
			"prefix": serverPrefix,
		}).Fatal("Error serving health check endpoint", err)
	}
}

type Context struct{}

func (c *Context) Healthcheck(rw web.ResponseWriter, req *web.Request) {
	rw.Header().Set("Content-type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(`{"status": "ok"}`))
}

func (c *Context) GetConfig(rw web.ResponseWriter, req *web.Request){
	rw.Header().Set("Content-type", "application/json")
	rw.WriteHeader(http.StatusOK)
	blurredConf := pumpConf.BlurSensitiveData()
	confBytes,_ := json.Marshal(blurredConf)
	rw.Write(confBytes)
}


