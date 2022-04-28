package serializer

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	logger "github.com/TykTechnologies/tyk-pump/logger"
)

var log = logger.GetLogger()

type AnalyticsSerializer interface {
	Encode(record *analytics.AnalyticsRecord) ([]byte, error)
	Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error
	GetSuffix() string
}

const MSGP_SERIALIZER = "msgpack"
const PROTOBUF_SERIALIZER = "protobuf"

func NewAnalyticsSerializer(serializerType string) AnalyticsSerializer {
	switch serializerType {
	case PROTOBUF_SERIALIZER:
		serializer := &ProtobufSerializer{}
		log.Debugf("Using serializer %v for analytics \n", PROTOBUF_SERIALIZER)
		return serializer
	case MSGP_SERIALIZER:
	default:
		log.Debugf("Using serializer %v for analytics \n", MSGP_SERIALIZER)
	}
	return &MsgpSerializer{}
}
