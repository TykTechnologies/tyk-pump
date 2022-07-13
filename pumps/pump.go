package pumps

import (
	"context"
	"errors"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

const PUMPS_ENV_PREFIX = "TYK_PMP_PUMPS"
const PUMPS_ENV_META_PREFIX = "_META"

type Pump interface {
	GetName() string
	Init(interface{}) error
	WriteData(context.Context, []interface{}) error
	SetFilters(analytics.AnalyticsFilters)
	GetFilters() analytics.AnalyticsFilters
	SetTimeout(timeout int)
	GetTimeout() int
	SetOmitDetailedRecording(bool)
	GetOmitDetailedRecording() bool
	GetEnvPrefix() string
	Shutdown() error
	SetMaxRecordSize(size int)
	GetMaxRecordSize() int
}

type UptimePump interface {
	GetName() string
	Init(interface{}) error
	WriteUptimeData(data []interface{})
}

func GetPumpByName(name string) (Pump, error) {

	if pump, ok := AvailablePumps[name]; ok && pump != nil {
		return pump, nil
	}

	return nil, errors.New(name + " Not found")
}
