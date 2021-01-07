package pumps

import (
	"context"
	"encoding/json"

	"github.com/TykTechnologies/tyk-pump/analyticspb"
	"github.com/mitchellh/mapstructure"
	segment "github.com/segmentio/analytics-go"

	"github.com/TykTechnologies/logrus"
)

type SegmentPump struct {
	segmentClient *segment.Client
	segmentConf   *SegmentConf
	CommonPumpConfig
}

var segmentPrefix = "segment-pump"

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

func (s *SegmentPump) WriteData(ctx context.Context, data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": segmentPrefix,
	}).Info("Writing ", len(data), " records")

	for _, v := range data {
		s.WriteDataRecord(v.(analyticspb.AnalyticsRecord))
	}

	return nil
}

func (s *SegmentPump) WriteDataRecord(record analyticspb.AnalyticsRecord) error {
	key := record.APIKey
	properties, err := s.ToJSONMap(record)

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": segmentPrefix,
		}).Error("Couldn't marshal analytics data:", err)
	} else {
		err = s.segmentClient.Track(&segment.Track{
			Event:       "Hit",
			AnonymousId: key,
			Properties:  properties,
		})
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": segmentPrefix,
			}).Error("Couldn't track record:", err)
		}
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
