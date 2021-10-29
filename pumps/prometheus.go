package pumps

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/TykTechnologies/tyk-pump/analytics"

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
	// [ADD COMMENT]
	Addr      string `json:"listen_address" mapstructure:"listen_address"`
	// [ADD COMMENT]
	Path      string `json:"path" mapstructure:"path"`
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

	for i, item := range data {
		select {
		case <-ctx.Done():
			p.log.Warn("Purged ", i, " of ", len(data), " because of timeout.")
			return errors.New("prometheus pump couldn't write all the analytics records")
		default:
		}
		record := item.(analytics.AnalyticsRecord)
		code := strconv.Itoa(record.ResponseCode)

		p.TotalStatusMetrics.WithLabelValues(code, record.APIID).Inc()
		p.PathStatusMetrics.WithLabelValues(code, record.APIID, record.Path, record.Method).Inc()
		p.KeyStatusMetrics.WithLabelValues(code, record.APIKey).Inc()
		if record.OauthID != "" {
			p.OauthStatusMetrics.WithLabelValues(code, record.OauthID).Inc()
		}
		p.TotalLatencyMetrics.WithLabelValues("total", record.APIID).Observe(float64(record.RequestTime))
	}
	p.log.Info("Purged ", len(data), " records...")

	return nil
}
