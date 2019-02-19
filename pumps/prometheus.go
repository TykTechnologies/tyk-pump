package pumps

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/TykTechnologies/logrus"
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
}

type PrometheusConf struct {
	Addr string `mapstructure:"listen_address"`
	Path string `mapstructure:"path"`
}

var prometheusPrefix = "prometheus-pump"

var prometheusLogger = log.WithFields(logrus.Fields{"prefix": prometheusPrefix})

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

func (p *PrometheusPump) Init(conf interface{}) error {
	p.conf = &PrometheusConf{}
	err := mapstructure.Decode(conf, &p.conf)
	if err != nil {
		prometheusLogger.Fatal("Failed to decode configuration: ", err)
	}

	if p.conf.Path == "" {
		p.conf.Path = "/metrics"
	}

	if p.conf.Addr == "" {
		return errors.New("Prometheus listen_addr not set")
	}

	prometheusLogger.Info("Starting prometheus listener on:", p.conf.Addr)

	http.Handle(p.conf.Path, promhttp.Handler())

	go func() {
		log.Fatal(http.ListenAndServe(p.conf.Addr, nil))
	}()

	return nil
}

func (p *PrometheusPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": graylogPrefix,
	}).Debug("Writing ", len(data), " records")

	for _, item := range data {
		record := item.(analytics.AnalyticsRecord)
		code := strconv.Itoa(record.ResponseCode)

		p.TotalStatusMetrics.WithLabelValues(code, record.APIID).Inc()
		p.PathStatusMetrics.WithLabelValues(code, record.APIID, record.Path, record.Method).Inc()
		p.KeyStatusMetrics.WithLabelValues(code, record.APIKey)
		if record.OauthID != "" {
			p.OauthStatusMetrics.WithLabelValues(code, record.OauthID).Inc()
		}
		p.TotalLatencyMetrics.WithLabelValues("total", record.APIID).Observe(float64(record.RequestTime))
	}
	return nil
}
