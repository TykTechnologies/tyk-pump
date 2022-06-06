package pumps

import (
	"context"
	"errors"
	"fmt"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"net/http"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusPump struct {
	conf *PrometheusConf
	// Per service
	TotalStatusMetrics  *prometheus.CounterVec
	PathStatusMetrics   *prometheus.CounterVec
	KeyStatusMetrics    *prometheus.CounterVec
	OauthStatusMetrics  *prometheus.CounterVec
	TotalLatencyMetrics *prometheus.HistogramVec

	CommonPumpConfig
}

// @PumpConf Prometheus
type PrometheusConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The full URL to your Prometheus instance, {HOST}:{PORT}. For example `localhost:9090`.
	Addr string `json:"listen_address" mapstructure:"listen_address"`
	// The path to the Prometheus collection. For example `/metrics`.
	Path string `json:"path" mapstructure:"path"`
}

var prometheusPrefix = "prometheus-pump"
var prometheusDefaultENV = PUMPS_ENV_PREFIX + "_PROMETHEUS"

var buckets = []float64{1, 2, 5, 7, 10, 15, 20, 25, 30, 40, 50, 60, 70, 80, 90, 100, 200, 300, 400, 500, 1000, 2000, 5000, 10000, 30000, 60000}

func (p *PrometheusPump) New() Pump {
	newPump := PrometheusPump{}
	newPump.TotalStatusMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tyk_http_status",
			Help: "HTTP status codes per API",
		},
		[]string{"code", "api"},
	)
	newPump.PathStatusMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tyk_http_status_per_path",
			Help: "HTTP status codes per API path and method",
		},
		[]string{"code", "api", "path", "method"},
	)
	newPump.KeyStatusMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tyk_http_status_per_key",
			Help: "HTTP status codes per API key",
		},
		[]string{"code", "key"},
	)
	newPump.OauthStatusMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tyk_http_status_per_oauth_client",
			Help: "HTTP status codes per oAuth client id",
		},
		[]string{"code", "client_id"},
	)
	newPump.TotalLatencyMetrics = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tyk_latency",
			Help:    "Latency added by Tyk, Total Latency, and upstream latency per API",
			Buckets: buckets,
		},
		[]string{"type", "api"},
	)

	prometheus.MustRegister(newPump.TotalStatusMetrics)
	prometheus.MustRegister(newPump.PathStatusMetrics)
	prometheus.MustRegister(newPump.KeyStatusMetrics)
	prometheus.MustRegister(newPump.OauthStatusMetrics)
	prometheus.MustRegister(newPump.TotalLatencyMetrics)
	return &newPump
}

func (p *PrometheusPump) GetName() string {
	return "Prometheus Pump"
}

func (p *PrometheusPump) GetEnvPrefix() string {
	return p.conf.EnvPrefix
}

func (p *PrometheusPump) Init(conf interface{}) error {
	p.conf = &PrometheusConf{}
	p.log = log.WithField("prefix", prometheusPrefix)

	err := mapstructure.Decode(conf, &p.conf)
	if err != nil {
		p.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(p, p.log, p.conf, prometheusDefaultENV)

	if p.conf.Path == "" {
		p.conf.Path = "/metrics"
	}

	if p.conf.Addr == "" {
		return errors.New("Prometheus listen_addr not set")
	}

	p.log.Info("Starting prometheus listener on:", p.conf.Addr)

	http.Handle(p.conf.Path, promhttp.Handler())

	go func() {
		log.Fatal(http.ListenAndServe(p.conf.Addr, nil))
	}()
	p.log.Info(p.GetName() + " Initialized")

	return nil
}

func (p *PrometheusPump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Debug("Attempting to write ", len(data), " records...")

	totalStatusMetrics := make(map[string]float64)
	pathStatusMetrics := make(map[string]float64)
	keyStatusMetrics := make(map[string]float64)
	oauthStatusMetrics := make(map[string]float64)
	totalLatencyMetrics := make(map[string]float64)

	for i, item := range data {
		select {
		case <-ctx.Done():
			p.log.Warn("Purged ", i, " of ", len(data), " because of timeout.")
			return errors.New("prometheus pump couldn't write all the analytics records")
		default:
		}
		record := item.(analytics.AnalyticsRecord)
		code := strconv.Itoa(record.ResponseCode)

		totalStatusMetricsLabel := fmt.Sprintf("%v-%v", code, record.APIID)
		pathStatusMetricsLabel := fmt.Sprintf("%v-%v-%v-%v", code, record.APIID, record.Path, record.Method)
		keyStatusMetricsLabel := fmt.Sprintf("%v-%v", code, record.APIKey)
		oauthStatusMetricsLabel := fmt.Sprintf("%v-%v", code, record.OauthID)

		totalStatusMetrics[totalStatusMetricsLabel] += 1
		pathStatusMetrics[pathStatusMetricsLabel] += 1
		keyStatusMetrics[keyStatusMetricsLabel] += 1
		if record.OauthID != "" {
			oauthStatusMetrics[oauthStatusMetricsLabel] += 1
		}

		totalLatencyMetrics[record.APIID] = float64(record.RequestTime)
	}

	for k, v := range totalStatusMetrics {
		labels := strings.Split(k, "-")
		code := labels[0]
		apiId := labels[1]
		p.TotalStatusMetrics.WithLabelValues(code, apiId).Add(v)
	}

	for k, v := range pathStatusMetrics {
		labels := strings.Split(k, "-")
		code := labels[0]
		apiId := labels[1]
		recordPath := labels[2]
		recordMethod := labels[3]
		p.PathStatusMetrics.WithLabelValues(code, apiId, recordPath, recordMethod).Add(v)
	}

	for k, v := range keyStatusMetrics {
		labels := strings.Split(k, "-")
		code := labels[0]
		apiKey := labels[1]
		p.KeyStatusMetrics.WithLabelValues(code, apiKey).Add(v)
	}

	for k, v := range oauthStatusMetrics {
		labels := strings.Split(k, "-")
		code := labels[0]
		oauthId := labels[1]
		p.OauthStatusMetrics.WithLabelValues(code, oauthId).Add(v)
	}

	for k, v := range totalLatencyMetrics {
		p.TotalLatencyMetrics.WithLabelValues("total", k).Observe(v)
	}

	p.log.Info("Purged ", len(data), " records...")

	return nil
}
