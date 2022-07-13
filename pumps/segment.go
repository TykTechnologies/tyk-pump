package pumps

import (
	"context"
	"encoding/json"

	"github.com/TykTechnologies/tyk-pump/pumps/common"
	"github.com/mitchellh/mapstructure"
	segment "github.com/segmentio/analytics-go"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type SegmentPump struct {
	segmentClient *segment.Client
	segmentConf   *SegmentConf
	common.Pump
}

var segmentPrefix = "segment-pump"
var segmentDefaultENV = PUMPS_ENV_PREFIX + "_SEGMENT" + PUMPS_ENV_META_PREFIX

type SegmentConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	WriteKey  string `json:"segment_write_key" mapstructure:"segment_write_key"`
}

func (s *SegmentPump) GetName() string {
	return "Segment Pump"
}

func (s *SegmentPump) GetEnvPrefix() string {
	return s.segmentConf.EnvPrefix
}

func (s *SegmentPump) Init(config interface{}) error {
	s.segmentConf = &SegmentConf{}
	s.Log = log.WithField("prefix", segmentPrefix)

	loadConfigErr := mapstructure.Decode(config, &s.segmentConf)
	if loadConfigErr != nil {
		s.Log.Fatal("Failed to decode configuration: ", loadConfigErr)
	}

	processPumpEnvVars(s, s.Log, s.segmentConf, segmentDefaultENV)

	s.segmentClient = segment.New(s.segmentConf.WriteKey)
	s.Log.Info(s.GetName() + " Initialized")

	return nil
}

func (s *SegmentPump) WriteData(ctx context.Context, data []interface{}) error {
	s.Log.Debug("Attempting to write ", len(data), " records...")

	for _, v := range data {
		s.WriteDataRecord(v.(analytics.AnalyticsRecord))
	}
	s.Log.Info("Purged ", len(data), " records...")

	return nil
}

func (s *SegmentPump) WriteDataRecord(record analytics.AnalyticsRecord) error {
	key := record.APIKey
	properties, err := s.ToJSONMap(record)

	if err != nil {
		s.Log.Error("Couldn't marshal analytics data:", err)
	} else {
		err = s.segmentClient.Track(&segment.Track{
			Event:       "Hit",
			AnonymousId: key,
			Properties:  properties,
		})
		if err != nil {
			s.Log.Error("Couldn't track record:", err)
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
