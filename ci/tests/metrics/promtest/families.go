package promtest

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
}
