package logger

import (
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	log                = logrus.New()
	fieldMapperDefault = logrus.FieldMap{
		logrus.FieldKeyMsg: "message",
	}
)

type Format string

const (
	FormatJSON   Format = "json"
	FormatText   Format = "text"
	FormatLegacy Format = "legacy"
)

const (
	EnvTykLoglevel  = "TYK_LOGLEVEL"
	EnvTykLogformat = "TYK_LOGFORMAT"
)

const (
	TimeFormatLegacy  = "Jan 02 15:04:05"
	TimeFormatRFC3339 = time.RFC3339
)

func init() {
	log.Level = level(os.Getenv(EnvTykLoglevel))

	formatter := newFormatter(FormatText)
	log.SetFormatter(formatter)
}

func SetupFormatter(format Format, envVars ...string) {
	resolvedFormat := format

	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			resolvedFormat = Format(val)
			break
		}
	}

	formatter := newFormatter(resolvedFormat)

	log.SetFormatter(formatter)
	if resolvedFormat != FormatLegacy {
		logrus.StandardLogger().SetFormatter(formatter)
	}
}

func GetLogger() *logrus.Logger {
	return log
}

func newFormatter(format Format) logrus.Formatter {
	switch format {
	case FormatLegacy:
		return &logrus.TextFormatter{
			TimestampFormat: TimeFormatLegacy,
			FullTimestamp:   true,
			DisableColors:   true,
		}
	case FormatJSON:
		return &logrus.JSONFormatter{
			FieldMap:        fieldMapperDefault,
			TimestampFormat: TimeFormatRFC3339,
		}
	case FormatText:
		fallthrough
	default:
		return &logrus.TextFormatter{
			FieldMap:        fieldMapperDefault,
			TimestampFormat: TimeFormatRFC3339,
			FullTimestamp:   true,
			DisableColors:   true,
		}
	}
}

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
