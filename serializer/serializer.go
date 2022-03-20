package serializer

import (
	"errors"
	"reflect"

	"github.com/TykTechnologies/tyk-pump/analytics"
	logger "github.com/TykTechnologies/tyk-pump/logger"
	"github.com/niubaoshu/gotiny"
	"gopkg.in/vmihailenco/msgpack.v2"
)

var log = logger.GetLogger()

type AnalyticsSerializer interface {
	Encode(record *analytics.AnalyticsRecord) ([]byte, error)
	Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error
	GetSuffix() string
}

const MSGP_SERIALIZER = "msgpack"
const GOTINY_SERIALIZER = "gotiny"

func NewAnalyticsSerializer(serializerType string) AnalyticsSerializer {
	switch serializerType {
	case GOTINY_SERIALIZER:
		serializer := &GoTinySerializer{}

		recordType := reflect.TypeOf(analytics.AnalyticsRecord{})
		serializer.encoder = gotiny.NewEncoderWithType(recordType)
		serializer.decoder = gotiny.NewDecoderWithType(recordType)

		log.Debugf("Using serializer %v for analytics \n", GOTINY_SERIALIZER)
		return serializer
	case MSGP_SERIALIZER:
	default:
		log.Debugf("Using serializer %v for analytics \n", MSGP_SERIALIZER)
	}
	return &MsgpSerializer{}
}

type MsgpSerializer struct {
}

func (serializer *MsgpSerializer) Encode(record *analytics.AnalyticsRecord) ([]byte, error) {
	return msgpack.Marshal(record)
}

func (serializer *MsgpSerializer) Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error {
	data := []byte{}
	switch analyticsData.(type) {
	case string:
		data = []byte(analyticsData.(string))
	case []byte:
		data = analyticsData.([]byte)
	}

	return msgpack.Unmarshal(data, record)
}

func (serializer *MsgpSerializer) GetSuffix() string {
	return ""
}

type GoTinySerializer struct {
	encoder *gotiny.Encoder
	decoder *gotiny.Decoder
}

func (serializer *GoTinySerializer) Encode(record *analytics.AnalyticsRecord) ([]byte, error) {
	data := serializer.encoder.Encode(record)
	if len(data) == 0 {
		return data, errors.New("error encoding analytic record")
	}
	return data, nil
}

func (serializer *GoTinySerializer) Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error {
	index := serializer.decoder.Decode(analyticsData.([]byte), record)
	if index == 0 {
		return errors.New("error decoding analytic record")
	}
	return nil
}

func (serializer *GoTinySerializer) GetSuffix() string {
	return "_" + GOTINY_SERIALIZER
}
