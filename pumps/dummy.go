package pumps

import (
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type DummyPump struct {
	filters analytics.AnalyticsFilters
}

var dummyPrefix = "dummy-pump"

func (p *DummyPump) New() Pump {
	newPump := DummyPump{}
	return &newPump
}

func (p *DummyPump) GetName() string {
	return "Dummy Pump"
}

func (p *DummyPump) Init(conf interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": dummyPrefix,
	}).Debug("Dummy Initialized")
	return nil
}

func (p *DummyPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": dummyPrefix,
	}).Info("Writing ", len(data), " records")
	return nil
}

func (p *DummyPump) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *DummyPump) GetFilters() analytics.AnalyticsFilters {
	return p.filters
}
