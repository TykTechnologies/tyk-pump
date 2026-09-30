package otel

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
)

// AttrDeploymentEnvironment is the standard OTel deployment environment
// resource attribute key.
const AttrDeploymentEnvironment = "deployment.environment"

// Identity carries the values that identify this Pump process on every
// exported metric.
//
// There is deliberately no control plane id: the Pump runs either next to the
// Dashboard (control plane) or in a data plane with the hybrid pump, so a
// control-plane id would be wrong half the time. service.instance.id and
// deployment.environment identify the process.
type Identity struct {
	// InstanceID becomes service.instance.id (use otel.InstanceID()).
	InstanceID string
	// Version becomes service.version.
	Version string
}

var (
	instanceIDOnce sync.Once
	instanceID     string
)

// InstanceID returns the identifier for this Pump process, used as the
// service.instance.id resource attribute. Like the Gateway's node id it is a
// UUID generated once at boot, so every restart yields a new id; the value is
// stable for the lifetime of the process.
func InstanceID() string {
	instanceIDOnce.Do(func() {
		id, err := uuid.NewV4()
		if err != nil {
			// Only possible if the system's random source fails. A time-based
			// v1 UUID is still unique per process and keeps boot going.
			id = uuid.Must(uuid.NewV1())
		}
		instanceID = id.String()
	})
	return instanceID
}

// InitMetrics initializes the OTel metrics provider from config and returns a
// nil-safe MetricInstruments container. It never returns nil and never panics:
// if metrics are disabled or the config is invalid, the container wraps a noop
// provider. A disabled config logs one debug line; an invalid one logs exactly
// one warning naming the problem. cfg is taken by value so defaulting never
// mutates the caller's config. extraOpts is used by tests to inject a manual
// reader; production callers pass none.
//
//nolint:gocritic // cfg is by value on purpose: defaulting must not mutate the caller's config.
func InitMetrics(ctx context.Context, logger logrus.FieldLogger, cfg OpenTelemetry, id Identity, extraOpts ...tykmetric.Option) *MetricInstruments {
	cfg.SetDefaults()

	libLogger := logger.WithFields(logrus.Fields{
		"prefix":          "otel-metrics",
		"exporter":        cfg.Metrics.Exporter,
		"endpoint":        cfg.Metrics.Endpoint,
		"export_interval": cfg.Metrics.ExportInterval,
	})

	opts := []tykmetric.Option{
		tykmetric.WithContext(ctx),
		tykmetric.WithConfig(&cfg.Metrics.BaseMetricsConfig),
		tykmetric.WithLogger(libLogger),
		// InitMetrics logs the init failure itself; without this the library
		// would log the same error a second time.
		tykmetric.WithQuietInitErrors(),
		tykmetric.WithServiceID(id.InstanceID),
		tykmetric.WithServiceVersion(id.Version),
		tykmetric.WithHostDetector(),
		tykmetric.WithContainerDetector(),
		tykmetric.WithProcessDetector(),
		tykmetric.WithCustomResourceAttributes(
			tykmetric.StringAttribute(AttrDeploymentEnvironment, cfg.Metrics.DeploymentEnvironment),
		),
	}
	opts = append(opts, extraOpts...)

	provider, err := tykmetric.NewProvider(opts...)
	if err != nil {
		// The library still returns a safe noop provider alongside the error.
		logger.WithError(err).Warn("OpenTelemetry metrics disabled: provider initialization failed")
	} else if !provider.Enabled() {
		logger.Debug("OpenTelemetry metrics disabled")
	}

	return NewMetricInstruments(provider, logger, cfg)
}

// ShutdownTimeout returns how long the shutdown flush may take for cfg:
// `shutdown_timeout` seconds, after defaults are applied.
//
//nolint:gocritic // cfg is by value on purpose: defaulting must not mutate the caller's config.
func ShutdownTimeout(cfg OpenTelemetry) time.Duration {
	cfg.SetDefaults()
	return time.Duration(cfg.Metrics.ShutdownTimeout) * time.Second
}

// active holds the process-wide Pump instruments so any package can record via
// otel.Metrics() without the container being passed around. It is an atomic
// pointer because SetActive runs at startup while other goroutines may already
// read through Metrics().
var active atomic.Pointer[MetricInstruments]

// noopInstruments is what Metrics() returns before SetActive runs (or after
// SetActive(nil)), so callers never see nil.
var noopInstruments = &MetricInstruments{}

// SetActive installs the process-wide instruments at startup. A nil argument
// installs an empty no-op container so Metrics() never returns nil.
func SetActive(i *MetricInstruments) {
	if i == nil {
		i = noopInstruments
	}
	active.Store(i)
}

// Metrics returns the process-wide instruments. Never nil and safe before
// SetActive: methods no-op until instruments exist.
func Metrics() *MetricInstruments {
	if m := active.Load(); m != nil {
		return m
	}
	return noopInstruments
}
