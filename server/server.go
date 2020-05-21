package server

import (
	"fmt"
	"net/http"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/gocraft/web"
)

var defaultHealthEndpoint = "health"
var defaultHealthPort = 8080
var serverPrefix = "server"
var log = logger.GetLogger()

func ServeHealthCheck(configHealthEndpoint string, configHealthPort int) {
	healthEndpoint := configHealthEndpoint
	if healthEndpoint == "" {
		healthEndpoint = defaultHealthEndpoint
	}
	healthPort := configHealthPort
	if healthPort == 0 {
		healthPort = defaultHealthPort
	}

	router := web.New(Context{}).Get("/"+healthEndpoint, (*Context).Healthcheck)

	log.WithFields(logrus.Fields{
		"prefix": serverPrefix,
	}).Info("serving health check endpoint at http://localhost:", healthPort, "/", healthEndpoint)

	log.Fatal(http.ListenAndServe("localhost:"+fmt.Sprint(healthPort), router))
}

type Context struct{}

func (c *Context) Healthcheck(rw web.ResponseWriter, req *web.Request) {
	rw.Header().Set("Content-type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(`{"status": "ok"}`))
}
