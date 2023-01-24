package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var log = logrus.New()

func init() {
	log.Formatter = GetFormatterWithForcedPrefix()
}

func GetFormatterWithForcedPrefix() *prefixed.TextFormatter {
	formatter := new(prefixed.TextFormatter)
	formatter.TimestampFormat = `Jan 02 15:04:05`
	formatter.FullTimestamp = true

	return formatter
}

func GetLogger() *logrus.Logger {
	level := os.Getenv("TYK_LOGLEVEL")

	switch strings.ToLower(level) {
	case "error":
		log.Level = logrus.ErrorLevel
	case "warn":
		log.Level = logrus.WarnLevel
	case "debug":
		log.Level = logrus.DebugLevel
	default:
		log.Level = logrus.InfoLevel
	}

	return log
}
