package logger

import (
	"os"
	"strings"
	"time"

	"github.com/samber/lo"
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

func init() {
	log.Level = level(os.Getenv(EnvTykLoglevel))

	formatter := NewFormatter(FormatText)
	log.SetFormatter(formatter)
}

func SetupFormatter(format Format, env ...string) {
	envValuers := lo.Map(env, func(item string, _ int) Format {
		return Format(os.Getenv(item))
	})

	envFormat := lo.CoalesceOrEmpty(envValuers...)

	if len(envFormat) != 0 {
		format = envFormat
	}

	formatter := NewFormatter(format)
	log.SetFormatter(formatter)

	if format != FormatLegacy {
		logrus.StandardLogger().SetFormatter(formatter)
	}
}

func GetLogger() *logrus.Logger {
	return log
}

func NewFormatter(format Format) logrus.Formatter {
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
