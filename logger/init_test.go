package logger

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestFormatterWithForcedPrefixFileOutput check if the prefix is stored in not TTY outputs
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

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name         string
		format       Format
		expectedType interface{}
		expectedTime string
	}{
		{
			name:         "Format JSON",
			format:       FormatJSON,
			expectedType: &logrus.JSONFormatter{},
		},
		{
			name:         "Format Text",
			format:       FormatText,
			expectedType: &logrus.TextFormatter{},
			expectedTime: TimeFormatRFC3339,
		},
		{
			name:         "Format Legacy",
			format:       FormatLegacy,
			expectedType: &logrus.TextFormatter{},
			expectedTime: TimeFormatLegacy,
		},
		{
			name:         "Format Default or Unknown",
			format:       Format("unknown-format"),
			expectedType: &logrus.TextFormatter{},
			expectedTime: TimeFormatRFC3339,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := newFormatter(tt.format)

			assert.IsType(t, tt.expectedType, formatter)

			if txtFormatter, ok := formatter.(*logrus.TextFormatter); ok {
				assert.Equal(t, tt.expectedTime, txtFormatter.TimestampFormat)
			}
		})
	}
}

func TestSetupFormatter(t *testing.T) {
	oldLocalFormatter := log.Formatter
	oldGlobalFormatter := logrus.StandardLogger().Formatter

	defer func() {
		log.SetFormatter(oldLocalFormatter)
		logrus.StandardLogger().SetFormatter(oldGlobalFormatter)
	}()

	tests := []struct {
		name        string
		inputFormat Format
		envSetup    func(t *testing.T)
		envVars     []string
		expectJSON  bool
		expectSync  bool
	}{
		{
			name:        "Use default format without env vars",
			inputFormat: FormatJSON,
			envSetup:    func(t *testing.T) {},
			envVars:     []string{"NON_EXISTENT_ENV"},
			expectJSON:  true,
			expectSync:  true,
		},
		{
			name:        "Override text format with first env var to JSON",
			inputFormat: FormatText,
			envSetup: func(t *testing.T) {
				t.Setenv("LOG_FORMAT_ENV", "json")
			},
			envVars:    []string{"LOG_FORMAT_ENV", "SECONDARY_ENV"},
			expectJSON: true,
			expectSync: true,
		},
		{
			name:        "Skip empty env var and read next config",
			inputFormat: FormatText,
			envSetup: func(t *testing.T) {
				t.Setenv("SECONDARY_ENV", "json")
			},
			envVars:    []string{"LOG_FORMAT_ENV", "SECONDARY_ENV"},
			expectJSON: true,
			expectSync: true,
		},
		{
			name:        "Legacy format blocks propagation to global logrus",
			inputFormat: FormatLegacy,
			envSetup:    func(t *testing.T) {},
			envVars:     []string{},
			expectJSON:  false,
			expectSync:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.SetFormatter(oldLocalFormatter)
			logrus.StandardLogger().SetFormatter(oldGlobalFormatter)

			tt.envSetup(t)

			SetupFormatter(tt.inputFormat, tt.envVars...)

			if tt.expectJSON {
				assert.IsType(t, &logrus.JSONFormatter{}, log.Formatter)
			} else {
				assert.IsType(t, &logrus.TextFormatter{}, log.Formatter)
			}

			if tt.expectSync {
				assert.Equal(t, log.Formatter, logrus.StandardLogger().Formatter)
			} else {
				assert.Equal(t, oldGlobalFormatter, logrus.StandardLogger().Formatter)
			}
		})
	}
}
