package pumps

import (
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type CommonPumpConfig struct {
	filters               analytics.AnalyticsFilters
	timeout               int
	maxRecordSize         int
	OmitDetailedRecording bool
	log                   *logrus.Entry
}

func (p *CommonPumpConfig) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *CommonPumpConfig) GetFilters() analytics.AnalyticsFilters {
	return p.filters
}
func (p *CommonPumpConfig) SetTimeout(timeout int) {
	p.timeout = timeout
}

func (p *CommonPumpConfig) GetTimeout() int {
	return p.timeout
}

func (p *CommonPumpConfig) SetOmitDetailedRecording(OmitDetailedRecording bool) {
	p.OmitDetailedRecording = OmitDetailedRecording
}
func (p *CommonPumpConfig) GetOmitDetailedRecording() bool {
	return p.OmitDetailedRecording
}

func (p *CommonPumpConfig) GetEnvPrefix() string {
	return ""
}

func (p *CommonPumpConfig) Shutdown() error {
	return nil
}

func (p *CommonPumpConfig) SetMaxRecordSize(size int) {
	p.maxRecordSize = size
}

func (p *CommonPumpConfig) GetMaxRecordSize() int {
	return p.maxRecordSize
}

func (p *CommonPumpConfig) Validate() []interface{} {
	result := []interface{}{}
	return result
}
