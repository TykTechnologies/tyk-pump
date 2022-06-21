package logger

import (
	"os"
	"strings"

	"github.com/TykTechnologies/logrus"
	prefixed "github.com/TykTechnologies/logrus-prefixed-formatter"
)

var log = logrus.New()

func init() {
	log.Formatter = GetFormatterWithForcedPrefix()
}

func GetFormatterWithForcedPrefix() *prefixed.TextFormatter {
	textFormatter := new(prefixed.TextFormatter)
	textFormatter.ForceColors = true
	textFormatter.TimestampFormat = `Jan 02 15:04:05`
	return textFormatter
}

func GetLogger() *logrus.Logger {
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
