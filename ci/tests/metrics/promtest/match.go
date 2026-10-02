package promtest

import (
	"fmt"
	"sort"
	"strings"
)

// LabelKeys returns the sorted label keys of the series, excluding __name__.
func (s Series) LabelKeys() []string {
	keys := make([]string, 0, len(s.Labels))
	for k := range s.Labels {
		if k == "__name__" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// MissingLabels returns the keys in want that the series does not carry, or
// that it carries with an empty value. An empty result means every wanted
// label is present.
func (s Series) MissingLabels(want ...string) []string {
	var missing []string
	for _, k := range want {
		if v, ok := s.Labels[k]; !ok || v == "" {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

// Family describes one metric family the Pump is expected to export, as
// it appears in Prometheus after the collector's OTLP → Prometheus conversion
// (unit suffix appended, dots replaced by underscores).
type Family struct {
	// Name is the Prometheus metric name, e.g. "process_uptime_seconds".
	Name string
	// Type is the Prometheus metric type: "gauge", "counter" or "histogram".
	Type string
	// Labels are the label keys every series of the family must carry with a
	// non-empty value. Extra labels (host/process detectors, job, instance)
	// are allowed; a missing required key fails.
	Labels []string
	// OTelName is the instrument name as registered in the Pump, kept
	// for readable failure messages.
	OTelName string
}

// SeriesName is the metric name to use in series queries for the family. A
// histogram has no series under its base name in Prometheus — only _bucket,
// _sum and _count — so its per-request series are addressed via _count; the
// base Name still addresses metadata.
func (f Family) SeriesName() string {
	if f.Type == "histogram" {
		return f.Name + "_count"
	}
	return f.Name
}

// Check validates a family against the metadata Prometheus reports and the
// series it currently holds. It returns a human-readable problem list; an
// empty list means the family matches. The messages name the family so a
// missing or renamed metric fails with a clear pointer.
func (f Family) Check(md Metadata, found bool, series []Series) []string {
	if !found {
		return []string{fmt.Sprintf("%s (%s): metric family not found in Prometheus — missing or renamed?", f.Name, f.OTelName)}
	}
	var problems []string
	if md.Type != f.Type {
		problems = append(problems, fmt.Sprintf("%s: type is %q, want %q", f.Name, md.Type, f.Type))
	}
	if len(series) == 0 {
		problems = append(problems, fmt.Sprintf("%s: metadata present but no series exported", f.Name))
		return problems
	}
	for _, s := range series {
		if missing := s.MissingLabels(f.Labels...); len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("%s: series %s is missing required labels %v", f.Name, s.LabelsString(), missing))
		}
	}
	return problems
}

// LabelsString renders the series labels as a compact {k="v",...} selector,
// useful in failure messages.
func (s Series) LabelsString() string {
	return formatLabels(s.Labels, s.LabelKeys())
}

// Selector builds a PromQL selector for name with exact label matches.
func Selector(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return name + formatLabels(labels, keys)
}

// formatLabels renders labels as {k="v",...} in the order of keys.
func formatLabels(labels map[string]string, keys []string) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}
