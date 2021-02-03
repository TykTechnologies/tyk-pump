package pumps

import "github.com/TykTechnologies/tyk-pump/analytics"

type CommonPumpConfig struct {
	filters               analytics.AnalyticsFilters
	timeout               int
	OmitDetailedRecording bool
	IgnoreFields	[]string
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

func (p *CommonPumpConfig) SetIgnoredFields(IgnoreFields []string) {
	p.IgnoreFields = IgnoreFields
}
func (p *CommonPumpConfig) GetIgnoredFields() []string {
	return p.IgnoreFields
}