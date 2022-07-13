package common

import (
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type Pump struct {
	filters       analytics.AnalyticsFilters
	Timeout       int
	maxRecordSize int
	OmitDetailedRecording bool
	Log                   *logrus.Entry
}

func (p *Pump) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *Pump) GetFilters() analytics.AnalyticsFilters {
	return p.filters
}
func (p *Pump) SetTimeout(timeout int) {
	p.Timeout = timeout
}

func (p *Pump) GetTimeout() int {
	return p.Timeout
}

func (p *Pump) SetOmitDetailedRecording(OmitDetailedRecording bool) {
	p.OmitDetailedRecording = OmitDetailedRecording
}
func (p *Pump) GetOmitDetailedRecording() bool {
	return p.OmitDetailedRecording
}

func (p *Pump) GetEnvPrefix() string {
	return ""
}

func (p *Pump) Shutdown() error {
	return nil
}

func (p *Pump) SetMaxRecordSize(size int) {
	p.maxRecordSize = size
}

func (p *Pump) GetMaxRecordSize() int {
	return p.maxRecordSize
}
