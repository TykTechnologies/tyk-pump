package pumps

import (
	"context"
	"errors"
	"fmt"
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


	customMetrics []*PrometheusMetric


	CommonPumpConfig
}

// @PumpConf Prometheus
type PrometheusConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The full URL to your Prometheus instance, {HOST}:{PORT}. For example `localhost:9090`.
	Addr string `json:"listen_address" mapstructure:"listen_address"`
	// The path to the Prometheus collection. For example `/metrics`.
	Path string `json:"path" mapstructure:"path"`
	// Custom Prometheus metrics.
	CustomMetrics []PrometheusMetric `json:"custom_metrics" mapstructure:"custom_metrics"`
}

type PrometheusMetric struct {
	// The name of the custom metric. For example: `tyk_http_status_per_api_name`
	Name string  `json:"name" mapstructure:"name"`
	// Description text of the custom metric. For example: `HTTP status codes per API`
	Help string `json:"help" mapstructure:"help"`
	// Determines the type of the metric. There's currently 2 available options: `counter` or `histogram`.
	// In case of histogram, you can only modify the labels since it always going to use the request_time.
	MetricType string `json:"metric_type" mapstructure:"metric_type"`
	// Defines the buckets into which observations are counted. By default, []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	Buckets []float64 `json:"buckets" mapstructure:"buckets"`
	// Defines the partitions in the metrics. For example: ['response_code','api_name'].
	// The available labels are: `["host","method",
	// "path", "response_code", "api_key", "time_stamp", "api_version", "api_name", "api_id",
	// "org_id", "oauth_id","request_time", "ip_address"]`.
	Labels []string `json:"labels" mapstructure:"labels"`

	enabled bool
	counterVec *prometheus.CounterVec
	histogramVec *prometheus.HistogramVec
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

	if len(p.conf.CustomMetrics) > 0 {
		for _, metric := range p.conf.CustomMetrics{
			newMetric := &metric
			errInit := newMetric.InitVec()
			if errInit != nil {
				p.log.Error(errInit)
			}else{
				p.customMetrics = append(p.customMetrics, newMetric)
			}
		}
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


		for _, customMetric := range p.customMetrics{
			if customMetric.enabled {
				p.log.Debug("Processing metric:", customMetric.Name)

				switch customMetric.MetricType {
				case "counter":
					if customMetric.counterVec != nil {
						values := customMetric.GetLabelsValues(record)
						customMetric.counterVec.WithLabelValues(values...).Inc()
					}
				case "histogram":
					if customMetric.histogramVec != nil {
						values := customMetric.GetLabelsValues(record)
						customMetric.histogramVec.WithLabelValues(values...).Observe(float64(record.RequestTime))
					}
				default:
				}
			}else{
				p.log.Info("DISABLED")
			}
		}
	}
	p.log.Info("Purged ", len(data), " records...")

	return nil
}


// InitVec inits the prometheus metric based on the metric_type. It only can create counter and histogram,
// if the metric_type is anything else it returns an error
func (pm *PrometheusMetric) InitVec() error {
	if pm.MetricType == "counter"{
		pm.counterVec = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: pm.Name,
				Help: pm.Help,
			},
			pm.Labels,
		)
		prometheus.MustRegister(pm.counterVec)
	}else if pm.MetricType == "histogram"{
		pm.histogramVec = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: pm.Name,
				Help: pm.Help,
				Buckets: buckets,
			},
			pm.Labels,
		)
		prometheus.MustRegister(pm.histogramVec)
	}else{
		return errors.New("invalid metric type:"+ pm.MetricType)
	}

	pm.enabled = true
	return nil
}

// GetLabelsValues return a list of string values based on the custom metric labels.
func (pm *PrometheusMetric) GetLabelsValues(decoded analytics.AnalyticsRecord) []string{
	values := []string{}
	mapping := map[string]interface{}{
		"host":			decoded.Host,
		"method":        decoded.Method,
		"path":          decoded.Path,
		"response_code": decoded.ResponseCode,
		"api_key":       decoded.APIKey,
		"time_stamp":    decoded.TimeStamp,
		"api_version":   decoded.APIVersion,
		"api_name":      decoded.APIName,
		"api_id":        decoded.APIID,
		"org_id":        decoded.OrgID,
		"oauth_id":      decoded.OauthID,
		"request_time":  decoded.RequestTime,
		"ip_address":    decoded.IPAddress,
	}

	for _, label := range pm.Labels{
		if val, ok := mapping[label]; ok {
			values = append(values, fmt.Sprint(val))
		}
	}
	return values
}