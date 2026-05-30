package serializer

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	"gopkg.in/vmihailenco/msgpack.v2"
)

type MsgpSerializer struct {
}

// reqproof:implements SW-REQ-008
func (serializer *MsgpSerializer) Encode(record *analytics.AnalyticsRecord) ([]byte, error) {
	return msgpack.Marshal(record)
}

// reqproof:implements SW-REQ-008
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

// reqproof:implements SW-REQ-008
func (serializer *MsgpSerializer) GetSuffix() string {
	return ""
}
