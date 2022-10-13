package pumps

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"

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

	allMetrics []*PrometheusMetric

	CommonPumpConfig
}

// @PumpConf Prometheus
type PrometheusConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The full URL to your Prometheus instance, {HOST}:{PORT}. For example `localhost:9090`.
	Addr string `json:"listen_address" mapstructure:"listen_address"`
	// The path to the Prometheus collection. For example `/metrics`.
	Path string `json:"path" mapstructure:"path"`
	// This will enable an experimental feature that will aggregate the histogram metrics request time values before exposing them to prometheus.
	// Enabling this will reduce the CPU usage of your prometheus pump but you will loose histogram precision. Experimental.
	AggregateObservations bool `json:"aggregate_observations" mapstructure:"aggregate_observations"`
	// Metrics to exclude from exposition. Currently, excludes only the base metrics.
	DisabledMetrics []string `json:"disabled_metrics" mapstructure:"disabled_metrics"`
	// Custom Prometheus metrics.
	CustomMetrics []PrometheusMetric `json:"custom_metrics" mapstructure:"custom_metrics"`
}

type PrometheusMetric struct {
	// The name of the custom metric. For example: `tyk_http_status_per_api_name`
	Name string `json:"name" mapstructure:"name"`
	// Description text of the custom metric. For example: `HTTP status codes per API`
	Help string `json:"help" mapstructure:"help"`
	// Determines the type of the metric. There's currently 2 available options: `counter` or `histogram`.
	// In case of histogram, you can only modify the labels since it always going to use the request_time.
	MetricType string `json:"metric_type" mapstructure:"metric_type"`
	// Defines the buckets into which observations are counted. The type is float64 array and by default, [1, 2, 5, 7, 10, 15, 20, 25, 30, 40, 50, 60, 70, 80, 90, 100, 200, 300, 400, 500, 1000, 2000, 5000, 10000, 30000, 60000]
	Buckets []float64 `json:"buckets" mapstructure:"buckets"`
	// Defines the partitions in the metrics. For example: ['response_code','api_name'].
	// The available labels are: `["host","method",
	// "path", "response_code", "api_key", "time_stamp", "api_version", "api_name", "api_id",
	// "org_id", "oauth_id","request_time", "ip_address", "alias"]`.
	Labels []string `json:"labels" mapstructure:"labels"`

	enabled      bool
	counterVec   *prometheus.CounterVec
	histogramVec *prometheus.HistogramVec

	counterMap map[string]uint64

	histogramMap           map[string]histogramCounter
	aggregatedObservations bool
}

// histogramCounter is a helper struct to mantain the totalRequestTime and hits in memory
type histogramCounter struct {
	totalRequestTime uint64
	hits             uint64
}

const COUNTER_TYPE = "counter"
const HISTOGRAM_TYPE = "histogram"

var prometheusPrefix = "prometheus-pump"
var prometheusDefaultENV = PUMPS_ENV_PREFIX + "_PROMETHEUS"

var buckets = []float64{1, 2, 5, 7, 10, 15, 20, 25, 30, 40, 50, 60, 70, 80, 90, 100, 200, 300, 400, 500, 1000, 2000, 5000, 10000, 30000, 60000}

func (p *PrometheusPump) New() Pump {
	newPump := PrometheusPump{}

	newPump.CreateBasicMetrics()

	return &newPump
}

// CreateBasicMetrics stores all the predefined pump metrics in allMetrics slice
func (p *PrometheusPump) CreateBasicMetrics() {

	//counter metrics
	totalStatusMetric := &PrometheusMetric{
		Name:       "tyk_http_status",
		Help:       "HTTP status codes per API",
		MetricType: COUNTER_TYPE,
		Labels:     []string{"code", "api"},
	}
	pathStatusMetrics := &PrometheusMetric{
		Name:       "tyk_http_status_per_path",
		Help:       "HTTP status codes per API path and method",
		MetricType: COUNTER_TYPE,
		Labels:     []string{"code", "api", "path", "method"},
	}
	keyStatusMetrics := &PrometheusMetric{
		Name:       "tyk_http_status_per_key",
		Help:       "HTTP status codes per API key",
		MetricType: COUNTER_TYPE,
		Labels:     []string{"code", "key"},
	}
	oauthStatusMetrics := &PrometheusMetric{
		Name:       "tyk_http_status_per_oauth_client",
		Help:       "HTTP status codes per oAuth client id",
		MetricType: COUNTER_TYPE,
		Labels:     []string{"code", "client_id"},
	}

	//histogram metrics
	totalLatencyMetrics := &PrometheusMetric{
		Name:       "tyk_latency",
		Help:       "Latency added by Tyk, Total Latency, and upstream latency per API",
		MetricType: HISTOGRAM_TYPE,
		Buckets:    buckets,
		Labels:     []string{"type", "api"},
	}

	p.allMetrics = append(p.allMetrics, totalStatusMetric, pathStatusMetrics, keyStatusMetrics, oauthStatusMetrics, totalLatencyMetrics)
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

	//first we init the base metrics
	p.initBaseMetrics()

	//then we check the custom ones
	p.InitCustomMetrics()

	p.log.Info("Starting prometheus listener on:", p.conf.Addr)

	http.Handle(p.conf.Path, promhttp.Handler())

	go func() {
		log.Fatal(http.ListenAndServe(p.conf.Addr, nil))
	}()
	p.log.Info(p.GetName() + " Initialized")

	return nil
}

func (p *PrometheusPump) initBaseMetrics() {
	toDisableSet := map[string]struct{}{}
	for _, metric := range p.conf.DisabledMetrics {
		toDisableSet[metric] = struct{}{}
	}
	// exclude disabled base metrics if needed. This disables exposition entirely during scrapes.
	trimmedAllMetrics := make([]*PrometheusMetric, 0, len(p.allMetrics))
	for _, metric := range p.allMetrics {
		if _, isDisabled := toDisableSet[metric.Name]; isDisabled {
			continue
		}
		metric.aggregatedObservations = p.conf.AggregateObservations
		if errInit := metric.InitVec(); errInit != nil {
			p.log.Error(errInit)
		}
		trimmedAllMetrics = append(trimmedAllMetrics, metric)
	}
	p.allMetrics = trimmedAllMetrics
}

// InitCustomMetrics initialise custom prometheus metrics based on p.conf.CustomMetrics and add them into p.allMetrics
func (p *PrometheusPump) InitCustomMetrics() {
	if len(p.conf.CustomMetrics) > 0 {
		customMetrics := []*PrometheusMetric{}
		for i := range p.conf.CustomMetrics {
			newMetric := &p.conf.CustomMetrics[i]
			newMetric.aggregatedObservations = p.conf.AggregateObservations
			errInit := newMetric.InitVec()
			if errInit != nil {
				p.log.Error("there was an error initialising custom prometheus metric ", newMetric.Name, " error:", errInit)
			} else {
				p.log.Info("added custom prometheus metric:", newMetric.Name)
				customMetrics = append(customMetrics, newMetric)
			}
		}

		p.allMetrics = append(p.allMetrics, customMetrics...)
	}
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
		//we loop through all the metrics avaialble.
		for _, metric := range p.allMetrics {
			if metric.enabled {
				p.log.Debug("Processing metric:", metric.Name)
				//we get the values for that metric required labels
				values := metric.GetLabelsValues(record)

				switch metric.MetricType {
				case COUNTER_TYPE:
					if metric.counterVec != nil {
						//if the metric is a counter, we increment the counter memory map
						err := metric.Inc(values...)
						if err != nil {
							p.log.WithFields(logrus.Fields{
								"metric_type": metric.MetricType,
								"metric_name": metric.Name,
							}).Error("error incrementing prometheus metric value:", err)
						}
					}
				case HISTOGRAM_TYPE:
					if metric.histogramVec != nil {
						//if the metric is an histogram, we Observe the request time with the given values
						err := metric.Observe(record.RequestTime, values...)
						if err != nil {
							p.log.WithFields(logrus.Fields{
								"metric_type": metric.MetricType,
								"metric_name": metric.Name,
							}).Error("error incrementing prometheus metric value:", err)
						}
					}
				default:
					p.log.Debug("trying to process an invalid prometheus metric type:", metric.MetricType)
				}
			}
		}
	}

	//after looping through all the analytics records, we expose the metrics to prometheus endpoint
	for _, customMetric := range p.allMetrics {
		err := customMetric.Expose()
		if err != nil {
			p.log.WithFields(logrus.Fields{
				"metric_type": customMetric.MetricType,
				"metric_name": customMetric.Name,
			}).Error("error writing prometheus metric:", err)
		}
	}

	p.log.Info("Purged ", len(data), " records...")

	return nil
}

// InitVec inits the prometheus metric based on the metric_type. It only can create counter and histogram,
// if the metric_type is anything else it returns an error
func (pm *PrometheusMetric) InitVec() error {
	if pm.MetricType == "counter" {
		pm.counterVec = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: pm.Name,
				Help: pm.Help,
			},
			pm.Labels,
		)
		pm.counterMap = make(map[string]uint64)
		prometheus.MustRegister(pm.counterVec)
	} else if pm.MetricType == "histogram" {
		bkts := pm.Buckets
		if len(bkts) == 0 {
			bkts = buckets
		}
		pm.histogramVec = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    pm.Name,
				Help:    pm.Help,
				Buckets: buckets,
			},
			pm.Labels,
		)
		pm.histogramMap = make(map[string]histogramCounter)
		prometheus.MustRegister(pm.histogramVec)
	} else {
		return errors.New("invalid metric type:" + pm.MetricType)
	}

	pm.enabled = true
	return nil
}

// GetLabelsValues return a list of string values based on the custom metric labels.
func (pm *PrometheusMetric) GetLabelsValues(decoded analytics.AnalyticsRecord) []string {
	values := []string{}
	mapping := map[string]interface{}{
		"host":          decoded.Host,
		"method":        decoded.Method,
		"path":          decoded.Path,
		"code":          decoded.ResponseCode,
		"response_code": decoded.ResponseCode,
		"api_key":       decoded.APIKey,
		"key":           decoded.APIKey,
		"time_stamp":    decoded.TimeStamp,
		"api_version":   decoded.APIVersion,
		"api_name":      decoded.APIName,
		"api":           decoded.APIID,
		"api_id":        decoded.APIID,
		"org_id":        decoded.OrgID,
		"client_id":     decoded.OauthID,
		"oauth_id":      decoded.OauthID,
		"request_time":  decoded.RequestTime,
		"ip_address":    decoded.IPAddress,
		"alias":         decoded.Alias,
	}

	for _, label := range pm.Labels {
		if val, ok := mapping[label]; ok {
			values = append(values, fmt.Sprint(val))
		}
	}
	return values
}

// Inc is going to fill counterMap and histogramMap with the data from record.
func (pm *PrometheusMetric) Inc(values ...string) error {
	switch pm.MetricType {
	case COUNTER_TYPE:
		// "response_code", "api_name", "method"
		// key = map[500--apitest-GET] = 4

		//map[]

		pm.counterMap[strings.Join(values, "--")] += 1
	default:
		return errors.New("invalid metric type:" + pm.MetricType)
	}

	return nil
}

// Observe will fill hitogramMap with the sum of totalRequest and hits per label value if aggregate_observations is true. If aggregate_observations is set to false (default) it will execute prometheus Observe directly.
func (pm *PrometheusMetric) Observe(requestTime int64, values ...string) error {
	switch pm.MetricType {
	case HISTOGRAM_TYPE:
		labelValues := []string{"total"}
		labelValues = append(labelValues, values...)
		if pm.aggregatedObservations {
			key := strings.Join(labelValues, "--")

			if currentValue, ok := pm.histogramMap[key]; ok {
				currentValue.hits += 1
				currentValue.totalRequestTime += uint64(requestTime)
				pm.histogramMap[key] = currentValue
			} else {
				pm.histogramMap[key] = histogramCounter{
					hits:             1,
					totalRequestTime: uint64(requestTime),
				}
			}
		} else {
			pm.histogramVec.WithLabelValues(labelValues...).Observe(float64(requestTime))
		}

	default:
		return errors.New("invalid metric type:" + pm.MetricType)
	}
	return nil
}

// Expose executes prometheus library functions using the counter/histogram vector from the PrometheusMetric struct.
// If the PrometheusMetric is COUNTER_TYPE, it will execute prometheus client Add function to add the counters from counterMap to the labels value metric
// If the PrometheusMetric is HISTOGRAM_TYPE and aggregate_observations config is true, it will calculate the average value of the metrics in the histogramMap and execute prometheus Observe.
// If aggregate_observations is false, it won't do anything since it means that we already exposed the metric.
func (pm *PrometheusMetric) Expose() error {
	switch pm.MetricType {
	case COUNTER_TYPE:
		for key, value := range pm.counterMap {

			labelsValue := strings.Split(key, "--")
			pm.counterVec.WithLabelValues(labelsValue...).Add(float64(value))
		}
		pm.counterMap = make(map[string]uint64)
	case HISTOGRAM_TYPE:
		if pm.aggregatedObservations {
			for key, value := range pm.histogramMap {
				labelsValue := strings.Split(key, "--")
				pm.histogramVec.WithLabelValues(labelsValue...).Observe(value.getAverageRequestTime())
			}
			pm.histogramMap = make(map[string]histogramCounter)
		}
	default:
		return errors.New("invalid metric type:" + pm.MetricType)
	}
	return nil
}

// getAverageRequestTime returns the average request time of an histogramCounter dividing the sum of all the RequestTimes by the hits.
func (c histogramCounter) getAverageRequestTime() float64 {
	return float64(c.totalRequestTime / c.hits)
}
