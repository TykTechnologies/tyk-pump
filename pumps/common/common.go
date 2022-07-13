package common

import (
	"fmt"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/kelseyhightower/envconfig"
)

type Pump struct {
	filters               analytics.AnalyticsFilters
	Timeout               int
	maxRecordSize         int
	OmitDetailedRecording bool
	Log                   *logrus.Entry
}

func (p *Pump) GetName() string {
	return ""
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

func (p *Pump) ProcessEnvVars(log *logrus.Entry, cfg interface{}, defaultEnv string) {
	if envVar := p.GetEnvPrefix(); envVar != "" {
		log.Debug(fmt.Sprintf("Checking %s env variables with prefix %s", p.GetName(), envVar))
		overrideErr := envconfig.Process(envVar, cfg)
		if overrideErr != nil {
			log.Error(fmt.Sprintf("Failed to process environment variables for %s pump %s with err:%v ", envVar, p.GetName(), overrideErr))
		}
	} else {
		log.Debug(fmt.Sprintf("Checking default %s env variables with prefix %s", p.GetName(), defaultEnv))
		overrideErr := envconfig.Process(defaultEnv, cfg)
		if overrideErr != nil {
			log.Error(fmt.Sprintf("Failed to process environment variables for %s pump %s with err:%v ", defaultEnv, p.GetName(), overrideErr))
		}
	}
}
