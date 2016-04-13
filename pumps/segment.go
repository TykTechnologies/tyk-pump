package pumps

import (
	"fmt"
	"encoding/json"
	"errors"
	"github.com/Sirupsen/logrus"
	segment "github.com/Segment/analytics-go"
)

type SegmentPump struct {
	segmentClient *segment.Client
	segmentConf   *SegmentConf
}

var segmentPrefix string = "segment-pump"

type SegmentConf struct {
	WriteKey string `mapstructure:"segment_write_key"`
}

func (s *SegmentPump) New() Pump {
	newPump := SegmentPump{}
	return &newPump
}

func (s *SegmentPump) GetName() string {
	return "Segment Pump"
}

func (s *SegmentPump) Init(config interface{}) error {
	s.segmentConf = &SegmentConf{}
	loadConfigErr := mapstructure.Decode(config, &s.segmentConf)

	if loadConfigErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": segmentPrefix,
		}).Fatal("Failed to decode configuration: ", loadConfigErr)
	}

	s.segmentClient = segment.New(s.segmentConf.WriteKey)

	return nil
}

func (s *SegmentPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": segmentPrefix,
	}).Info("Writing ", len(data), " records")

	for i, v := range data {
		analyticsRecord := AnalyticsRecord{}
		err := msgpack.Unmarshal(v.([]byte), &analyticsRecord)
		log.Debug("Decoded record: ", analyticsRecord)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": segmentPrefix,
			}).Error("Couldn't unmarshal analytics data:", err)
		} else {
			s.WriteDataRecord(analyticsRecord)
		}
	}

	return nil
}

func (s *SegmentPump) WriteDataRecord(record AnalyticsRecord) error {
	key := record.APIKey
	properties, err := s.ToJSONMap(record)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": segmentPrefix,
		}).Error("Couldn't marshal analytics data:", err)
	} else {
		s.segmentClient.Track(&segment.Track{
			Event:       "Hit",
			AnonymousId: key,
			Properties:  properties,
		})
	}

	return nil
}

func (s *SegmentPump) ToJSONMap(obj interface{}) (map[string]interface{}, error) {
	ev, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var properties map[string]interface{}
	err = json.Unmarshal(ev, &properties)
	if err != nil {
		return nil, err
	}

	return properties, nil
}
