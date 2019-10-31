package logger

import (
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

var log = logrus.New()

func init() {
	log.Formatter = GetDefaultFormatter()
}

//GetDefaultFormatter returns a logrus.TextFormatter object
//with the configs to show all the info even in
//TTY-less logs displayer
func GetDefaultFormatter() *logrus.TextFormatter{
	textFormatter := new(logrus.TextFormatter)
	textFormatter.ForceColors= true
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
