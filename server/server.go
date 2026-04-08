package server

import (
	"fmt"
	"net/http"
	pprof_http "net/http/pprof"

	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var defaultHealthEndpoint = "health"
var defaultHealthPort = 8083
var serverPrefix = "server"
var log = logger.GetLogger()

func ServeHealthCheck(configHealthEndpoint string, configHealthPort int, enableProfiling bool) {
	healthEndpoint := configHealthEndpoint
	if healthEndpoint == "" {
		healthEndpoint = defaultHealthEndpoint
	}
	healthPort := configHealthPort
	if healthPort == 0 {
		healthPort = defaultHealthPort
	}

	r := mux.NewRouter()

	r.HandleFunc("/"+healthEndpoint, Healthcheck).Methods("GET")
	if enableProfiling {
		r.HandleFunc("/debug/pprof/profile", pprof_http.Profile)
		r.HandleFunc("/debug/pprof/{_:.*}", pprof_http.Index)
	}

	log.WithFields(logrus.Fields{
		"prefix": serverPrefix,
	}).Info("Serving health check endpoint at http://localhost:", healthPort, "/", healthEndpoint, " ...")

	if err := http.ListenAndServe(":"+fmt.Sprint(healthPort), r); err != nil {
		log.WithFields(logrus.Fields{
			"prefix": serverPrefix,
		}).Fatal("Error serving health check endpoint", err)
	}
}

func Healthcheck(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(`{"status": "ok"}`))
}
