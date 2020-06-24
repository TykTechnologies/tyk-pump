package pumps

import "github.com/TykTechnologies/tyk-pump/analytics"

type CommonPumpConfig struct {
	filters     analytics.AnalyticsFilters
	timeout     int
	omitDetails bool
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

func (p *CommonPumpConfig) SetOmitDetails(omitDetails bool) {
	p.omitDetails = omitDetails
}
func (p *CommonPumpConfig) GetOmitDetails() bool {
	return p.omitDetails
}
