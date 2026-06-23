package logger

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestFormatterWithForcedPrefixFileOutput check if the prefix is stored in not TTY outputs
// SW-REQ-033:nominal:nominal
func TestFormatterWithForcedPrefixFileOutput(t *testing.T) {
	outputFile := "test.log"
	var f *os.File
	var err error
	if f, err = os.Create(outputFile); err != nil {
		t.Errorf("create log test file failed")
		return
	}

	logger := log
	logger.Out = f

	logger.WithFields(logrus.Fields{
		"prefix": "test-prefix",
	}).Errorf("test error log")

	err = f.Sync()
	if err != nil {
		t.Error("Sync test logs file:" + err.Error())
	}
	err = f.Close()
	if err != nil {
		t.Error("Closing test logs file:" + err.Error())
	}

	//Now check the content in the file
	b, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Error("Reading test logs file")
	}
	fileContent := string(b)
	if !strings.Contains(fileContent, "prefix") {
		t.Error("Prefix is not being added to logs information")
	}

	err = os.Remove(outputFile)
	if err != nil {
		t.Error("Error removing test logs file:" + err.Error())
	}
}

// Verifies: SW-REQ-033
// SW-REQ-033:nominal:nominal
// MCDC SW-REQ-033: env_level_recognised=F, legacy_formatter_installed=T, level_set_from_env=F => TRUE
// MCDC SW-REQ-033: env_level_recognised=T, legacy_formatter_installed=T, level_set_from_env=T => TRUE
func Test_GetLogger(t *testing.T) {
	tests := []struct {
		name          string
		env           string
		expectedLevel logrus.Level
	}{
		{
			name:          "default",
			env:           "",
			expectedLevel: logrus.InfoLevel,
		},
		{
			name:          "error",
			env:           "error",
			expectedLevel: logrus.ErrorLevel,
		},
		{
			name:          "warn",
			env:           "warn",
			expectedLevel: logrus.WarnLevel,
		},
		{
			name:          "info",
			env:           "info",
			expectedLevel: logrus.InfoLevel,
		},
		{
			name:          "debug",
			env:           "debug",
			expectedLevel: logrus.DebugLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logLevel := level(tt.env)
			if logLevel != tt.expectedLevel {
				t.Errorf("Expected level %v, got %v", tt.expectedLevel, logLevel)
			}
		})
	}
}
