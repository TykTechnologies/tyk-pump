package analytics

import (
	"gopkg.in/vmihailenco/msgpack.v2"
)

type MsgpSerializer struct {
}

func (serializer *MsgpSerializer) Encode(record *AnalyticsRecord) ([]byte, error) {
	return msgpack.Marshal(record)
}

func (serializer *MsgpSerializer) Decode(analyticsData interface{}, record *AnalyticsRecord) error {
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
