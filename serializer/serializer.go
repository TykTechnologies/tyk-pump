package serializer

import (
	"reflect"

	"github.com/TykTechnologies/tyk-pump/analytics"
	logger "github.com/TykTechnologies/tyk-pump/logger"
	"github.com/niubaoshu/gotiny"
)

var log = logger.GetLogger()

type AnalyticsSerializer interface {
	Encode(record *analytics.AnalyticsRecord) ([]byte, error)
	Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error
	GetSuffix() string
}

const MSGP_SERIALIZER = "msgpack"
const GOTINY_SERIALIZER = "gotiny"
const PROTOBUF_SERIALIZER = "protobuf"

func NewAnalyticsSerializer(serializerType string) AnalyticsSerializer {
	switch serializerType {
	case GOTINY_SERIALIZER:
		serializer := &GoTinySerializer{}

		recordType := reflect.TypeOf(analytics.AnalyticsRecord{})
		serializer.encoder = gotiny.NewEncoderWithType(recordType)
		serializer.decoder = gotiny.NewDecoderWithType(recordType)

		log.Debugf("Using serializer %v for analytics \n", GOTINY_SERIALIZER)
		return serializer
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
