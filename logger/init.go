package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
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

	log.Formatter = formatter()
}

func formatter() *logrus.TextFormatter {
	formatter := new(logrus.TextFormatter)
	formatter.TimestampFormat = `Jan 02 15:04:05`
	formatter.FullTimestamp = true
	formatter.DisableColors = true
	return formatter
}

func GetLogger() *logrus.Logger {
	return log
}
