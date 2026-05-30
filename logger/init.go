package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

// reqproof:implements SW-REQ-033
func init() {
	log.Level = level(os.Getenv("TYK_LOGLEVEL"))
	log.Formatter = formatter()
}

// reqproof:implements SW-REQ-033
func level(level string) logrus.Level {
	switch strings.ToLower(level) {
	case "error":
		return logrus.ErrorLevel
	case "warn":
		return logrus.WarnLevel
	case "debug":
		return logrus.DebugLevel
	default:
		return logrus.InfoLevel
	}
}

// reqproof:implements SW-REQ-033
func formatter() *logrus.TextFormatter {
	formatter := new(logrus.TextFormatter)
	formatter.TimestampFormat = `Jan 02 15:04:05`
	formatter.FullTimestamp = true
	formatter.DisableColors = true
	return formatter
}

// reqproof:implements SW-REQ-033
func GetLogger() *logrus.Logger {
	return log
}
