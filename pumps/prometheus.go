package pumps

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "tyk"
)

var (
	metricSet = []string{
		"http_status",
		"http_status_per_path",
		"http_status_per_key",
		"http_status_per_oauth_client",
		"total_latency_seconds",
		"upstream_latency_seconds",
		"gateway_latency_seconds",
	}
)

type PrometheusPump struct {
	conf *PrometheusConf
	// Per service
	TotalStatusMetrics  *prometheus.CounterVec
	PathStatusMetrics   *prometheus.CounterVec
	KeyStatusMetrics    *prometheus.CounterVec
	OauthStatusMetrics  *prometheus.CounterVec
	TotalLatencyMetrics *prometheus.HistogramVec
	UpstreamLatencyHist *prometheus.HistogramVec
	GatewayLatencyHist  *prometheus.HistogramVec

	filters analytics.AnalyticsFilters
	timeout int
}

type PrometheusConf struct {
	Addr    string    `mapstructure:"listen_address"`
	Path    string    `mapstructure:"path"`
	Metrics []string  `mapstructure:"metrics"` // whitelist of metrics we want to expose
	Buckets []float64 `mapstructure:"buckets"`
}

var (
	prometheusPrefix = "prometheus-pump"
	prometheusLogger = log.WithFields(logrus.Fields{"prefix": prometheusPrefix})
	buckets          = []float64{
		0.000,
		0.010,
		0.020,
		0.030,
		0.040,
		0.050,
		0.060,
		0.070,
		0.080,
		0.090,
		0.100,
		0.200,
		0.300,
		0.400,
		0.500,
		0.600,
		0.700,
		0.800,
		0.900,
		1.000,
		2.000,
		3.000,
		4.000,
		5.000,
		6.000,
		7.000,
		8.000,
		9.000,
		10.00,
	}
)

func (p *PrometheusPump) New() Pump {
	newPump := PrometheusPump{}

	return &newPump
}

func (p *PrometheusPump) GetName() string {
	return "Prometheus Pump"
}

func (p *PrometheusPump) Init(conf interface{}) error {
	p.conf = &PrometheusConf{}
	err := mapstructure.Decode(conf, &p.conf)
	if err != nil {
		prometheusLogger.WithError(err).Fatal("failed to decode configuration")
	}

	if p.conf.Path == "" {
		p.conf.Path = "/metrics"
	}

	if p.conf.Addr == "" {
		return errors.New("prometheus listen_addr not set")
	}

	if len(p.conf.Buckets) == 0 {
		prometheusLogger.Info("no buckets specified, using default buckets")
		p.conf.Buckets = buckets
	}

	if len(p.conf.Metrics) == 0 {
		prometheusLogger.Infof("no metrics whitelisted, all enabled: %v", metricSet)
		p.conf.Metrics = metricSet
	} else {
		var metricsToRecord []string
		for _, metric := range p.conf.Metrics {
			for _, validMetric := range metricSet {
				if metric == validMetric {
					metricsToRecord = append(metricsToRecord, validMetric)
				}
			}
		}

		p.conf.Metrics = metricsToRecord
		prometheusLogger.Infof("publishing metrics: %q", metricsToRecord)
	}

	p.registerMetrics()

	prometheusLogger.Infof("starting prometheus listener: %s%s", p.conf.Addr, p.conf.Path)

	http.Handle(p.conf.Path, promhttp.Handler())

	go func() {
		log.Fatal(http.ListenAndServe(p.conf.Addr, nil))
	}()

	return nil
}

func (p *PrometheusPump) registerMetrics() {
	for _, metric := range p.conf.Metrics {
		switch metric {
		case "http_status":
			p.TotalStatusMetrics = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: namespace,
					Subsystem: "http",
					Name:      "status",
					Help:      "HTTP status codes by API",
				},
				[]string{"code", "api", "api_name", "api_version"},
			)
			prometheus.MustRegister(p.TotalStatusMetrics)
		case "http_status_per_path":
			p.PathStatusMetrics = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: namespace,
					Subsystem: "http",
					Name:      "status_per_path",
					Help:      "HTTP status codes per API path and method",
				},
				[]string{"code", "api", "api_name", "api_version", "path", "method"},
			)
			prometheus.MustRegister(p.PathStatusMetrics)
		case "http_status_per_key":
			// Useful to discover if a particular API Key has bad credentials.
			// Need to show the hashed key rather than plaintext key.
			p.KeyStatusMetrics = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: namespace,
					Subsystem: "http",
					Name:      "status_per_key",
					Help:      "HTTP status codes per API key",
				},
				[]string{"code", "key", "alias"},
			)
			prometheus.MustRegister(p.KeyStatusMetrics)
		case "tyk_http_status_per_oauth_client":
			p.OauthStatusMetrics = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tyk_http_status_per_oauth_client",
					Help: "HTTP status codes per oAuth client id",
				},
				[]string{"code", "client_id"},
			)
			prometheus.MustRegister(p.OauthStatusMetrics)
		case "total_latency_seconds":
			p.TotalLatencyMetrics = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: namespace,
					Subsystem: "http",
					Name:      "total_latency_seconds",
					Help:      "Total Latency in seconds by API",
					Buckets:   p.conf.Buckets,
				},
				[]string{"api"},
			)
			prometheus.MustRegister(p.TotalLatencyMetrics)
		case "upstream_latency_seconds":
			p.UpstreamLatencyHist = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: namespace,
					Subsystem: "http",
					Name:      "upstream_latency_seconds",
					Help:      "Upstream latency in seconds by API",
					Buckets:   p.conf.Buckets,
				},
				[]string{"api"},
			)
			prometheus.MustRegister(p.UpstreamLatencyHist)
		case "gateway_latency_seconds":
			p.GatewayLatencyHist = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: namespace,
					Subsystem: "http",
					Name:      "gateway_latency_seconds",
					Help:      "Gateway Latency in seconds by API",
					Buckets:   p.conf.Buckets,
				},
				[]string{"api"},
			)
			prometheus.MustRegister(p.GatewayLatencyHist)
		}
	}
}

func (p *PrometheusPump) WriteData(_ context.Context, data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": prometheusPrefix,
	}).Debug("Writing ", len(data), " records")

	for _, item := range data {
		record := item.(analytics.AnalyticsRecord)
		code := strconv.Itoa(record.ResponseCode)

		if p.TotalStatusMetrics != nil {
			labels := prometheus.Labels{
				"code":        code,
				"api":         record.APIID,
				"api_name":    record.APIName,
				"api_version": record.APIVersion,
			}

			p.TotalStatusMetrics.With(labels).Inc()
		}

		if p.PathStatusMetrics != nil {
			labels := prometheus.Labels{
				"code":        code,
				"api":         record.APIID,
				"api_name":    record.APIName,
				"api_version": record.APIVersion,
				"path":        record.Path,
				"method":      record.Method,
			}
			p.PathStatusMetrics.With(labels).Inc()
		}

		if p.KeyStatusMetrics != nil {
			labels := prometheus.Labels{
				"code":  code,
				"key":   record.APIKey,
				"alias": record.Alias,
			}
			p.KeyStatusMetrics.With(labels).Inc()
		}

		if p.OauthStatusMetrics != nil {
			if record.OauthID != "" {
				labels := prometheus.Labels{
					"code":      code,
					"client_id": record.OauthID,
				}

				p.OauthStatusMetrics.With(labels).Inc()
			}
		}

		if p.TotalLatencyMetrics != nil {
			labels := prometheus.Labels{
				"api": record.APIID,
			}

			// https://prometheus.io/docs/practices/naming/#base-units
			// Prometheus does not have any units hard coded. For better compatibility, base units should be used.
			requestTime := time.Duration(record.Latency.Total) * time.Millisecond
			p.TotalLatencyMetrics.With(labels).Observe(requestTime.Seconds())
		}

		if p.UpstreamLatencyHist != nil {
			labels := prometheus.Labels{
				"api": record.APIID,
			}

			// https://prometheus.io/docs/practices/naming/#base-units
			// Prometheus does not have any units hard coded. For better compatibility, base units should be used.
			requestTime := time.Duration(record.Latency.Upstream) * time.Millisecond
			p.UpstreamLatencyHist.With(labels).Observe(requestTime.Seconds())
		}

		if p.GatewayLatencyHist != nil {
			labels := prometheus.Labels{
				"api": record.APIID,
			}

			// https://prometheus.io/docs/practices/naming/#base-units
			// Prometheus does not have any units hard coded. For better compatibility, base units should be used.
			requestTime := time.Duration(record.Latency.Total)*time.Millisecond - time.Duration(record.Latency.Upstream)*time.Millisecond
			p.GatewayLatencyHist.With(labels).Observe(requestTime.Seconds())
		}
	}
	return nil
}

func (p *PrometheusPump) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *PrometheusPump) GetFilters() analytics.AnalyticsFilters {
	return p.filters
}

func (p *PrometheusPump) SetTimeout(timeout int) {
	p.timeout = timeout
}

func (p *PrometheusPump) GetTimeout() int {
	return p.timeout
}
