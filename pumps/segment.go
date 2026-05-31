package pumps

import (
	"context"
	"encoding/json"

	"github.com/mitchellh/mapstructure"
	segment "github.com/segmentio/analytics-go"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type SegmentPump struct {
	segmentClient *segment.Client
	segmentConf   *SegmentConf
	CommonPumpConfig
}

var segmentPrefix = "segment-pump"
var segmentDefaultENV = PUMPS_ENV_PREFIX + "_SEGMENT" + PUMPS_ENV_META_PREFIX

type SegmentConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	WriteKey  string `json:"segment_write_key" mapstructure:"segment_write_key"`
}

// reqproof:implements SW-REQ-053
func (s *SegmentPump) New() Pump {
	newPump := SegmentPump{}
	return &newPump
}

// reqproof:implements SW-REQ-053
func (s *SegmentPump) GetName() string {
	return "Segment Pump"
}

// reqproof:implements SW-REQ-053
func (s *SegmentPump) GetEnvPrefix() string {
	return s.segmentConf.EnvPrefix
}

// reqproof:implements SW-REQ-053
func (s *SegmentPump) Init(config interface{}) error {
	s.segmentConf = &SegmentConf{}
	s.log = log.WithField("prefix", segmentPrefix)

	loadConfigErr := mapstructure.Decode(config, &s.segmentConf)
	if loadConfigErr != nil {
		s.log.Fatal("Failed to decode configuration: ", loadConfigErr)
	}

	processPumpEnvVars(s, s.log, s.segmentConf, segmentDefaultENV)

	s.segmentClient = segment.New(s.segmentConf.WriteKey)
	s.log.Info(s.GetName() + " Initialized")

	return nil
}

// reqproof:implements SW-REQ-053
func (s *SegmentPump) WriteData(ctx context.Context, data []interface{}) error {
	s.log.Debug("Attempting to write ", len(data), " records...")

	for _, v := range data {
		s.WriteDataRecord(v.(analytics.AnalyticsRecord))
	}
	s.log.Info("Purged ", len(data), " records...")

	return nil
}

// reqproof:implements SW-REQ-053
func (s *SegmentPump) WriteDataRecord(record analytics.AnalyticsRecord) error {
	key := record.APIKey
	properties, err := s.ToJSONMap(record)

	if err != nil {
		s.log.Error("Couldn't marshal analytics data:", err)
	} else {
		err = s.segmentClient.Track(&segment.Track{
			Event:       "Hit",
			AnonymousId: key,
			Properties:  properties,
		})
		if err != nil {
			s.log.Error("Couldn't track record:", err)
		}
	}

	return nil
}

// reqproof:implements SW-REQ-053
func (s *SegmentPump) ToJSONMap(obj interface{}) (map[string]interface{}, error) {
	ev, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var properties map[string]interface{}
	err = json.Unmarshal(ev, &properties)
	if err != nil { //mcdc:ignore json.Unmarshal of bytes just produced by json.Marshal of an AnalyticsRecord into map[string]interface{} cannot fail. KI mcdc-pumps-below-95.
		return nil, err
	}

	return properties, nil
}
