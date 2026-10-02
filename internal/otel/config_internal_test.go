package otel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFamilyEnabled pins the three-state contract every per-family toggle
// follows: on when metrics are on and the toggle is not explicitly false.
func TestFamilyEnabled(t *testing.T) {
	on, off := true, false

	for _, tc := range []struct {
		metrics *bool
		toggle  *bool
		name    string
		want    bool
	}{
		{nil, nil, "metrics unset, toggle unset", false},
		{nil, &on, "metrics unset, toggle true", false},
		{&off, &on, "metrics off, toggle true", false},
		{&on, nil, "metrics on, toggle unset", true},
		{&on, &on, "metrics on, toggle true", true},
		{&on, &off, "metrics on, toggle false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := OpenTelemetry{}
			c.Metrics.Enabled = tc.metrics
			assert.Equal(t, tc.want, c.familyEnabled(tc.toggle))
		})
	}
}
