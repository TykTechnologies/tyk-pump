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
	ENV_TYK_LOGLEVEL = "TYK_LOGLEVEL"
)

func init() {
	log.Level = level(os.Getenv(ENV_TYK_LOGLEVEL))

	formatter := newFormatter(FormatText)
	log.SetFormatter(formatter)
}

func SetupFormatter(format Format, env ...string) {
	envFormat := Format(coalesce(env...))

	if len(envFormat) != 0 {
		format = envFormat
	}

	formatter := newFormatter(format)
	log.SetFormatter(formatter)

	if format != FormatLegacy {
		logrus.StandardLogger().SetFormatter(formatter)
	}
}

func GetLogger() *logrus.Logger {
	return log
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

func newFormatter(format Format) logrus.Formatter {
	switch format {
	case FormatLegacy:
		return &logrus.TextFormatter{
			TimestampFormat: `Jan 02 15:04:05`,
			FullTimestamp:   true,
			DisableColors:   true,
		}
	case FormatJSON:
		return &logrus.JSONFormatter{
			FieldMap:        fieldMapperDefault,
			TimestampFormat: time.RFC3339,
		}
	case FormatText:
		fallthrough
	default:
		return &logrus.TextFormatter{
			FieldMap:        fieldMapperDefault,
			TimestampFormat: time.RFC3339,
			FullTimestamp:   true,
			DisableColors:   true,
		}
	}
}

func coalesce[T comparable](values ...T) T {
	var zero T

	for _, v := range values {
		if zero != v {
			return v
		}
	}

	return zero
}
