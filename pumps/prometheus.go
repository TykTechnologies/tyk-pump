package pumps

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "tyk"
)

type PrometheusPump struct {
	conf *PrometheusConf
	// Per service
	TotalStatusMetrics    *prometheus.CounterVec
	PathStatusMetrics     *prometheus.CounterVec
	KeyStatusMetrics      *prometheus.CounterVec
	OauthStatusMetrics    *prometheus.CounterVec
	TotalLatencyMetrics   *prometheus.HistogramVec
	LatencyTykMetrics     *prometheus.HistogramVec
	LatencyServiceMetrics *prometheus.HistogramVec
	LatencyTotalMetrics   *prometheus.HistogramVec

	filters analytics.AnalyticsFilters
	timeout int
}

type PrometheusConf struct {
	Addr    string   `mapstructure:"listen_address"`
	Path    string   `mapstructure:"path"`
	Metrics []string `mapstructure:"metrics"` // whitelist of metrics we want to expose
}

var (
	prometheusPrefix = "prometheus-pump"
	prometheusLogger = log.WithFields(logrus.Fields{"prefix": prometheusPrefix})
	buckets          = []float64{1, 2, 5, 7, 10, 15, 20, 25, 30, 40, 50, 60, 70, 80, 90, 100, 200, 300, 400, 500, 1000, 2000, 5000, 10000, 30000, 60000}
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
		prometheusLogger.Fatal("Failed to decode configuration: ", err)
	}

	if p.conf.Path == "" {
		p.conf.Path = "/metrics"
	}

	if p.conf.Addr == "" {
		return errors.New("Prometheus listen_addr not set")
	}

	metricSet := []string{
		"tyk_http_status",
		"tyk_http_status_per_path",
		"tyk_http_status_per_key",
		"tyk_http_status_per_oauth_client",
		"tyk_latency", // we should deprecate this
		"tyk_latency_total",
		"tyk_latency_tyk",
		"tyk_latency_service",
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
		prometheusLogger.Infof("publishing: %v", p.conf.Metrics)
	}

	p.registerMetrics()

	prometheusLogger.Info("Starting prometheus listener on:", p.conf.Addr)

	http.Handle(p.conf.Path, promhttp.Handler())

	go func() {
		log.Fatal(http.ListenAndServe(p.conf.Addr, nil))
	}()

	return nil
}

func (p *PrometheusPump) registerMetrics() {
	for _, metric := range p.conf.Metrics {
		switch metric {
		case "tyk_http_status":
			p.TotalStatusMetrics = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: namespace,
					Name:      "http_status",
					Help:      "HTTP status codes by API",
				},
				[]string{"code", "api", "api_name", "api_version"},
			)
			prometheus.MustRegister(p.TotalStatusMetrics)
		case "tyk_http_status_per_path":
			p.PathStatusMetrics = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tyk_http_status_per_path",
					Help: "HTTP status codes per API path and method",
				},
				[]string{"code", "api", "path", "method"},
			)
			prometheus.MustRegister(p.PathStatusMetrics)
		case "tyk_http_status_per_key":
			p.KeyStatusMetrics = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "tyk_http_status_per_key",
					Help: "HTTP status codes per API key",
				},
				[]string{"code", "key"},
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
		case "tyk_latency":
			// TODO: We should deprecate this because it is mislabeled
			p.TotalLatencyMetrics = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "tyk_latency",
					Help:    "Latency added by Tyk, Total Latency, and upstream latency per API",
					Buckets: buckets,
				},
				[]string{"type", "api"},
			)
			prometheus.MustRegister(p.TotalLatencyMetrics)
		case "latency_total":
			p.LatencyTotalMetrics = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "latency_total",
					Help:    "Total Latency, Tyk + service",
					Buckets: buckets,
				},
				[]string{"type", "api"},
			)
			prometheus.MustRegister(p.LatencyTotalMetrics)
		case "latency_tyk":
			p.LatencyTykMetrics = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "latency_tyk",
					Help:    "Latency added by Tyk",
					Buckets: buckets,
				},
				[]string{"type", "api"},
			)
			prometheus.MustRegister(p.LatencyTykMetrics)
		case "latency_service":
			p.LatencyServiceMetrics = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "latency_service",
					Help:    "Latency of the underlying service or API",
					Buckets: buckets,
				},
				[]string{"type", "api"},
			)
			prometheus.MustRegister(p.LatencyServiceMetrics)
		}
	}
}

func (p *PrometheusPump) WriteData(ctx context.Context, data []interface{}) error {
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
			tags := []string{code, record.APIID, record.Path, record.Method}
			tags = append(tags, record.Tags...)
			p.PathStatusMetrics.WithLabelValues(tags...).Inc()
		}

		if p.KeyStatusMetrics != nil {
			tags := []string{code, record.APIKey}
			tags = append(tags, record.Tags...)
			p.KeyStatusMetrics.WithLabelValues(tags...)
		}

		if p.OauthStatusMetrics != nil {
			if record.OauthID != "" {
				tags := []string{code, record.OauthID}
				tags = append(tags, record.Tags...)
				p.OauthStatusMetrics.WithLabelValues(code, record.OauthID).Inc()
			}
		}

		if p.TotalLatencyMetrics != nil {
			tags := []string{"total", record.APIID}
			tags = append(tags, record.Tags...)
			p.TotalLatencyMetrics.WithLabelValues(tags...).Observe(float64(record.RequestTime))
		}

		if p.LatencyTykMetrics != nil {
			tags := []string{"total", record.APIID}
			tags = append(tags, record.Tags...)
			p.LatencyTykMetrics.WithLabelValues(tags...).Observe(float64(record.RequestTime))
		}

		if p.LatencyServiceMetrics != nil {
			tags := []string{"total", record.APIID}
			tags = append(tags, record.Tags...)
			p.LatencyServiceMetrics.WithLabelValues(tags...).Observe(float64(record.RequestTime))
		}

		if p.LatencyTotalMetrics != nil {
			tags := []string{"total", record.APIID}
			tags = append(tags, record.Tags...)
			p.LatencyTotalMetrics.WithLabelValues(tags...).Observe(float64(record.RequestTime))
		}
	}
	return nil
}

func (p *PrometheusPump) normalizeTagsToLabels(tags []string) prometheus.Labels {
	labels := prometheus.Labels{}

	for _, tag := range tags {
		t := strings.SplitN(tag, "-", 2)
		if len(t) != 2 {
			continue
		}
		labels[t[0]] = t[1]
	}

	return labels
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
