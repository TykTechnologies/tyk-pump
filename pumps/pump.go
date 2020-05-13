package pumps

import (
	"context"
	"errors"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type Pump interface {
	GetName() string
	New() Pump
	Init(interface{}) error
	WriteData(context.Context, []interface{}) error
	SetFilters(analytics.AnalyticsFilters)
	GetFilters() analytics.AnalyticsFilters
	SetTimeout(timeout int)
	GetTimeout() int
}

func GetPumpByName(name string) (Pump, error) {

	if pump, ok := AvailablePumps[name]; ok && pump != nil {
		return pump, nil
	}

	return nil, errors.New(name + " Not found")
}
