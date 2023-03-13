package analytics

type AnalyticsSerializer interface {
	Encode(record *AnalyticsRecord) ([]byte, error)
	Decode(analyticsData interface{}, record *AnalyticsRecord) error
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
