package serializer

import (
	"errors"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/niubaoshu/gotiny"
)

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
