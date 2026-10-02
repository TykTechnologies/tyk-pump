package metrics

import (
	"regexp"
	"testing"

	"github.com/TykTechnologies/tyk-pump/ci/tests/metrics/promtest"
)

var (
	uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	// versionLogRe matches the version banner the Pump logs at startup.
	versionLogRe = regexp.MustCompile(`## Tyk Pump, (\S+) ##`)
	// instanceIDLogRe matches the startup line announcing metrics export.
	instanceIDLogRe = regexp.MustCompile(`OpenTelemetry metrics enabled: exporter=\S+ endpoint=\S+ instance_id=([0-9a-f-]+)`)
)

// TestIdentity asserts the resource attributes every Pump export carries, as
// they surface in Prometheus:
//   - as labels on each Pump series (resource_to_telemetry_conversion),
//   - and on the collector's target_info metric.
func TestIdentity(t *testing.T) {
	requireProfile(t, "default")
	waitForPump(t)

	series := promtest.WaitForSeries(t, prom, pumpSelector(uptimeFamily), pollTimeout, pollInterval)
	s := series[0]

	t.Run("resource attributes on metrics", func(t *testing.T) {
		if missing := s.MissingLabels(promtest.IdentityLabels...); len(missing) > 0 {
			t.Fatalf("series %s is missing identity labels %v", s.LabelsString(), missing)
		}
		expectLabel(t, s, "service_name", promtest.ServiceName)
		// From profiles/default/pump.env.
		expectLabel(t, s, "deployment_environment", "e2e")

		if id := s.Labels["service_instance_id"]; !uuidRe.MatchString(id) {
			t.Errorf("service_instance_id = %q, want a UUID (fresh per process)", id)
		}
		if _, ok := s.Labels["control_plane_id"]; ok {
			t.Errorf("series %s carries control_plane_id; the Pump must not export one", s.LabelsString())
		}
	})

	t.Run("version matches the startup banner", func(t *testing.T) {
		want := waitForLog(t, versionLogRe)[1]
		expectLabel(t, s, "service_version", want)
	})

	t.Run("instance id matches the startup log", func(t *testing.T) {
		logged := waitForLog(t, instanceIDLogRe)[1]
		expectLabel(t, s, "service_instance_id", logged)
		if n := len(linesContaining(pumpLogs(t), logged)); n != 1 {
			t.Errorf("instance id %s appears on %d log lines, want exactly 1", logged, n)
		}
	})

	t.Run("detector attributes on metrics", func(t *testing.T) {
		// Host/container/process detectors are enabled like the Gateway.
		if missing := s.MissingLabels("host_name", "process_pid"); len(missing) > 0 {
			t.Errorf("series %s is missing detector labels %v", s.LabelsString(), missing)
		}
	})

	t.Run("target_info", func(t *testing.T) {
		// job/instance on target_info are service.name/service.instance.id
		// (see prometheus.yml honor_labels).
		info := promtest.WaitForSeries(t, prom, targetInfoSelector(), pollTimeout, pollInterval)
		if len(info) != 1 {
			t.Fatalf("expected one target_info series for one Pump process, got %d", len(info))
		}
		ti := info[0]
		if missing := ti.MissingLabels("instance", "service_version", "deployment_environment"); len(missing) > 0 {
			t.Fatalf("target_info %s is missing identity labels %v", ti.LabelsString(), missing)
		}
		if ti.Labels["instance"] != s.Labels["service_instance_id"] {
			t.Errorf("target_info instance %q differs from metric service_instance_id %q",
				ti.Labels["instance"], s.Labels["service_instance_id"])
		}
		expectLabel(t, ti, "deployment_environment", "e2e")
	})

	t.Run("job and instance carry the OTel identity", func(t *testing.T) {
		// No exported_* leftovers: honor_labels keeps the Pump's identity as
		// job/instance on every series.
		expectLabel(t, s, "job", promtest.ServiceName)
		expectLabel(t, s, "instance", s.Labels["service_instance_id"])
		for _, k := range []string{"exported_job", "exported_instance"} {
			if _, ok := s.Labels[k]; ok {
				t.Errorf("unexpected %s label on %s — is honor_labels set in prometheus.yml?", k, s.LabelsString())
			}
		}
	})
}

func expectLabel(t *testing.T, s promtest.Series, key, want string) {
	t.Helper()
	if got := s.Labels[key]; got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}
