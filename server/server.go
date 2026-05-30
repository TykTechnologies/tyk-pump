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

// reqproof:implements SW-REQ-032
// resolveHealthCheckParams applies endpoint+port defaults and returns the
// effective values, decoupled from the blocking ListenAndServe call so the
// defaulting decisions can be unit-tested without binding a port.
func resolveHealthCheckParams(configHealthEndpoint string, configHealthPort int) (endpoint string, port int) {
	endpoint = configHealthEndpoint
	if endpoint == "" {
		endpoint = defaultHealthEndpoint
	}
	port = configHealthPort
	if port == 0 {
		port = defaultHealthPort
	}
	return endpoint, port
}

// reqproof:implements SW-REQ-032
// buildHealthCheckRouter constructs the health-check + optional pprof router,
// decoupled from ListenAndServe so route registration can be unit-tested.
func buildHealthCheckRouter(endpoint string, enableProfiling bool) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/"+endpoint, Healthcheck).Methods("GET")
	if enableProfiling {
		r.HandleFunc("/debug/pprof/profile", pprof_http.Profile)
		r.HandleFunc("/debug/pprof/{_:.*}", pprof_http.Index)
	}
	return r
}

// reqproof:implements SW-REQ-032
func ServeHealthCheck(configHealthEndpoint string, configHealthPort int, enableProfiling bool) {
	endpoint, port := resolveHealthCheckParams(configHealthEndpoint, configHealthPort)
	r := buildHealthCheckRouter(endpoint, enableProfiling)

	log.WithFields(logrus.Fields{
		"prefix": serverPrefix,
	}).Info("Serving health check endpoint at http://localhost:", port, "/", endpoint, " ...")

	if err := http.ListenAndServe(":"+fmt.Sprint(port), r); err != nil { //mcdc:ignore http.ListenAndServe is a blocking IO call whose error path cannot be unit-tested without binding a real port
		log.WithFields(logrus.Fields{
			"prefix": serverPrefix,
		}).Fatal("Error serving health check endpoint", err)
	}
}

// reqproof:implements SW-REQ-032
func Healthcheck(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(`{"status": "ok"}`))
}
