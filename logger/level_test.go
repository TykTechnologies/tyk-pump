package logger

import (
	"testing"

	"github.com/sirupsen/logrus"
)

// Verifies: SW-REQ-033
// SW-REQ-033:nominal:negative
func TestLevel_AllBranches(t *testing.T) {
	cases := []struct {
		in   string
		want logrus.Level
	}{
		{"error", logrus.ErrorLevel},
		{"ERROR", logrus.ErrorLevel}, // case-insensitive via strings.ToLower
		{"warn", logrus.WarnLevel},
		{"debug", logrus.DebugLevel},
		{"", logrus.InfoLevel},             // default
		{"unrecognised", logrus.InfoLevel}, // default
	}
	for _, c := range cases {
		if got := level(c.in); got != c.want {
			t.Fatalf("level(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// Verifies: SW-REQ-033
func TestFormatter_FixedShape(t *testing.T) {
	f := formatter()
	if !f.FullTimestamp {
		t.Fatal("formatter should set FullTimestamp")
	}
	if !f.DisableColors {
		t.Fatal("formatter should disable colors")
	}
	if f.TimestampFormat == "" {
		t.Fatal("formatter should set a TimestampFormat")
	}
}

// Verifies: SW-REQ-033
func TestGetLogger_ReturnsSingleton(t *testing.T) {
	a := GetLogger()
	b := GetLogger()
	if a != b {
		t.Fatal("GetLogger should return the package-level singleton")
	}
}
