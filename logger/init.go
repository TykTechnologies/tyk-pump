package logger

import (
	"os"
	"strings"

	"github.com/TykTechnologies/logrus"
	prefixed "github.com/TykTechnologies/logrus-prefixed-formatter"
)

var log *logrus.Logger

// GetLogger returns the default logger.
func GetLogger() *logrus.Logger {
	// Make sure the logger is only initialized once:
	if log != nil {
		return log
	}

	// First check the log level environment variable:
	log = logrus.New()
	log.Formatter = new(prefixed.TextFormatter)
	level := os.Getenv("TYK_LOGLEVEL")
	if level == "" {
		level = "info"
	}

	switch strings.ToLower(level) {
	case "error":
		log.Level = logrus.ErrorLevel
	case "warn":
		log.Level = logrus.WarnLevel
	case "info":
		log.Level = logrus.InfoLevel
	case "debug":
		log.Level = logrus.DebugLevel
	default:
		log.Level = logrus.InfoLevel
	}

	return log
}
