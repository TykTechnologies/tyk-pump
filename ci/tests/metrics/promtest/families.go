package promtest

import "slices"

// IdentityLabels are the resource attributes the Pump stamps on every export,
// as Prometheus label keys after the collector's
// resource_to_telemetry_conversion (dots become underscores). Every Pump
// metric family must carry them. There is deliberately no control_plane_id:
// the Pump runs in either plane.
var IdentityLabels = []string{
	"service_name",
	"service_instance_id",
	"service_version",
	"deployment_environment",
}

// ServiceName is the value of service_name on every Pump series.
const ServiceName = "tyk-pump"

// Families is the registry of metric families the Pump exports today. Each
// signal story appends its families here (and adds its own *_test.go);
// TestFamilies in the e2e suite checks every entry, so a renamed or dropped
// metric fails with a clear message.
var Families = []Family{
	{
		// Liveness gauge (TT-18517): the end-to-end proof that the OTLP
		// pipeline works.
		Name:     "process_uptime_seconds",
		OTelName: "process.uptime",
		Type:     "gauge",
		Labels:   IdentityLabels,
	},
	HealthFamily,
	// Pump write metrics (TT-18519). Checked under the "pumps" profile,
	// which configures pumps for every outcome and sends traffic.
	{
		Name:     "tyk_pump_initialized",
		OTelName: "tyk.pump.initialized",
		Type:     "gauge",
		Labels:   slices.Concat(IdentityLabels, []string{"pump", "pump_type"}),
		Profile:  PumpsProfile,
	},
	{
		Name:     "tyk_pump_writes_total",
		OTelName: "tyk.pump.writes",
		Type:     "counter",
		Labels:   slices.Concat(IdentityLabels, []string{"pump", "pump_type", "outcome"}),
		Profile:  PumpsProfile,
	},
	{
		Name:     "tyk_pump_write_records_total",
		OTelName: "tyk.pump.write.records",
		Type:     "counter",
		Labels:   slices.Concat(IdentityLabels, []string{"pump", "pump_type", "outcome"}),
		Profile:  PumpsProfile,
	},
	{
		Name:     "tyk_pump_purge_records_total",
		OTelName: "tyk.pump.purge.records",
		Type:     "counter",
		Labels:   slices.Concat(IdentityLabels, []string{"result"}),
		Profile:  PumpsProfile,
	},
}

// PumpsProfile is the e2e profile that exercises the pump write metrics.
const PumpsProfile = "pumps"

// HealthFamily is the per-dependency self-health gauge (TT-18518): one series
// per probed dependency, labelled with its role and engine. Registered without
// a unit, so Prometheus adds no suffix.
var HealthFamily = Family{
	Name:     "tyk_pump_health",
	OTelName: "tyk.pump.health",
	Type:     "gauge",
	Labels:   slices.Concat(IdentityLabels, []string{"component", "store"}),
}
