package serializer

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/sirupsen/logrus"
)

type AnalyticsSerializer interface {
	Encode(record *analytics.AnalyticsRecord) ([]byte, error)
	Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error
	GetSuffix() string
}

const (
	MSGP_SERIALIZER     = "msgpack"
	PROTOBUF_SERIALIZER = "protobuf"
)

type analyticsSerializerOptions struct {
	logger *logrus.Logger
}

type NewAnalyticsSerializerOpt func(*analyticsSerializerOptions)

func NewAnalyticsSerializer(
	serializerType string,
	options ...NewAnalyticsSerializerOpt,
) AnalyticsSerializer {
	opt := analyticsSerializerOptions{
		logger: logger.GetLogger(),
	}

	for _, apply := range options {
		apply(&opt)
	}

	switch serializerType {
	case PROTOBUF_SERIALIZER:
		serializer := &ProtobufSerializer{}
		opt.logger.Debugf("Using serializer %v for analytics \n", PROTOBUF_SERIALIZER)
		return serializer
	case MSGP_SERIALIZER:
		fallthrough
	default:
		opt.logger.Debugf("Using serializer %v for analytics \n", MSGP_SERIALIZER)
		return &MsgpSerializer{}
	}
}

// WithLogger
// Overrides default logger.
func WithLogger(logger *logrus.Logger) NewAnalyticsSerializerOpt {
	return func(o *analyticsSerializerOptions) {
		o.logger = logger
	}
}
